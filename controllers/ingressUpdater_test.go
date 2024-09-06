package controllers

import (
	"context"
	"encoding/json"
	v1 "github.com/cloudogu/k8s-dogu-operator/api/v1"
	"github.com/cloudogu/k8s-dogu-operator/controllers/annotation"
	registryconfig "github.com/cloudogu/k8s-registry-lib/config"
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

func getGlobalConfigRepoMockWithMaintenance(t *testing.T, maintenanceMode bool) GlobalConfigRepository {
	var entries registryconfig.Entries

	if maintenanceMode {
		entries = registryconfig.Entries{
			"maintenance": "maintenance",
		}
	} else {
		entries = registryconfig.Entries{}
	}

	globalConfigRepoMock := NewMockGlobalConfigRepository(t)
	globalConfig := registryconfig.GlobalConfig{
		Config: registryconfig.CreateConfig(entries),
	}
	globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

	return globalConfigRepoMock
}

func TestNewIngressUpdater(t *testing.T) {
	t.Parallel()

	t.Run("fail when getting the config", func(t *testing.T) {
		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		ctrl.GetConfig = func() (*rest.Config, error) {
			return &rest.Config{}, assert.AnError
		}

		// when
		_, err := NewIngressUpdater(clientMock, nil, "my-namespace", "my-ingress-class-name", newMockEventRecorder(t))

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
		_, err := NewIngressUpdater(clientMock, nil, "my-namespace", "my-ingress-class-name", newMockEventRecorder(t))

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
		creator, err := NewIngressUpdater(clientMock, nil, "my-namespace", "my-ingress-class-name", newMockEventRecorder(t))

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
		creator, creationError := NewIngressUpdater(clientMock, getGlobalConfigRepoMockWithMaintenance(t, true), myNamespace, myIngressClass, newMockEventRecorder(t))
		require.NoError(t, creationError)

		// when
		err := creator.UpsertIngressForService(ctx, &service)

		// then
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
		creator, creationError := NewIngressUpdater(clientMock, getGlobalConfigRepoMockWithMaintenance(t, false), myNamespace, myIngressClass, newMockEventRecorder(t))
		require.NoError(t, creationError)

		// when
		err := creator.UpsertIngressForService(ctx, &service)

		// then
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
		creator, creationError := NewIngressUpdater(clientMock, getGlobalConfigRepoMockWithMaintenance(t, false), myNamespace, myIngressClass, newMockEventRecorder(t))
		require.NoError(t, creationError)

		// when
		err := creator.UpsertIngressForService(ctx, &service)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal ces services")
	})
	t.Run("error when fetching the dogu", func(t *testing.T) {
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
				Name:      "test",
				Namespace: myNamespace,
				Annotations: map[string]string{
					CesServiceAnnotation: string(cesServiceString),
				},
				Labels: map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects().Build()
		creator, creationError := NewIngressUpdater(clientMock, getGlobalConfigRepoMockWithMaintenance(t, false), myNamespace, myIngressClass, newMockEventRecorder(t))
		require.NoError(t, creationError)

		// when
		err := creator.UpsertIngressForService(ctx, &service)

		// then
		assert.ErrorContains(t, err, "failed to create ingress object for ces service [{Name:test Port:55 Location:/myLocation Pass:/myPass Rewrite:}]")
		assert.ErrorContains(t, err, "not found")
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
				Name:      "test",
				Namespace: myNamespace,
				Annotations: map[string]string{
					CesServiceAnnotation: string(cesServiceString),
				},
				Labels: map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		creator, creationError := NewIngressUpdater(clientMock, getGlobalConfigRepoMockWithMaintenance(t, false), myNamespace, myIngressClass, newMockEventRecorder(t))
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(false, assert.AnError)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.UpsertIngressForService(ctx, &service)

		// then
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
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}

		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&v1.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", "test")
		creator, creationError := NewIngressUpdater(clientMock, getGlobalConfigRepoMockWithMaintenance(t, false), myNamespace, myIngressClass, recorderMock)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.UpsertIngressForService(ctx, &service)

		// then
		require.NoError(t, err)
	})
}

