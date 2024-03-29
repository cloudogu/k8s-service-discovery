//go:build k8s_integration
// +build k8s_integration

package controllers

import (
	"context"
	"fmt"
	cesmocks "github.com/cloudogu/cesapp-lib/registry/mocks"
	doguv1 "github.com/cloudogu/k8s-dogu-operator/api/v1"
	"github.com/stretchr/testify/mock"
	etcdclient "go.etcd.io/etcd/client/v2"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const myNamespace = "my-test-namespace"
const myIngressClassName = "my-ingress-class-name"

var SSLChannel chan *etcdclient.Response

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var ctx context.Context
	ctx, cancel = context.WithCancel(context.TODO())

	testScheme := scheme.Scheme
	err := doguv1.AddToScheme(testScheme)
	Expect(err).NotTo(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "vendor", "github.com", "cloudogu", "k8s-dogu-operator", "api", "v1")},
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

	myRegistry := &cesmocks.Registry{}
	globalConfigMock := &cesmocks.ConfigurationContext{}
	keyNotFoundErr := etcdclient.Error{Code: etcdclient.ErrorCodeKeyNotFound}
	globalConfigMock.On("Get", "maintenance").Return("", keyNotFoundErr)
	myRegistry.On("GlobalConfig").Return(globalConfigMock, nil)

	eventRecorder := k8sManager.GetEventRecorderFor("k8s-service-discovery-controller-manager")

	ingressCreator, err := NewIngressUpdater(k8sManager.GetClient(), globalConfigMock, myNamespace, myIngressClassName, eventRecorder)
	Expect(err).ToNot(HaveOccurred())

	serviceReconciler := &serviceReconciler{
		client:  k8sManager.GetClient(),
		updater: ingressCreator,
	}
	err = serviceReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	deploymentReconciler := &deploymentReconciler{
		client:  k8sManager.GetClient(),
		updater: ingressCreator,
	}
	err = deploymentReconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// create initial ingress class
	ingressClassCreator := NewIngressClassCreator(k8sManager.GetClient(), myIngressClassName, myNamespace, eventRecorder)
	err = k8sManager.Add(ingressClassCreator)
	Expect(err).ToNot(HaveOccurred())

	watchRegistry := &cesmocks.WatchConfigurationContext{}
	myRegistry.On("RootConfig").Return(watchRegistry)

	globalConfigMock.On("Get", "certificate/server.crt").Return("mycrt", nil)
	globalConfigMock.On("Get", "certificate/server.key").Return("mykey", nil)

	// create ssl updater class
	watchRegistry.On("Watch", mock.Anything, "/config/_global/certificate", true, mock.Anything).Run(func(args mock.Arguments) {
		SSLChannel = args.Get(3).(chan *etcdclient.Response)
	})

	sslUpdater := NewSslCertificateUpdater(k8sManager.GetClient(), myNamespace, myRegistry, eventRecorder)
	err = k8sManager.Add(sslUpdater)
	Expect(err).ToNot(HaveOccurred())

	// create warp menu creator
	stage = os.Getenv("STAGE")
	err = os.Unsetenv("STAGE")
	Expect(err).NotTo(HaveOccurred())

	watchRegistry.On("Watch", mock.Anything, "/dogu", mock.Anything, mock.Anything)
	watchRegistry.On("Watch", mock.Anything, "/config/nginx/externals", mock.Anything, mock.Anything)
	watchRegistry.On("Watch", mock.Anything, "/config/_global/disabled_warpmenu_support_entries", mock.Anything, mock.Anything)

	warpMenuCreator := NewWarpMenuCreator(k8sManager.GetClient(), myRegistry, myNamespace, eventRecorder)
	err = k8sManager.Add(warpMenuCreator)
	Expect(err).ToNot(HaveOccurred())

	// create maintenance updater
	maintenanceUpdater, err := NewMaintenanceModeUpdater(k8sManager.GetClient(), myNamespace, ingressCreator, eventRecorder)
	Expect(err).ToNot(HaveOccurred())
	err = k8sManager.Add(maintenanceUpdater)
	Expect(err).ToNot(HaveOccurred())

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
