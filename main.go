package main

import (
	"flag"
	"fmt"
	"github.com/cloudogu/k8s-dogu-operator/v2/api/ecoSystem"
	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/expose"
	"github.com/cloudogu/k8s-service-discovery/controllers/expose/ingressController"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	ginlogrus "github.com/toorop/gin-logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports

	"github.com/cloudogu/k8s-dogu-operator/v2/api/v2"

	"github.com/cloudogu/k8s-service-discovery/controllers"
	"github.com/cloudogu/k8s-service-discovery/controllers/logging"
	"github.com/cloudogu/k8s-service-discovery/controllers/ssl"
)

const (
	IngressClassName = "k8s-ecosystem-ces-service"
	apiPort          = 9090
)

var (
	scheme               = runtime.NewScheme()
	logger               = ctrl.Log.WithName("k8s-service-discovery.main")
	metricsAddr          string
	enableLeaderElection bool
	probeAddr            string
)

type k8sManager interface {
	manager.Manager
}

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v2.AddToScheme(scheme))
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

	watchNamespace, err := config.ReadWatchNamespace()

	options := getK8sManagerOptions(watchNamespace)
	if err != nil {
		return fmt.Errorf("failed to get manager options: %w", err)
	}

	serviceDiscManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return fmt.Errorf("failed to create new manager: %w", err)
	}

	eventRecorder := serviceDiscManager.GetEventRecorderFor("k8s-service-discovery-controller-manager")

	ingressControllerStr := config.ReadIngressController()
	controller := ingressController.ParseIngressController(ingressControllerStr)

	clientset, err := getK8sClientSet(serviceDiscManager.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create k8s client set: %w", err)
	}

	if err = handleIngressClassCreation(serviceDiscManager, clientset, watchNamespace, eventRecorder, controller); err != nil {
		return fmt.Errorf("failed to create ingress class creator: %w", err)
	}

	configMapInterface := clientset.CoreV1().ConfigMaps(watchNamespace)
	doguVersionRegistry := dogu.NewDoguVersionRegistry(configMapInterface)
	localDoguRepo := dogu.NewLocalDoguDescriptorRepository(configMapInterface)
	globalConfigRepo := repository.NewGlobalConfigRepository(configMapInterface)

	provideSSLAPI(globalConfigRepo)

	if err = handleWarpMenuCreation(serviceDiscManager, doguVersionRegistry, localDoguRepo, watchNamespace, eventRecorder, globalConfigRepo); err != nil {
		return fmt.Errorf("failed to create warp menu creator: %w", err)
	}

	if err = handleSslUpdates(serviceDiscManager, watchNamespace, globalConfigRepo, eventRecorder); err != nil {
		return fmt.Errorf("failed to create ssl certificate updater: %w", err)
	}

	if err = handleSelfsignedCertificateUpdates(serviceDiscManager, watchNamespace, globalConfigRepo, eventRecorder); err != nil {
		return fmt.Errorf("failed to create selfsigned certificate updater: %w", err)
	}

	ecoSystemClientSet, err := ecoSystem.NewForConfig(serviceDiscManager.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create ecosystem client set: %w", err)
	}

	ingressUpdater := expose.NewIngressUpdater(clientset, ecoSystemClientSet.Dogus(watchNamespace), globalConfigRepo, watchNamespace, IngressClassName, eventRecorder, controller)

	if err = handleMaintenanceMode(serviceDiscManager, watchNamespace, ingressUpdater, eventRecorder, globalConfigRepo); err != nil {
		return err
	}

	if err = configureManager(serviceDiscManager, ingressUpdater); err != nil {
		return fmt.Errorf("failed to configure service discovery manager: %w", err)
	}

	if err = startK8sManager(serviceDiscManager); err != nil {
		return fmt.Errorf("failure at service discovery manager: %w", err)
	}

	return nil
}

func getK8sClientSet(config *rest.Config) (*kubernetes.Clientset, error) {
	k8sClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client set: %w", err)
	}

	return k8sClientSet, nil
}

func provideSSLAPI(globalConfigRepo controllers.GlobalConfigRepository) {
	go func() {
		router := createSSLAPIRouter(globalConfigRepo)
		err := router.Run(fmt.Sprintf(":%d", apiPort))
		if err != nil {
			logger.Error(fmt.Errorf("failed to start gin router %w", err), "SSL api error")
		}
	}()
}

