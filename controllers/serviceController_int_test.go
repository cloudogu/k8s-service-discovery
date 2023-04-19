//go:build k8s_integration
// +build k8s_integration

package controllers

import (
	"context"
	_ "embed"
	"fmt"
	doguv1 "github.com/cloudogu/k8s-dogu-operator/api/v1"
	"github.com/cloudogu/k8s-service-discovery/controllers/dogustart"
	etcdclient "go.etcd.io/etcd/client/v2"
	"k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"sigs.k8s.io/yaml"
)

const (
	timeoutInterval = time.Second * 10
	pollingInterval = time.Second * 1
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

var serviceDeployment = &v1.Deployment{}

func resetData() {
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

	serviceDeployment = &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nexus",
			Namespace: "my-test-namespace",
			Labels:    map[string]string{"dogu.name": "nexus"},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"dogu.name": "nexus"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"dogu.name": "nexus"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "nexus-123123", Image: "nginx:1.7.9"},
					},
				},
			},
		},
		Status: v1.DeploymentStatus{
			Replicas:      0,
			ReadyReplicas: 0,
		},
	}
}

var _ = Describe("Creating ingress objects with the ingress generator", func() {
	ctx := context.Background()
	resetData()

	Context("Handle new service resource", func() {
		AfterEach(cleanup)

		It("Should do nothing if service without annotations", func() {
			By("Create dogu")
			dogu := &doguv1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "nexus", Namespace: myNamespace}}
			Expect(k8sApiClient.Create(context.Background(), dogu)).Should(Succeed())
			Eventually(func() bool {
				err := k8sApiClient.Get(ctx, types.NamespacedName{Name: "nexus", Namespace: myNamespace}, &doguv1.Dogu{})
				return err == nil
			}, timeoutInterval, pollingInterval).Should(BeTrue())

			By("Creating service with no annotations")
			Expect(k8sApiClient.Create(ctx, serviceNoAnnotations)).Should(Succeed())

			By("Expect no ingress resource")
			expectedIngress := &networking.IngressList{}

			Eventually(func() bool {
				err := k8sApiClient.List(ctx, expectedIngress)
				return err == nil
			}, timeoutInterval, pollingInterval).Should(BeTrue())

			Expect(expectedIngress.Items).Should(HaveLen(0))
		})

		It("Should create ingress object for simple webapp service", func() {
			By("Create deployment for service (which is not ready)")
			Expect(k8sApiClient.Create(ctx, serviceDeployment)).Should(Succeed())

			By("Creating service with ces annotations")
			Expect(k8sApiClient.Create(ctx, serviceWebApp)).Should(Succeed())

			By("Expect exactly one ingress resource for the dogu is starting ingress object")
			expectedIngress := &networking.IngressList{}

			Eventually(func() bool {
				err := k8sApiClient.List(ctx, expectedIngress)
				if err != nil {
					_ = fmt.Errorf("%w", err)
					return false
				}

				return len(expectedIngress.Items) == 1
			}, timeoutInterval, pollingInterval).Should(BeTrue())

			Expect(expectedIngress.Items[0].Namespace).Should(Equal(myNamespace))
			Expect(expectedIngress.Items[0].Name).Should(Equal("nexus"))
			Expect(*expectedIngress.Items[0].Spec.IngressClassName).Should(Equal(myIngressClassName))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Path).Should(Equal("/nexus"))
			Expect(*expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].PathType).Should(Equal(networking.PathTypePrefix))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).Should(Equal("nginx-static"))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number).Should(Equal(int32(80)))

			value, ok := expectedIngress.Items[0].Annotations[ingressRewriteTargetAnnotation]
			Expect(ok).Should(BeTrue())
			Expect(value).Should(Equal("/errors/starting.html"))

			By("Wait for deployment to become ready")
			serviceDeployment.Status.ReadyReplicas = 1
			serviceDeployment.Status.Replicas = 1
			err := k8sApiClient.Status().Update(ctx, serviceDeployment)
			Expect(err).NotTo(HaveOccurred())

			client, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
			Expect(err).NotTo(HaveOccurred())
			deploymentWaiter := dogustart.NewDeploymentReadyChecker(client, "my-test-namespace")
			waitOptions := dogustart.WaitOptions{Timeout: time.Minute * 30, TickRate: time.Millisecond * 200}
			err = deploymentWaiter.WaitForReady(ctx, "nexus", waitOptions, func(ctx context.Context) {})
			Expect(err).NotTo(HaveOccurred())

			By("Wait until async onReady method was called")
			time.Sleep(time.Second * 2)

			By("Expect exactly one ingress resource for the service")
			expectedIngress = &networking.IngressList{}
			Eventually(func() bool {
				err := k8sApiClient.List(ctx, expectedIngress)
				if err != nil {
					_ = fmt.Errorf("%w", err)
					return false
				}

				return len(expectedIngress.Items) == 1
			}, timeoutInterval, pollingInterval).Should(BeTrue())

			Expect(expectedIngress.Items[0].Namespace).Should(Equal(myNamespace))
			Expect(expectedIngress.Items[0].Name).Should(Equal("nexus"))
			Expect(*expectedIngress.Items[0].Spec.IngressClassName).Should(Equal(myIngressClassName))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Path).Should(Equal("/nexus"))
			Expect(*expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].PathType).Should(Equal(networking.PathTypePrefix))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).Should(Equal("nexus"))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number).Should(Equal(int32(8082)))

			_, ok = expectedIngress.Items[0].Annotations[ingressRewriteTargetAnnotation]
			Expect(ok).Should(BeFalse())
		})

		It("Should create ingress object for multiple ces services", func() {
			By("Create deployment for service (which is not ready)")
			Expect(k8sApiClient.Create(ctx, serviceDeployment)).Should(Succeed())

			By("Wait for deployment to become ready")
			serviceDeployment.Status.ReadyReplicas = 1
			serviceDeployment.Status.Replicas = 1
			err := k8sApiClient.Status().Update(ctx, serviceDeployment)
			Expect(err).NotTo(HaveOccurred())

			client, err := kubernetes.NewForConfig(ctrl.GetConfigOrDie())
			Expect(err).NotTo(HaveOccurred())
			deploymentWaiter := dogustart.NewDeploymentReadyChecker(client, "my-test-namespace")
			waitOptions := dogustart.WaitOptions{Timeout: time.Minute * 1, TickRate: time.Second * 1}
			err = deploymentWaiter.WaitForReady(ctx, "nexus", waitOptions, func(ctx context.Context) {})
			Expect(err).NotTo(HaveOccurred())

			By("Creating service with multiple ces services")
			Expect(k8sApiClient.Create(ctx, serviceAdditional)).Should(Succeed())

			By("Expect exactly two ingress resource for the service")
			expectedIngress := &networking.IngressList{}

			Eventually(func() bool {
				err := k8sApiClient.List(ctx, expectedIngress)
				if err != nil {
					_ = fmt.Errorf("%w", err)
					return false
				}

				return len(expectedIngress.Items) == 2
			}, timeoutInterval, pollingInterval).Should(BeTrue())

			Expect(expectedIngress.Items[0].Namespace).Should(Equal(myNamespace))
			Expect(expectedIngress.Items[0].Name).Should(Equal("nexus"))
			Expect(*expectedIngress.Items[0].Spec.IngressClassName).Should(Equal(myIngressClassName))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Path).Should(Equal("/nexus"))
			Expect(*expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].PathType).Should(Equal(networking.PathTypePrefix))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).Should(Equal("nexus"))
			Expect(expectedIngress.Items[0].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number).Should(Equal(int32(8082)))
			Expect(expectedIngress.Items[0].Annotations).Should(HaveKeyWithValue(ingressConfigurationSnippetAnnotation, "proxy_set_header Accept-Encoding \"identity\";\n"))
			Expect(expectedIngress.Items[0].Annotations).Should(HaveKeyWithValue("example-key", "example-value"))
			Expect(expectedIngress.Items[0].Annotations).Should(Not(HaveKey(ingressRewriteTargetAnnotation)))

			Expect(expectedIngress.Items[1].Namespace).Should(Equal(myNamespace))
			Expect(expectedIngress.Items[1].Name).Should(Equal("nexus-docker-registry"))
			Expect(*expectedIngress.Items[1].Spec.IngressClassName).Should(Equal(myIngressClassName))
			Expect(expectedIngress.Items[1].Spec.Rules[0].HTTP.Paths[0].Path).Should(Equal("/v2"))
			Expect(*expectedIngress.Items[1].Spec.Rules[0].HTTP.Paths[0].PathType).Should(Equal(networking.PathTypePrefix))
			Expect(expectedIngress.Items[1].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name).Should(Equal("nexus"))
			Expect(expectedIngress.Items[1].Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number).Should(Equal(int32(8082)))
			Expect(expectedIngress.Items[1].Annotations).Should(HaveKeyWithValue(ingressRewriteTargetAnnotation, "/nexus/repository/docker-registry/v2"))
			Expect(expectedIngress.Items[1].Annotations).Should(HaveKeyWithValue(ingressConfigurationSnippetAnnotation, "proxy_set_header Accept-Encoding \"identity\";\n"))
			Expect(expectedIngress.Items[1].Annotations).Should(HaveKeyWithValue("example-key", "example-value"))
		})

		It("Should create ssl cert", func() {
			By("Create test data")
			createSelfDeployment(k8sApiClient)

			By("Trigger channel")
			SSLChannel <- &etcdclient.Response{}

			By("Expect ssl secret")
			Eventually(func() bool {
				secret := &corev1.Secret{}
				err := k8sApiClient.Get(ctx, types.NamespacedName{Name: "ecosystem-certificate", Namespace: myNamespace}, secret)
				if err != nil {
					return false
				}

				data := secret.StringData
				if data[corev1.TLSCertKey] == "mycert" && data[corev1.TLSPrivateKeyKey] == "mykey" {
					return true
				}

				return true
			}, timeoutInterval, pollingInterval).Should(BeTrue())
		})
	})
})

