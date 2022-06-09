package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/cloudogu/cesapp-lib/core"
	"github.com/cloudogu/cesapp-lib/registry"

	"github.com/sirupsen/logrus"

	"github.com/cloudogu/k8s-service-discovery/controllers"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"github.com/bombsimon/logrusr/v2"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
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
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	configureLogger()
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

	if err = handleIngressClassCreation(k8sManager); err != nil {
		return fmt.Errorf("failed to create ingress class creator: %w", err)
	}

	reg, err := createEtcdRegistry(options.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create registry: %w", err)
	}

	if err = handleWarpMenuCreation(k8sManager, reg, options.Namespace); err != nil {
		return fmt.Errorf("failed to create warp menu creator: %w", err)
	}

	if err = handleSslUpdates(k8sManager, options.Namespace); err != nil {
		return fmt.Errorf("failed to create ssl certificate updater: %w", err)
	}

	if err = configureManager(k8sManager, options.Namespace); err != nil {
		return fmt.Errorf("failed to configure service discovery manager: %w", err)
	}

	if err = startK8sManager(k8sManager); err != nil {
		return fmt.Errorf("failed to start service discovery manager: %w", err)
	}

	return nil
}

func createEtcdRegistry(namespace string) (registry.Registry, error) {
	r, err := registry.New(core.Registry{
		Type:      "etcd",
		Endpoints: []string{fmt.Sprintf("http://etcd.%s.svc.cluster.local:4001", namespace)},
	})

	return r, err
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

func configureLogger() {
	logrusLog := logrus.New()
	logrusLog.SetFormatter(&logrus.TextFormatter{})
	logrusLog.SetLevel(logrus.DebugLevel)

	ctrl.SetLogger(logrusr.New(logrusLog))
}

func getK8sManagerOptions() (manager.Options, error) {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

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

func handleWarpMenuCreation(k8sManager manager.Manager, registry registry.Registry, namespace string) error {
	warpMenuCreator := controllers.NewWarpMenuCreator(k8sManager.GetClient(), registry, namespace)

	if err := k8sManager.Add(warpMenuCreator); err != nil {
		return fmt.Errorf("failed to add warp menu creator as runnable to the manager: %w", err)
	}

	return nil
}

func handleSslUpdates(k8sManager manager.Manager, namespace string) error {
	sslUpdater, err := controllers.NewSslCertificateUpdater(k8sManager.GetClient(), namespace)
	if err != nil {
		return fmt.Errorf("failed to create new ssl certificate updater: %w", err)
	}

	if err = k8sManager.Add(sslUpdater); err != nil {
		return fmt.Errorf("failed to add ssl certificate updater as runnable to the manager: %w", err)
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
