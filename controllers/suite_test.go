//go:build k8s_integration
// +build k8s_integration

package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/k8s-dogu-operator/v3/api/ecoSystem"
	doguv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/ssl"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ *rest.Config
var k8sApiClient client.Client
var cancel context.CancelFunc
var testEnv *envtest.Environment

var oldGetConfig func() (*rest.Config, error)
var oldGetConfigOrDie func() *rest.Config

var stage string

const (
	myNamespace           = "my-test-namespace"
	myIngressClassName    = "my-ingress-class-name"
	mockRewriteAnnotation = "rewrite"
	mockRegexAnnotation   = "regex"
	mockProxyAnnotation   = "proxybodysize"
)

var (
	FqdnChannel chan repository.GlobalConfigWatchResult
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var ctx context.Context
	ctx, cancel = context.WithCancel(context.TODO())

	testScheme := scheme.Scheme
	err := doguv2.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "vendor", "github.com", "cloudogu", "k8s-dogu-operator", "v3", "api", "v2")},
		ErrorIfCRDPathMissing: true,
		Scheme:                testScheme,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	oldGetConfig = ctrl.GetConfig
	ctrl.GetConfig = func() (*rest.Config, error) {
		return cfg, nil
	}

	oldGetConfigOrDie = ctrl.GetConfigOrDie
	ctrl.GetConfigOrDie = func() *rest.Config {
		return cfg
	}

	// +kubebuilder:scaffold:scheme
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: testScheme,
		Cache: cache.Options{DefaultNamespaces: map[string]cache.Config{
			myNamespace: {},
		}},
	})
	Expect(err).ToNot(HaveOccurred())

	eventRecorder := k8sManager.GetEventRecorderFor("k8s-service-discovery-controller-manager")

	t := GinkgoT()
	globalConfig := config.CreateGlobalConfig(config.Entries{
		"certificate/server.crt": "mycert",
		"certificate/server.key": "mykey",
		"certificate/type":       "selfsigned",
		"fqdn":                   "example.com",
		"domain":                 "example.com",
	})
	globalConfigRepoMock := NewMockGlobalConfigRepository(t)
	globalConfigRepoMock.EXPECT().Get(mock.Anything).Return(globalConfig, nil)

	ecosystemClient, err := ecoSystem.NewForConfig(k8sManager.GetConfig())
	Expect(err).ToNot(HaveOccurred())
	doguInterface := ecosystemClient.Dogus(myNamespace)

	ingressControllerMock := newMockIngressController(t)
	ingressControllerMock.EXPECT().GetRewriteAnnotationKey().Return(mockRewriteAnnotation)
	ingressControllerMock.EXPECT().GetProxyBodySizeKey().Return(mockProxyAnnotation)
	ingressControllerMock.EXPECT().GetUseRegexKey().Return(mockRegexAnnotation)
	ingressControllerMock.EXPECT().DeleteExposedPorts(mock.Anything, myNamespace, "nexus").Return(nil)
	ingressControllerMock.EXPECT().GetName().Return("nginx-ingress")

	clientSet, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())

	ingressCreator := expose.NewIngressUpdater(clientSet, doguInterface, globalConfigRepoMock, myNamespace, myIngressClassName, eventRecorder, ingressControllerMock)
	Expect(err).ToNot(HaveOccurred())

	exposedPortUpdater := expose.NewExposedPortHandler(clientSet.CoreV1().Services(myNamespace), ingressControllerMock, myNamespace)
	networkPolicyUpdater := expose.NewNetworkPolicyHandler(clientSet.NetworkingV1().NetworkPolicies(myNamespace), ingressControllerMock, "0.0.0.0/0")

	serviceReconciler := &serviceReconciler{
		client:                 k8sManager.GetClient(),
		ingressUpdater:         ingressCreator,
		exposedPortUpdater:     exposedPortUpdater,
		networkPolicyUpdater:   networkPolicyUpdater,
		networkPoliciesEnabled: true,
	}
	err = serviceReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	deploymentReconciler := &deploymentReconciler{
		client:  k8sManager.GetClient(),
		updater: ingressCreator,
	}
	err = deploymentReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	secretInterface := clientSet.CoreV1().Secrets(myNamespace)
	globalConfigRepoMock.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, globalConfig config.GlobalConfig) (config.GlobalConfig, error) {
		serverCrt, exists := globalConfig.Get("certificate/server.crt")
		Expect(exists).To(BeTrue())
		Expect(serverCrt).To(ContainSubstring("-----BEGIN CERTIFICATE-----"))
		_, exists = globalConfig.Get("certificate/server.key")
		Expect(exists).To(BeFalse())

		return config.CreateGlobalConfig(globalConfig.GetAll()), nil
	})
	certSync := ssl.NewCertificateSynchronizer(secretInterface, globalConfigRepoMock)
	certificateReconciler := NewEcosystemCertificateReconciler(certSync)
	err = certificateReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// create initial ingress class
	ingressControllerMock.EXPECT().GetControllerSpec().Return("k8s.io/nginx-ingress")
	ingressClassCreator := expose.NewIngressClassCreator(clientSet, myIngressClassName, myNamespace, eventRecorder, ingressControllerMock)
	err = k8sManager.Add(ingressClassCreator)
	Expect(err).ToNot(HaveOccurred())

	err = k8sManager.Add(certSync)
	Expect(err).ToNot(HaveOccurred())

	FqdnChannel = make(chan repository.GlobalConfigWatchResult)
	globalConfigRepoMock.EXPECT().Watch(mock.Anything, mock.Anything).Return(FqdnChannel, nil)
	updater := NewSelfsignedCertificateUpdater(myNamespace, globalConfigRepoMock, secretInterface)
	err = k8sManager.Add(updater)
	Expect(err).ToNot(HaveOccurred())

	// create warp menu creator
	stage = os.Getenv("STAGE")
	err = os.Unsetenv("STAGE")
	Expect(err).NotTo(HaveOccurred())

	createInitialTestData(k8sManager.GetClient())
	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sApiClient, err = client.New(cfg, client.Options{Scheme: testScheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sApiClient).NotTo(BeNil())

}, 60)

func createInitialTestData(client client.Client) {
	createTestNamespace(client)
	createMenuJsonConfig(client)
	createSelfDeployment(client)
}

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())

	ctrl.GetConfig = oldGetConfig
	ctrl.GetConfigOrDie = oldGetConfigOrDie
	err = os.Setenv("STAGE", stage)
	Expect(err).NotTo(HaveOccurred())
})

func createTestNamespace(client client.Client) {
	By(fmt.Sprintf("Create %s", myNamespace))
	namespace := corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: myNamespace},
		Spec:       corev1.NamespaceSpec{},
		Status:     corev1.NamespaceStatus{},
	}
	err := client.Create(context.Background(), &namespace)
	Expect(err).NotTo(HaveOccurred())
}

func createSelfDeployment(client client.Client) {
	labels := make(map[string]string)
	labels["app"] = "ces"
	selfDeploy := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "k8s-service-discovery-controller-manager",
			Namespace: myNamespace,
			Labels:    labels,
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "testcontainer", Image: "testimage"},
					},
				},
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
			},
		},
	}
	Expect(client.Create(context.Background(), selfDeploy)).Should(Succeed())
}

func createMenuJsonConfig(client client.Client) {
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "k8s-ces-warp-config", Namespace: myNamespace}}
	Expect(client.Create(context.Background(), cm)).Should(Succeed())
}
