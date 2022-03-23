//go:build k8s_integration
// +build k8s_integration

/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/controllers"
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

	//create initial ingress class
	ingressClassCreator, err := controllers.NewIngressClassCreator(k8sManager.GetClient(), myIngressClassName)
	Expect(err).ToNot(HaveOccurred())

	err = k8sManager.Add(ingressClassCreator)
	Expect(err).ToNot(HaveOccurred())

	ingressCreator := controllers.NewIngressGenerator(k8sManager.GetClient(), myNamespace, myIngressClassName)
	reconciler := &controllers.ServiceReconciler{
		Client:         k8sManager.GetClient(),
		Scheme:         k8sManager.GetScheme(),
		IngressCreator: ingressCreator,
	}
	err = reconciler.SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
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