func Test_ingressUpdater_upsertIngressForCesService(t *testing.T) {
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
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": "test"}},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&v1.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", "test")

		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, recorderMock)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		// then
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
		assert.Equal(t, map[string]string{
			ingressConfigurationSnippetAnnotation: "proxy_set_header Accept-Encoding \"identity\";",
			ingressRewriteTargetAnnotation:        cesServiceWithOneWebapp.Pass,
		}, ingressResource.Annotations)
	})
	t.Run("Create ingress resource for a single ces service with rewrite", func(t *testing.T) {
		// given
		cesServiceWithOneWebapp := CesService{
			Name:     "test",
			Port:     12345,
			Location: "/myLocation",
			Pass:     "/myPass",
			Rewrite:  "{\"pattern\":\"myPattern\",\"rewrite\":\"\"}",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": "test"}},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&v1.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", "test")

		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, recorderMock)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		// then
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
		assert.Equal(t, map[string]string{
			ingressConfigurationSnippetAnnotation: "proxy_set_header Accept-Encoding \"identity\";\nrewrite ^/myPattern(/|$)(.*) /$2 break;",
			ingressRewriteTargetAnnotation:        cesServiceWithOneWebapp.Pass,
		}, ingressResource.Annotations)
	})
	t.Run("Create ingress resource for a single ces service with additional ingress annotations", func(t *testing.T) {
		// given
		cesServiceWithOneWebapp := CesService{
			Name:     "test",
			Port:     12345,
			Location: "/myLocation",
			Pass:     "/myPass",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": "test"},
				Annotations: map[string]string{
					annotation.AdditionalIngressAnnotationsAnnotation: "{\"nginx.org/client-max-body-size\":\"100m\",\"example-annotation\":\"example-value\"}",
				},
			},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&v1.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", "test")

		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, recorderMock)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		// then
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
		assert.Equal(t, map[string]string{
			ingressConfigurationSnippetAnnotation: "proxy_set_header Accept-Encoding \"identity\";",
			ingressRewriteTargetAnnotation:        cesServiceWithOneWebapp.Pass,
			"nginx.org/client-max-body-size":      "100m",
			"example-annotation":                  "example-value",
		}, ingressResource.Annotations)
	})
	t.Run("Fail to create ingress resource for a single ces service with invalid additional ingress annotations", func(t *testing.T) {
		// given
		cesServiceWithOneWebapp := CesService{
			Name:     "test",
			Port:     12345,
			Location: "/myLocation",
			Pass:     "/myPass",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": "test"},
				Annotations: map[string]string{
					annotation.AdditionalIngressAnnotationsAnnotation: "{{{{\"nginx.org/client-max-body-size\":\"100m\",\"example-annotation\":\"example-value\"}",
				},
			},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()

		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, nil)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get addtional ingress annotations from dogu service 'test': invalid character '{' looking for beginning of object key string")
	})
	t.Run("Create default ingress for nginx-static dogu even when maintenance mode is active", func(t *testing.T) {
		doguName := "nginx-static"

		// given
		cesServiceWithOneWebapp := CesService{
			Name:     doguName,
			Port:     12345,
			Location: "/myLocation",
			Pass:     "/myPass",
		}
		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      doguName,
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": doguName}},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: doguName, Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&v1.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", doguName)

		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, recorderMock)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, doguName).Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, true)

		// then
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
		assert.Equal(t, map[string]string{
			ingressConfigurationSnippetAnnotation: "proxy_set_header Accept-Encoding \"identity\";",
			ingressRewriteTargetAnnotation:        cesServiceWithOneWebapp.Pass,
		}, ingressResource.Annotations)
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
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&v1.Dogu{}), "Normal", "IngressCreation", "Ingress for service [%s] has been updated to maintenance mode.", "test")
		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, recorderMock)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, true)

		// then
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
		assert.Equal(t, map[string]string{
			ingressRewriteTargetAnnotation: "/errors/503.html",
		}, ingressResource.Annotations)
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
			ObjectMeta: metav1.ObjectMeta{Name: "test",
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": "test"}},
		}
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu).Build()
		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, newMockEventRecorder(t))
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(false, nil).Once()
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesServiceWithOneWebapp, &service, false)

		// then
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
		assert.Equal(t, map[string]string{
			ingressRewriteTargetAnnotation: "/errors/starting.html",
		}, ingressResource.Annotations)
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
			ObjectMeta: metav1.ObjectMeta{Name: "test",
				Namespace: myNamespace,
				Labels:    map[string]string{"dogu.name": "test"}},
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
		dogu := &v1.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: myNamespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(dogu, existingIngress).Build()
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&v1.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", "test")

		creator, creationError := NewIngressUpdater(clientMock, nil, myNamespace, myIngressClass, recorderMock)
		require.NoError(t, creationError)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(ctx, "test").Return(true, nil)
		creator.deploymentReadyChecker = deploymentReadyChecker

		// when
		err := creator.upsertIngressForCesService(ctx, cesService, &service, false)
		require.NoError(t, err)

		// then
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
		assert.Equal(t, map[string]string{
			ingressConfigurationSnippetAnnotation: "proxy_set_header Accept-Encoding \"identity\";",
			ingressRewriteTargetAnnotation:        cesService.Pass,
		}, ingressResource.Annotations)
	})
}

func TestCesService_generateRewriteConfig(t *testing.T) {
	tests := []struct {
		name    string
		rewrite string
		want    string
		wantErr func(t *testing.T, err error)
	}{
		{
			name:    "should fail to unmarshal invalid rewrite",
			rewrite: "{\"pattern\": \"portainer\", \"rewrite\": \"\"",
			want:    "",
			wantErr: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "failed to read service rewrite from ces service: unexpected end of JSON input")
			},
		},
		{
			name:    "should succeed to generate rewrite config",
			rewrite: "{\"pattern\": \"portainer\", \"rewrite\": \"p\"}",
			want:    "rewrite ^/portainer(/|$)(.*) p/$2 break;",
			wantErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			cs := CesService{Rewrite: tt.rewrite}
			// when
			got, err := cs.generateRewriteConfig()
			// then
			tt.wantErr(t, err)
			assert.Equalf(t, tt.want, got, "generateRewriteConfig()")
		})
	}
}
