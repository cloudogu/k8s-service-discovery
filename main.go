package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/cloudogu/k8s-dogu-operator/v3/api/ecoSystem"
	"github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/dogustart"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/ingressController"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/logging"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/ssl"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	networkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

const (
	// TODO Should be configurable because other component creates it
	IngressClassName = "k8s-ecosystem-ces-service"
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
	if err != nil {
		return fmt.Errorf("failed to read watch namespace: %w", err)
	}

	ingressControllerStr := config.ReadIngressController()

	options := getK8sManagerOptions(watchNamespace)
	serviceDiscManager, err := ctrl.NewManager(ctrl.GetConfigOrDie(), options)
	if err != nil {
		return fmt.Errorf("failed to create new manager: %w", err)
	}

	eventRecorder := serviceDiscManager.GetEventRecorderFor("k8s-service-discovery-controller-manager")

	clientSet, err := getK8sClientSet(serviceDiscManager.GetConfig(), watchNamespace)
	if err != nil {
		return fmt.Errorf("failed to create k8s client set: %w", err)
	}

	controller := ingressController.ParseIngressController(ingressController.Dependencies{
		Controller:         ingressControllerStr,
		ConfigMapInterface: clientSet.configMapClient,
		IngressInterface:   clientSet.ingressClient,
		IngressClassName:   IngressClassName,
	})

	globalConfigRepo := repository.NewGlobalConfigRepository(clientSet.configMapClient)
	certSync := ssl.NewCertificateSynchronizer(clientSet.secretClient, globalConfigRepo)

	if err = handleCertificateSynchronization(serviceDiscManager, certSync); err != nil {
		return fmt.Errorf("failed to create certificate key remover: %w", err)
	}

	if err = handleSelfsignedCertificateUpdates(serviceDiscManager, watchNamespace, globalConfigRepo, clientSet.secretClient); err != nil {
		return fmt.Errorf("failed to create selfsigned certificate updater: %w", err)
	}

	ecoSystemClientSet, err := ecoSystem.NewForConfig(serviceDiscManager.GetConfig())
	if err != nil {
		return fmt.Errorf("failed to create ecosystem client set: %w", err)
	}

	deploymentReadyChecker := dogustart.NewDeploymentReadyChecker(clientSet.k8sClient, watchNamespace)

	ingressUpdater := expose.NewIngressUpdater(expose.IngressUpdaterDependencies{
		DeploymentReadyChecker: deploymentReadyChecker,
		IngressInterface:       clientSet.ingressClient,
		DoguInterface:          ecoSystemClientSet.Dogus(watchNamespace),
		GlobalConfigRepo:       globalConfigRepo,
		Namespace:              watchNamespace,
		IngressClassName:       IngressClassName,
		Recorder:               eventRecorder,
		Controller:             controller,
	})

	if err = handleMaintenanceMode(serviceDiscManager, watchNamespace, ingressUpdater, eventRecorder, globalConfigRepo); err != nil {
		return err
	}

	cidr, err := config.ReadNetworkPolicyCIDR()
	if err != nil {
		return err
	}

	networkpoliciesEnabled, err := config.ReadNetworkPolicyEnabled()
	if err != nil {
		return err
	}

	networkPolicyUpdater := expose.NewNetworkPolicyHandler(clientSet.networkPolicyClient, controller, cidr)

	if err = configureManager(
		serviceDiscManager,
		clientSet,
		globalConfigRepo,
		controller,
		ingressUpdater,
		networkPolicyUpdater,
		networkpoliciesEnabled,
		certSync,
	); err != nil {
		return fmt.Errorf("failed to configure service discovery manager: %w", err)
	}

	if err = startK8sManager(serviceDiscManager); err != nil {
		return fmt.Errorf("failure at service discovery manager: %w", err)
	}

	return nil
}

type k8sClientSet struct {
	k8sClient           *kubernetes.Clientset
	configMapClient     v1.ConfigMapInterface
	secretClient        v1.SecretInterface
	serviceClient       v1.ServiceInterface
	deploymentClient    appsv1.DeploymentInterface
	ingressClient       networkingv1.IngressInterface
	networkPolicyClient networkingv1.NetworkPolicyInterface
}

