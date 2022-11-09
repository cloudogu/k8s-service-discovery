package controllers

import (
	"context"
	"encoding/json"
	"github.com/cloudogu/k8s-service-discovery/controllers/dogustart"
	"github.com/cloudogu/k8s-service-discovery/controllers/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	ctrl "sigs.k8s.io/controller-runtime"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

func TestNewIngressUpdater(t *testing.T) {
	t.Parallel()

	t.Run("fail when getting the config", func(t *testing.T) {
		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		ctrl.GetConfig = func() (*rest.Config, error) {
			return &rest.Config{}, assert.AnError
		}

		// when
		_, err := NewIngressUpdater(clientMock, "my-namespace", "my-ingress-class-name")

		// then
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("fail when getting creating client with invalid config", func(t *testing.T) {
		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		ctrl.GetConfig = func() (*rest.Config, error) {
			return &rest.Config{
				ExecProvider: &api.ExecConfig{},
				AuthProvider: &api.AuthProviderConfig{},
			}, nil
		}

		// when
		_, err := NewIngressUpdater(clientMock, "my-namespace", "my-ingress-class-name")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create client set: execProvider and authProvider cannot be used in combination")
	})
	t.Run("successfully create/update ingress object", func(t *testing.T) {
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
	})
}

func Test_ingressUpdater_UpdateIngressOfService(t *testing.T) {
	ctrl.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	myNamespace := "my-test-namespace"
	myIngressClass := "my-ingress-class"
	ctx := context.Background()

	t.Run("skipped as service has no ports", func(t *testing.T) {
		// given
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, creationError := NewIngressUpdater(clientMock, myNamespace, myIngressClass)
		require.NoError(t, creationError)

		// when
		err := creator.UpsertIngressForService(ctx, &service, false)

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
		err := creator.UpsertIngressForService(ctx, &service, false)

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
		err := creator.UpsertIngressForService(ctx, &service, false)

		//then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal ces services")
	})
	t.Run("error when updating service ingress object", func(t *testing.T) {
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

		deploymentReadyChecker := mocks.NewDeploymentReadyChecker(t)
		deploymentReadyChecker.On("IsReady", ctx, "test").Return(false, assert.AnError)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.UpsertIngressForService(ctx, &service, false)

		//then
		require.ErrorIs(t, err, assert.AnError)
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

		deploymentReadyChecker := mocks.NewDeploymentReadyChecker(t)
		deploymentReadyChecker.On("IsReady", ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		deploymentReadyReactor := mocks.NewDeploymentReadyReactor(t)
		creator.deploymentReadyReactor = deploymentReadyReactor

		// when
		err := creator.UpsertIngressForService(ctx, &service, false)

		//then
		require.NoError(t, err)
	})
}

func Test_ingressUpdater_updateServiceIngressObject(t *testing.T) {
	ctrl.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}
	myNamespace := "my-test-namespace"
	myIngressClass := "my-ingress-class"
	ctx := context.Background()

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

		deploymentReadyChecker := mocks.NewDeploymentReadyChecker(t)
		deploymentReadyChecker.On("IsReady", ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		deploymentReadyReactor := mocks.NewDeploymentReadyReactor(t)
		creator.deploymentReadyReactor = deploymentReadyReactor

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		//then
		require.NoError(t, err)
		ingressResource := &networking.Ingress{}
		ingressResourceKey := types.NamespacedName{
			Namespace: myNamespace,
			Name:      cesServiceWithOneWebapp.Name,
		}

		err = clientMock.Get(ctx, ingressResourceKey, ingressResource)
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

		deploymentReadyChecker := mocks.NewDeploymentReadyChecker(t)
		creator.deploymentReadyChecker = deploymentReadyChecker

		deploymentReadyReactor := mocks.NewDeploymentReadyReactor(t)
		creator.deploymentReadyReactor = deploymentReadyReactor

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, true)

		//then
		require.NoError(t, err)
		ingressResource := &networking.Ingress{}
		ingressResourceKey := types.NamespacedName{
			Namespace: myNamespace,
			Name:      cesServiceWithOneWebapp.Name,
		}

		err = clientMock.Get(ctx, ingressResourceKey, ingressResource)
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
	t.Run("Failed to wait for deployment to be ready -> stuck at dogu is staring ingress object", func(t *testing.T) {
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

		deploymentReadyChecker := mocks.NewDeploymentReadyChecker(t)
		deploymentReadyChecker.On("IsReady", ctx, "test").Return(false, nil).Once()
		creator.deploymentReadyChecker = deploymentReadyChecker

		deploymentReadyReactor := mocks.NewDeploymentReadyReactor(t)
		waitOptions := dogustart.WaitOptions{Timeout: waitForDeploymentTimeout, TickRate: waitForDeploymentTickRate}
		deploymentReadyReactor.On("WaitForReady", ctx, "test", waitOptions, mock.Anything).Return(assert.AnError)
		creator.deploymentReadyReactor = deploymentReadyReactor

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		//then
		require.NoError(t, err)
		ingressResource := &networking.Ingress{}
		ingressResourceKey := types.NamespacedName{
			Namespace: myNamespace,
			Name:      cesServiceWithOneWebapp.Name,
		}

		// wait for WaitForReady goroutine to finish so that the mock detects its execution.
		time.Sleep(time.Millisecond * 200)

		err = clientMock.Get(ctx, ingressResourceKey, ingressResource)
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
		assert.Equal(t, "/errors/starting.html", ingressResource.Annotations[ingressRewriteTargetAnnotation])
	})
	t.Run("Create dogu ingress after waiting for deployment to be ready", func(t *testing.T) {
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

		deploymentReadyChecker := mocks.NewDeploymentReadyChecker(t)
		deploymentReadyChecker.On("IsReady", ctx, "test").Return(false, nil).Once()
		creator.deploymentReadyChecker = deploymentReadyChecker

		deploymentReadyReactor := mocks.NewDeploymentReadyReactor(t)
		waitOptions := dogustart.WaitOptions{Timeout: waitForDeploymentTimeout, TickRate: waitForDeploymentTickRate}
		deploymentReadyReactor.On("WaitForReady", ctx, "test", waitOptions, mock.Anything).Run(func(args mock.Arguments) {
			onReadyFunction, ok := args[3].(func(context.Context))
			require.True(t, ok)

			deploymentReadyChecker.On("IsReady", ctx, "test").Return(true, nil)

			onReadyFunction(ctx)
		}).Return(nil)
		creator.deploymentReadyReactor = deploymentReadyReactor

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		//then
		require.NoError(t, err)
		ingressResource := &networking.Ingress{}
		ingressResourceKey := types.NamespacedName{
			Namespace: myNamespace,
			Name:      cesServiceWithOneWebapp.Name,
		}

		// wait for WaitForReady goroutine to finish so that the mock detects its execution.
		time.Sleep(time.Millisecond * 200)

		err = clientMock.Get(ctx, ingressResourceKey, ingressResource)
		require.NoError(t, err)

		assert.Equal(t, myNamespace, ingressResource.Namespace)
		assert.Equal(t, "Service", ingressResource.OwnerReferences[0].Kind)
		assert.Equal(t, service.GetName(), ingressResource.OwnerReferences[0].Name)
		assert.Equal(t, cesServiceWithOneWebapp.Name, ingressResource.Name)
		assert.Equal(t, myIngressClass, *ingressResource.Spec.IngressClassName)
		assert.Equal(t, cesServiceWithOneWebapp.Location, ingressResource.Spec.Rules[0].HTTP.Paths[0].Path)
		assert.Equal(t, networking.PathTypePrefix, *ingressResource.Spec.Rules[0].HTTP.Paths[0].PathType)
		assert.Equal(t, "test", ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
		assert.Equal(t, int32(12345), ingressResource.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Number)
		assert.Equal(t, "/myPass", ingressResource.Annotations[ingressRewriteTargetAnnotation])
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

		deploymentReadyChecker := mocks.NewDeploymentReadyChecker(t)
		deploymentReadyChecker.On("IsReady", ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		deploymentReadyReactor := mocks.NewDeploymentReadyReactor(t)
		creator.deploymentReadyReactor = deploymentReadyReactor

		// when
		err := creator.upsertIngressForCesService(ctx, cesService, &service, false)
		require.NoError(t, err)

		//then
		ingressResourceList := &networking.IngressList{}
		err = clientMock.List(ctx, ingressResourceList)
		require.NoError(t, err)
		assert.Len(t, ingressResourceList.Items, 1)

		ingressResource := &networking.Ingress{}
		ingressResourceKey := types.NamespacedName{
			Namespace: myNamespace,
			Name:      cesService.Name,
		}

		err = clientMock.Get(ctx, ingressResourceKey, ingressResource)
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
