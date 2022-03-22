//go:build k8s_integration
// +build k8s_integration

package controllers_test

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

//go:embed testdata/service_no_annotations.yaml
var serviceNoAnnotationsBytes []byte
var serviceNoAnnotations = &corev1.Service{}

//go:embed testdata/service_Type1_WebApp.yaml
var serviceWebAppBytes []byte
var serviceWebApp = &corev1.Service{}

//go:embed testdata/service_Type2_AdditionalService.yaml
var serviceAdditionalBytes []byte
var serviceAdditional = &corev1.Service{}

func init() {
	err := yaml.Unmarshal(serviceNoAnnotationsBytes, serviceNoAnnotations)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(serviceWebAppBytes, serviceWebApp)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(serviceAdditionalBytes, serviceAdditional)
	if err != nil {
		panic(err)
	}
}

var _ = Describe("Creating ingress objects with the ingress generator", func() {
	const timeout = time.Second * 10
	const interval = time.Second * 1
	ctx := context.TODO()

	Context("Handle new service resource", func() {
		It("Should do nothing if service without annotations", func() {
			By("Creating service with no annotations")
			Expect(k8sClient.Create(ctx, serviceNoAnnotations)).Should(Succeed())

			By("Expect no ingress resource")
			expectedIngress := &networking.IngressList{}

			Eventually(func() bool {
				err := k8sClient.List(ctx, expectedIngress)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(len(expectedIngress.Items)).Should(Equal(0))

			cleanup()
		})

		It("Should create ingress object for simple webapp service", func() {
			By("Creating service with ces annotations")
			Expect(k8sClient.Create(ctx, serviceWebApp)).Should(Succeed())

			By("Expect exactly one ingress resource for the service")
			expectedIngress := &networking.IngressList{}

			Eventually(func() bool {
				err := k8sClient.List(ctx, expectedIngress)
				if err != nil {
					fmt.Errorf("%w", err)
					return false
				}
				fmt.Printf("%+v", expectedIngress)

				return len(expectedIngress.Items) == 1
			}, timeout, interval).Should(BeTrue())

			cleanup()
		})
	})
})

func cleanup() {
	const timeout = time.Second * 10
	const interval = time.Second * 1
	ctx := context.TODO()

	By("Cleanup all ingresses")
	ingressesList := &networking.IngressList{}
	Eventually(func() bool {
		err := k8sClient.List(ctx, ingressesList)
		if err != nil {
			return false
		}

		for _, ingress := range ingressesList.Items {
			err = k8sClient.Delete(ctx, &ingress)
			if err != nil {
				return false
			}
		}

		return true
	}, timeout, interval).Should(BeTrue())

	By("Cleanup all services")
	servicesList := &corev1.ServiceList{}
	Eventually(func() bool {
		err := k8sClient.List(ctx, servicesList)
		if err != nil {
			return false
		}

		for _, service := range servicesList.Items {
			err = k8sClient.Delete(ctx, &service)
			if err != nil {
				return false
			}
		}

		return true
	}, timeout, interval).Should(BeTrue())
}