func getK8sClientSet(config *rest.Config, namespace string) (k8sClientSet, error) {
	k8sClients, err := kubernetes.NewForConfig(config)
	if err != nil {
		return k8sClientSet{}, fmt.Errorf("failed to create k8s client set: %w", err)
	}

	return k8sClientSet{
		k8sClient:           k8sClients,
		configMapClient:     k8sClients.CoreV1().ConfigMaps(namespace),
		secretClient:        k8sClients.CoreV1().Secrets(namespace),
		serviceClient:       k8sClients.CoreV1().Services(namespace),
		deploymentClient:    k8sClients.AppsV1().Deployments(namespace),
		ingressClient:       k8sClients.NetworkingV1().Ingresses(namespace),
		networkPolicyClient: k8sClients.NetworkingV1().NetworkPolicies(namespace),
	}, nil
}

type certificateSynchronizer interface {
	Synchronize(ctx context.Context) error
}

func configureManager(
	k8sManager k8sManager,
	k8sClients k8sClientSet,
	globalConfigRepo controllers.GlobalConfigRepository,
	ingressController controllers.IngressController,
	ingressUpdater controllers.IngressUpdater,
	networkPolicyUpdater controllers.NetworkPolicyUpdater,
	networkPoliciesEnabled bool,
	certSync certificateSynchronizer,
) error {
	if err := configureReconciler(
		k8sManager,
		k8sClients,
		globalConfigRepo,
		ingressController,
		ingressUpdater,
		networkPolicyUpdater,
		networkPoliciesEnabled,
		certSync,
	); err != nil {
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

func handleCertificateSynchronization(k8sManager k8sManager, certificateSynchronizer manager.Runnable) error {
	if err := k8sManager.Add(certificateSynchronizer); err != nil {
		return fmt.Errorf("failed to add certificate key remover as runnable to the manager: %w", err)
	}

	return nil
}

func handleSelfsignedCertificateUpdates(k8sManager k8sManager, namespace string, globalConfigRepo controllers.GlobalConfigRepository, secretClient v1.SecretInterface) error {
	selfsignedCertificateUpdater := controllers.NewSelfsignedCertificateUpdater(namespace, globalConfigRepo, secretClient)

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

func configureReconciler(
	k8sManager k8sManager,
	k8sClients k8sClientSet,
	globalConfigRepo controllers.GlobalConfigRepository,
	ingressController controllers.IngressController,
	ingressUpdater controllers.IngressUpdater,
	networkPolicyUpdater controllers.NetworkPolicyUpdater,
	networkPoliciesEnabled bool,
	certSync certificateSynchronizer,
) error {
	reconciler := controllers.NewServiceReconciler(k8sManager.GetClient(), ingressUpdater, networkPolicyUpdater, networkPoliciesEnabled)
	if err := reconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup service discovery with the manager: %w", err)
	}

	deploymentReconciler := controllers.NewDeploymentReconciler(k8sManager.GetClient(), ingressUpdater)
	if err := deploymentReconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup deployment reconciler with the manager: %w", err)
	}

	ecosystemCertificateReconciler := controllers.NewEcosystemCertificateReconciler(certSync)
	if err := ecosystemCertificateReconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup ecosystem certificate reconciler with the manager: %w", err)
	}

	redirectReconciler := &controllers.RedirectReconciler{
		Client:             k8sManager.GetClient(),
		GlobalConfigGetter: globalConfigRepo,
		Redirector:         ingressController,
	}

	if err := redirectReconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup redirct reconciler with the manager: %w", err)
	}

	loadbalacnerReconciler := &controllers.LoadBalancerReconciler{
		Client:            k8sManager.GetClient(),
		IngressController: ingressController,
		SvcClient:         k8sClients.serviceClient,
	}

	if err := loadbalacnerReconciler.SetupWithManager(k8sManager); err != nil {
		return fmt.Errorf("failed to setup loadbalancer reconciler with the manager: %w", err)
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
