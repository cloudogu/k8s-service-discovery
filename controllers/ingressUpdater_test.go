package controllers

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestNewIngressUpdater(t *testing.T) {
	t.Parallel()

	// given
	clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
	ctrl.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	// when
	creator, err := NewIngressUpdater(clientMock, "my-namespace", "my-ingress-class-name")

	// then
	require.NoError(t, err)
	assert.NotNil(t, creator)
}

func Test_ingressUpdater_UpdateIngressOfService(t *testing.T) {
	t.Parallel()
	ctrl.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	myNamespace := "my-test-namespace"
	myIngressClass := "my-ingress-class"

	t.Run("skipped as service has no ports", func(t *testing.T) {
		// given
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.UpdateIngressOfService(context.TODO(), &service, false)

		//then
		require.NoError(t, err)
	})

	t.Run("skipped as no annotation exist", func(t *testing.T) {
		// given
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.UpdateIngressOfService(context.TODO(), &service, false)

		//then
		require.NoError(t, err)
	})

	t.Run("error when annotation contains invalid ces service", func(t *testing.T) {
		// given
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					CesServiceAnnotation: "invalid json",
				},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.UpdateIngressOfService(context.TODO(), &service, false)

		//then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal ces services")
	})

	t.Run("successfully create/update ingress object", func(t *testing.T) {
		// given
		cesService := []CesService{
			{
				Name:     "test",
				Port:     55,
				Location: "/myLocation",
				Pass:     "/myPass",
			},
		}
		cesServiceString, _ := json.Marshal(cesService)

		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					CesServiceAnnotation: string(cesServiceString),
				},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.UpdateIngressOfService(context.TODO(), &service, false)

		//then
		require.NoError(t, err)
	})
}

func Test_ingressUpdater_createCesServiceIngress(t *testing.T) {
	t.Parallel()
	ctrl.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	myNamespace := "my-test-namespace"
	myIngressClass := "my-ingress-class"

	t.Run("Create ingress resource for a single ces service", func(t *testing.T) {
		// given
		cesServiceWithOneWebapp := CesService{
			Name:     "test",
			Port:     12345,
			Location: "/myLocation",
			Pass:     "/myPass",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.updateServiceIngressObject(context.TODO(), cesServiceWithOneWebapp, &service, false)

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
		assert.Equal(t, cesServiceWithOneWebapp.Pass, ingressResource.Annotations[ingressRewriteTargetAnnotation])
	})

	t.Run("Create ingress resource for a single ces service while maintenance mode is active", func(t *testing.T) {
		// given
		cesServiceWithOneWebapp := CesService{
			Name:     "test",
			Port:     12345,
			Location: "/myLocation",
			Pass:     "/myPass",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.updateServiceIngressObject(context.TODO(), cesServiceWithOneWebapp, &service, true)

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
		assert.Equal(t, "nginx-static", ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
		assert.Equal(t, int32(80), ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
		assert.Equal(t, "/errors/503.html", ingressResource.Annotations[ingressRewriteTargetAnnotation])
	})

	t.Run("Update an existing ingress object with new ces service data", func(t *testing.T) {
		// given
		cesService := CesService{
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
					ingressRewriteTargetAnnotation: "/myOldPass",
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
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.updateServiceIngressObject(context.TODO(), cesService, &service, false)
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
		assert.Equal(t, cesService.Pass, ingressResource.Annotations[ingressRewriteTargetAnnotation])
	})
}