func configureManager(k8sManager k8sManager, updater controllers.IngressUpdater) error {
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

func getK8sManagerOptions(watchNamespace string) manager.Options {
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	return ctrl.Options{
		Scheme:  scheme,
		Metrics: server.Options{BindAddress: metricsAddr},
		Cache: cache.Options{DefaultNamespaces: map[string]cache.Config{
			watchNamespace: {},
		}},
		WebhookServer:          webhook.NewServer(webhook.Options{Port: 9443}),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "92a787f2.cloudogu.com",
	}
}

func startK8sManager(k8sManager k8sManager) error {
	logger.Info("starting service discovery manager")

	err := k8sManager.Start(ctrl.SetupSignalHandler())
	if err != nil {
		return fmt.Errorf("failed to start service discovery manager: %w", err)
	}

	return nil
}

func handleIngressClassCreation(k8sManager k8sManager, clientSet *kubernetes.Clientset, namespace string, recorder record.EventRecorder, controller ingressController.IngressController) error {
	ingressClassCreator := expose.NewIngressClassCreator(clientSet, IngressClassName, namespace, recorder, controller)

	if err := k8sManager.Add(ingressClassCreator); err != nil {
		return fmt.Errorf("failed to add ingress class creator as runnable to the manager: %w", err)
	}

	return nil
}

func handleWarpMenuCreation(k8sManager k8sManager, doguVersionRegistry warp.DoguVersionRegistry, localDoguRepo warp.LocalDoguRepo, namespace string, recorder record.EventRecorder, globalConfigRepo warp.GlobalConfigRepository) error {
	warpMenuCreator := controllers.NewWarpMenuCreator(k8sManager.GetClient(), doguVersionRegistry, localDoguRepo, namespace, recorder, globalConfigRepo)

	if err := k8sManager.Add(warpMenuCreator); err != nil {
		return fmt.Errorf("failed to add warp menu creator as runnable to the manager: %w", err)
	}

	return nil
}

func handleSslUpdates(k8sManager k8sManager, namespace string, globalConfigRepo controllers.GlobalConfigRepository, recorder record.EventRecorder) error {
	sslUpdater := controllers.NewSslCertificateUpdater(k8sManager.GetClient(), namespace, globalConfigRepo, recorder)

	if err := k8sManager.Add(sslUpdater); err != nil {
		return fmt.Errorf("failed to add ssl certificate updater as runnable to the manager: %w", err)
	}

	return nil
}

func handleSelfsignedCertificateUpdates(k8sManager k8sManager, namespace string, globalConfigRepo controllers.GlobalConfigRepository, recorder record.EventRecorder) error {
	selfsignedCertificateUpdater := controllers.NewSelfsignedCertificateUpdater(k8sManager.GetClient(), namespace, globalConfigRepo, recorder)

	if err := k8sManager.Add(selfsignedCertificateUpdater); err != nil {
		return fmt.Errorf("failed to add selfsigned certificate updater as runnable to the manager: %w", err)
	}

	return nil
}

func handleMaintenanceMode(k8sManager k8sManager, namespace string, updater controllers.IngressUpdater, recorder record.EventRecorder, globalConfigRepo *repository.GlobalConfigRepository) error {
	maintenanceModeUpdater, err := controllers.NewMaintenanceModeUpdater(k8sManager.GetClient(), namespace, updater, recorder, globalConfigRepo)
	if err != nil {
		return fmt.Errorf("failed to create new maintenance updater: %w", err)
	}

	if err = k8sManager.Add(maintenanceModeUpdater); err != nil {
		return fmt.Errorf("failed to add maintenance updater as runnable to the manager: %w", err)
	}

	return nil
}

func configureReconciler(k8sManager k8sManager, ingressUpdater controllers.IngressUpdater) error {
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

func addChecks(mgr k8sManager) error {
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to add healthz check: %w", err)
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("failed to add readyz check: %w", err)
	}

	return nil
}

func createSSLAPIRouter(globalConfigRepo controllers.GlobalConfigRepository) *gin.Engine {
	router := gin.New()
	router.Use(ginlogrus.Logger(logrus.StandardLogger()), gin.Recovery())
	logger.Info("Setup ssl api")
	ssl.SetupAPI(router, globalConfigRepo)
	return router
}
