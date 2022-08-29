//go:build k8s_integration
// +build k8s_integration

package controllers

import (
	"context"
	"fmt"
	cesmocks "github.com/cloudogu/cesapp-lib/registry/mocks"
	etcdclient "github.com/coreos/etcd/client"
	"path/filepath"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var _ *rest.Config
var k8sClient client.Client
var cancel context.CancelFunc
var testEnv *envtest.Environment

const myNamespace = "my-test-namespace"
const myIngressClassName = "my-ingress-class-name"

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	var ctx context.Context
	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "bases")},
		ErrorIfCRDPathMissing: false,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	//+kubebuilder:scaffold:scheme
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:    scheme.Scheme,
		Namespace: myNamespace,
	})
	Expect(err).ToNot(HaveOccurred())

	myRegistry := &cesmocks.Registry{}
	globalConfigMock := &cesmocks.ConfigurationContext{}
	keyNotFoundErr := etcdclient.Error{Code: etcdclient.ErrorCodeKeyNotFound}
	globalConfigMock.On("Get", "maintenance").Return("", keyNotFoundErr)
	myRegistry.On("GlobalConfig").Return(globalConfigMock, nil)

	ingressCreator := NewIngressUpdater(k8sManager.GetClient(), myNamespace, myIngressClassName)
	reconciler := &serviceReconciler{
		client:   k8sManager.GetClient(),
		registry: myRegistry,
		updater:  ingressCreator,
	}
	err = reconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	// create initial ingress class
	ingressClassCreator := NewIngressClassCreator(k8sManager.GetClient(), myIngressClassName)
	err = k8sManager.Add(ingressClassCreator)
	Expect(err).ToNot(HaveOccurred())

	// create ssl updater class
	sslUpdater, err := NewSslCertificateUpdater(k8sManager.GetClient(), myNamespace)
	Expect(err).ToNot(HaveOccurred())
	err = k8sManager.Add(sslUpdater)
	Expect(err).ToNot(HaveOccurred())

	//// create warp menu creator
	//warpMenuCreator := NewWarpMenuCreator(k8sManager.GetClient(), myRegistry, myNamespace)
	//err = k8sManager.Add(warpMenuCreator)
	//Expect(err).ToNot(HaveOccurred())

	// create maintenance updater
	maintenanceUpdater, err := NewMaintenanceModeUpdater(k8sManager.GetClient(), myNamespace, ingressCreator)
	Expect(err).ToNot(HaveOccurred())
	err = k8sManager.Add(maintenanceUpdater)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())

		defer GinkgoRecover()
	}()

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	By(fmt.Sprintf("Create %s", myNamespace))
	namespace := corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{Name: myNamespace},
		Spec:       corev1.NamespaceSpec{},
		Status:     corev1.NamespaceStatus{},
	}
	err = k8sClient.Create(context.Background(), &namespace)
	Expect(err).NotTo(HaveOccurred())

}, 60)

var _ = AfterSuite(func() {
	cancel()
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