var _ = Describe("Ingress class should be created automatically", func() {
	ctx := context.TODO()
	Context("Ingress class should be created automatically", func() {
		It("Ingress class should be created automatically", func() {
			By("Check for creation of ingress class")
			ingressClassID := types.NamespacedName{
				Name: myIngressClassName,
			}
			expectedIngressClass := &networking.IngressClass{}

			Eventually(func() bool {
				err := k8sApiClient.Get(ctx, ingressClassID, expectedIngressClass)
				if err != nil {
					return false
				}

				return expectedIngressClass != nil
			}, timeoutInterval, pollingInterval).Should(BeTrue())
		})
	})
})

func cleanup() {
	ctx := context.Background()
	resetData()

	By("Cleanup all ingresses")
	ingressesList := &networking.IngressList{}
	Eventually(func() bool {
		err := k8sApiClient.List(ctx, ingressesList)
		if err != nil {
			return false
		}

		for _, ingress := range ingressesList.Items {
			err = k8sApiClient.Delete(ctx, &ingress)
			if err != nil {
				return false
			}
		}

		return true
	}, timeoutInterval, pollingInterval).Should(BeTrue())

	By("Cleanup all services")
	servicesList := &corev1.ServiceList{}
	Eventually(func() bool {
		err := k8sApiClient.List(ctx, servicesList)
		if err != nil {
			return false
		}

		for _, service := range servicesList.Items {
			err = k8sApiClient.Delete(ctx, &service)
			if err != nil {
				return false
			}
		}

		return true
	}, timeoutInterval, pollingInterval).Should(BeTrue())

	By("Cleanup all deployments")
	deploymentList := &v1.DeploymentList{}
	Eventually(func() bool {
		err := k8sApiClient.List(ctx, deploymentList)
		if err != nil {
			return false
		}

		for _, deployment := range deploymentList.Items {
			err = k8sApiClient.Delete(ctx, &deployment)
			if err != nil {
				return false
			}
		}

		return true
	}, timeoutInterval, pollingInterval).Should(BeTrue())
}
