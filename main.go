package main

import (
	"flag"
	"fmt"
	"github.com/cloudogu/k8s-service-discovery/controllers/ssl"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	ginlogrus "github.com/toorop/gin-logrus"
	"os"

	"k8s.io/client-go/tools/record"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	// +kubebuilder:scaffold:imports

	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/cesregistry"

	"github.com/cloudogu/k8s-dogu-operator/api/v1"

	"github.com/cloudogu/k8s-service-discovery/controllers"
	"github.com/cloudogu/k8s-service-discovery/controllers/logging"
)

const (
	IngressClassName = "k8s-ecosystem-ces-service"
	apiPort          = 9090
)

var (
	scheme               = runtime.NewScheme()
	logger               = ctrl.Log.WithName("k8s-service-discovery")
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
	utilruntime.Must(v1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	if err := logging.ConfigureLogger(); err != nil {
		logger.Error(err, "unable configure logger")
		os.Exit(1)
	}
}

func main() {
	if err := startManager(); err != nil {
		logger.Error(err, "manager produced an error")
		os.Exit(1)
	}
}

func startManager() error {
	logger.Info("Starting k8s-service-discovery...")

	options, err := getK8sManagerOptions()
	if err != nil {
		return fmt.Errorf("failed to get manager options: %w", err)
	}

	k8sManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return fmt.Errorf("failed to create new manager: %w", err)
	}

	eventRecorder := k8sManager.GetEventRecorderFor("k8s-service-discovery-controller-manager")

	if err = handleIngressClassCreation(k8sManager, options.Namespace, eventRecorder); err != nil {
		return fmt.Errorf("failed to create ingress class creator: %w", err)
	}

	reg, err := cesregistry.Create(options.Namespace)
	if err != nil {
		return fmt.Errorf("failed to create registry: %w", err)
	}

	go func() {
		router := createRouter(reg)
		err = router.Run(fmt.Sprintf(":%d", apiPort))
		if err != nil {
			logger.Error(fmt.Errorf("failed to start gin router %w", err), "SSL api error")
		}
	}()

	if err = handleWarpMenuCreation(k8sManager, reg, options.Namespace, eventRecorder); err != nil {
		return fmt.Errorf("failed to create warp menu creator: %w", err)
	}

	if err = handleSslUpdates(k8sManager, options.Namespace, reg, eventRecorder); err != nil {
		return fmt.Errorf("failed to create ssl certificate updater: %w", err)
	}

	ingressUpdater, err := controllers.NewIngressUpdater(k8sManager.GetClient(), reg, options.Namespace, IngressClassName, eventRecorder)
	if err != nil {
		return fmt.Errorf("failed to create new ingress updater: %w", err)
	}

	if err = handleMaintenanceMode(k8sManager, options.Namespace, ingressUpdater, eventRecorder); err != nil {
		return err
	}

	if err = configureManager(k8sManager, ingressUpdater); err != nil {
		return fmt.Errorf("failed to configure service discovery manager: %w", err)
	}

	if err = startK8sManager(k8sManager); err != nil {
		return fmt.Errorf("failure at service discovery manager: %w", err)
	}

	return nil
}

func configureManager(k8sManager manager.Manager, updater controllers.IngressUpdater) error {
	if err := configureReconciler(k8sManager, updater); err != nil {
		return fmt.Errorf("failed to configure reconciler: %w", err)
	}

	// This kubebuilder marking inserts boilerplate code required for the manager. Do not remove it!
	// +kubebuilder:scaffold:builder

	if err := addChecks(k8sManager); err != nil {
		return fmt.Errorf("failed to configure reconciler: %w", err)
	}

	return nil
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
	logger.Info(fmt.Sprintf("found target namespace: [%s]", watchNamespace))

	return options, nil
}

func startK8sManager(k8sManager manager.Manager) error {
	logger.Info("starting service discovery manager")

	err := k8sManager.Start(ctrl.SetupSignalHandler())
	if err != nil {
		return fmt.Errorf("failed to start service discovery manager: %w", err)
	}

	return nil
}

func handleIngressClassCreation(k8sManager manager.Manager, namespace string, recorder record.EventRecorder) error {
	ingressClassCreator := controllers.NewIngressClassCreator(k8sManager.GetClient(), IngressClassName, namespace, recorder)

	if err := k8sManager.Add(ingressClassCreator); err != nil {
		return fmt.Errorf("failed to add ingress class creator as runnable to the manager: %w", err)
	}

	return nil
}

func handleWarpMenuCreation(k8sManager manager.Manager, registry registry.Registry, namespace string, recorder record.EventRecorder) error {
	warpMenuCreator := controllers.NewWarpMenuCreator(k8sManager.GetClient(), registry, namespace, recorder)

	if err := k8sManager.Add(warpMenuCreator); err != nil {
		return fmt.Errorf("failed to add warp menu creator as runnable to the manager: %w", err)
	}

	return nil
}

func handleSslUpdates(k8sManager manager.Manager, namespace string, reg registry.Registry, recorder record.EventRecorder) error {
	sslUpdater := controllers.NewSslCertificateUpdater(k8sManager.GetClient(), namespace, reg, recorder)

	if err := k8sManager.Add(sslUpdater); err != nil {
		return fmt.Errorf("failed to add ssl certificate updater as runnable to the manager: %w", err)
	}

	return nil
}

func handleMaintenanceMode(k8sManager manager.Manager, namespace string, updater controllers.IngressUpdater, recorder record.EventRecorder) error {
	maintenanceModeUpdater, err := controllers.NewMaintenanceModeUpdater(k8sManager.GetClient(), namespace, updater, recorder)
	if err != nil {
		return fmt.Errorf("failed to create new maintenance updater: %w", err)
	}

	if err = k8sManager.Add(maintenanceModeUpdater); err != nil {
		return fmt.Errorf("failed to add maintenance updater as runnable to the manager: %w", err)
	}

	return nil
}

func configureReconciler(k8sManager manager.Manager, ingressUpdater controllers.IngressUpdater) error {
	reconciler := controllers.NewServiceReconciler(k8sManager.GetClient(), ingressUpdater)
	if err := reconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup service discovery with the manager: %w", err)
	}

	deploymentReconciler := controllers.NewDeploymentReconciler(k8sManager.GetClient(), ingressUpdater)
	if err := deploymentReconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup deployment reconciler with the manager: %w", err)
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

func createRouter(etcdRegistry registry.Registry) *gin.Engine {
	router := gin.New()
	router.Use(ginlogrus.Logger(logrus.StandardLogger()), gin.Recovery())
	logger.Info("Setup ssl api")
	ssl.SetupAPI(router, etcdRegistry)
	return router
}
