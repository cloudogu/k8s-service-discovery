package controllers_test

import (
	"context"
	"testing"

	networking "k8s.io/api/networking/v1"

	"k8s.io/apimachinery/pkg/types"

	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	"github.com/cloudogu/k8s-service-discovery/controllers"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIngressGenerator_CreateCesServiceIngress(t *testing.T) {
	t.Parallel()
	myNamespace := "my-test-namespace"
	myIngressClass := "my-ingress-class"

	t.Run("Create ingress resource for a single ces service", func(t *testing.T) {
		t.Parallel()

		// given
		cesServiceWithOneWebapp := controllers.CesService{
			Name:     "test",
			Port:     12345,
			Location: "/myLocation",
			Pass:     "/myPass",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator := controllers.NewIngressGenerator(clientMock, myNamespace, myIngressClass)

		// when
		err := creator.CreateCesServiceIngress(context.TODO(), cesServiceWithOneWebapp, &service)

		//then
		require.NoError(t, err)
		ingressResource := &networking.Ingress{}
		ingressResourceKey := types.NamespacedName{
			Namespace: myNamespace,
			Name:      cesServiceWithOneWebapp.Name,
		}

		err = clientMock.Get(context.TODO(), ingressResourceKey, ingressResource)
		require.NoError(t, err)

		assert.Equal(t, myNamespace, ingressResource.Namespace)
		assert.Equal(t, "Service", ingressResource.OwnerReferences[0].Kind)
		assert.Equal(t, service.GetName(), ingressResource.OwnerReferences[0].Name)
		assert.Equal(t, cesServiceWithOneWebapp.Name, ingressResource.Name)
		assert.Equal(t, myIngressClass, *ingressResource.Spec.IngressClassName)
		assert.Equal(t, cesServiceWithOneWebapp.Location, ingressResource.Spec.Rules[0].HTTP.Paths[0].Path)
		assert.Equal(t, networking.PathTypePrefix, *ingressResource.Spec.Rules[0].HTTP.Paths[0].PathType)
		assert.Equal(t, service.GetName(), ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
		assert.Equal(t, int32(cesServiceWithOneWebapp.Port), ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
		assert.Equal(t, cesServiceWithOneWebapp.Pass, ingressResource.Annotations[controllers.IngressRewriteTargetAnnotation])
	})

	t.Run("Update an existing ingress object with new ces service data", func(t *testing.T) {
		t.Parallel()

		// given
		cesService := controllers.CesService{
			Name:     "test",
			Port:     12345,
			Location: "/myNewLocation",
			Pass:     "/myNewPass",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		pathType := networking.PathTypePrefix
		existingIngress := &networking.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cesService.Name,
				Namespace: myNamespace,
				Annotations: map[string]string{
					controllers.IngressRewriteTargetAnnotation: "/myOldPass",
				},
			},
			Spec: networking.IngressSpec{
				IngressClassName: &myIngressClass,
				Rules: []networking.IngressRule{{
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{
							Paths: []networking.HTTPIngressPath{{Path: "/myOldLocation",
								PathType: &pathType,
								Backend: networking.IngressBackend{
									Service: &networking.IngressServiceBackend{
										Name: service.GetName(),
										Port: networking.ServiceBackendPort{
											Number: int32(cesService.Port),
										},
									}}}}}}}}},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(existingIngress).Build()
		creator := controllers.NewIngressGenerator(clientMock, myNamespace, myIngressClass)

		// when
		err := creator.CreateCesServiceIngress(context.TODO(), cesService, &service)
		require.NoError(t, err)

		//then
		ingressResourceList := &networking.IngressList{}
		err = clientMock.List(context.TODO(), ingressResourceList)
		require.NoError(t, err)
		assert.Len(t, ingressResourceList.Items, 1)

		ingressResource := &networking.Ingress{}
		ingressResourceKey := types.NamespacedName{
			Namespace: myNamespace,
			Name:      cesService.Name,
		}

		err = clientMock.Get(context.TODO(), ingressResourceKey, ingressResource)
		require.NoError(t, err)

		assert.Equal(t, myNamespace, ingressResource.Namespace)
		assert.Equal(t, "Service", ingressResource.OwnerReferences[0].Kind)
		assert.Equal(t, service.GetName(), ingressResource.OwnerReferences[0].Name)
		assert.Equal(t, cesService.Name, ingressResource.Name)
		assert.Equal(t, myIngressClass, *ingressResource.Spec.IngressClassName)
		assert.Equal(t, cesService.Location, ingressResource.Spec.Rules[0].HTTP.Paths[0].Path)
		assert.Equal(t, networking.PathTypePrefix, *ingressResource.Spec.Rules[0].HTTP.Paths[0].PathType)
		assert.Equal(t, service.GetName(), ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
		assert.Equal(t, int32(cesService.Port), ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
		assert.Equal(t, cesService.Pass, ingressResource.Annotations[controllers.IngressRewriteTargetAnnotation])
	})
}

func TestNewIngressGenerator(t *testing.T) {
	t.Parallel()

	// given
	clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()

	// when
	creator := controllers.NewIngressGenerator(clientMock, "my-namespace", "my-ingress-class-name")

	// then
	assert.NotNil(t, creator)
}
