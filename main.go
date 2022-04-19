package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/cloudogu/k8s-service-discovery/controllers"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

const (
	IngressClassName = "k8s-ecosystem-ces-service"
)

var (
	scheme               = runtime.NewScheme()
	setupLog             = ctrl.Log.WithName("setup")
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
	// namespaceEnvVar defines the name of the environment variables given into the service discovery to define the
	// namespace that should be watched by the service discovery. It is a required variable and an empty value will
	// produce an appropriate error message.
	namespaceEnvVar = "WATCH_NAMESPACE"
	// logModeEnvVar is the constant for env variable ZAP_DEVELOPMENT_MODE
	// which specifies the development mode for zap options. Valid values are
	// true or false. In development mode the logger produces a stacktrace on warnings and no sampling.
	// In regular mode (default) the logger produces a stacktrace on errors and sampling
	logModeEnvVar = "ZAP_DEVELOPMENT_MODE"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	if err := startManager(); err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func startManager() error {
	setupLog.Info("Starting k8s-service-discovery...")

	options, err := getK8sManagerOptions()
	if err != nil {
		return fmt.Errorf("failed to get manager options: %w", err)
	}

	k8sManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return fmt.Errorf("failed to create new manager: %w", err)
	}

	if err := handleIngressClassCreation(k8sManager); err != nil {
		return fmt.Errorf("failed to create ingress class creator: %w", err)
	}

	if err := configureManager(k8sManager, options.Namespace); err != nil {
		return fmt.Errorf("failed to configure service discovery manager: %w", err)
	}

	if err := startK8sManager(k8sManager); err != nil {
		return fmt.Errorf("failed to start service discovery manager: %w", err)
	}

	return nil
}

func configureManager(k8sManager manager.Manager, namespace string) error {
	if err := configureReconciler(k8sManager, namespace); err != nil {
		return fmt.Errorf("failed to configure reconciler: %w", err)
	}

	// This kubebuilder marking inserts boilerplate code required for the manager. Do not remove it!
	//+kubebuilder:scaffold:builder

	if err := addChecks(k8sManager); err != nil {
		return fmt.Errorf("failed to configure reconciler: %w", err)
	}

	return nil
}

func configureLogger() error {
	logMode := false

	logModeEnv, found := os.LookupEnv(logModeEnvVar)
	if found {
		parsedLogMode, err := strconv.ParseBool(logModeEnv)
		if err != nil {
			return fmt.Errorf("failed to parse %s; valid values are true or false: %w", logModeEnv, err)
		}
		logMode = parsedLogMode
	}

	opts := zap.Options{
		Development: logMode,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	return nil
}

func getK8sManagerOptions() (manager.Options, error) {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	if err := configureLogger(); err != nil {
		return manager.Options{}, fmt.Errorf("failed to configure logger: %w", err)
	}

	options := ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "92a787f2.cloudogu.com",
	}

	watchNamespace, found := os.LookupEnv(namespaceEnvVar)
	if !found {
		return manager.Options{}, fmt.Errorf("failed to read namespace to watch from environment variable [%s], please set the variable and try again", namespaceEnvVar)
	}
	options.Namespace = watchNamespace
	setupLog.Info(fmt.Sprintf("found target namespace: [%s]", watchNamespace))

	return options, nil
}

func startK8sManager(k8sManager manager.Manager) error {
	setupLog.Info("starting service discovery manager")

	err := k8sManager.Start(ctrl.SetupSignalHandler())
	if err != nil {
		return fmt.Errorf("failed to start service discovery manager: %w", err)
	}

	return nil
}

func handleIngressClassCreation(k8sManager manager.Manager) error {
	ingressClassCreator := controllers.NewIngressClassCreator(k8sManager.GetClient(), IngressClassName)

	if err := k8sManager.Add(ingressClassCreator); err != nil {
		return fmt.Errorf("failed to add ingress class creator as runnable to the manager: %w", err)
	}

	return nil
}

func configureReconciler(k8sManager manager.Manager, namespace string) error {
	ingressGenerator := controllers.NewIngressGenerator(k8sManager.GetClient(), namespace, IngressClassName)

	reconciler := &controllers.ServiceReconciler{
		Client:           k8sManager.GetClient(),
		Scheme:           k8sManager.GetScheme(),
		IngressGenerator: ingressGenerator,
	}

	if err := reconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup service discovery with the manager: %w", err)
	}

	return nil
}

func addChecks(mgr manager.Manager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to add healthz check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to add readyz check: %w", err)
	}

	return nil
}
