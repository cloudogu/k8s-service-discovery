package expose

import (
	"context"
	"encoding/json"
	doguv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-dogu-operator/v3/controllers/annotation"
	registryconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
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

const (
	testNamespace        = "my-namespace"
	testIngressClassName = "my-ingress-class-name"
)

func TestNewIngressUpdater(t *testing.T) {
	t.Run("successfully create ingress updater", func(t *testing.T) {
		// given
		clientSetMock := newMockClientSetInterface(t)
		ingressInterfaceMock := newMockIngressInterface(t)
		netv1Mock := newMockNetInterface(t)
		netv1Mock.EXPECT().Ingresses(testNamespace).Return(ingressInterfaceMock)
		clientSetMock.EXPECT().NetworkingV1().Return(netv1Mock)

		doguInterfaceMock := newMockDoguInterface(t)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		ingressControllerMock := newMockIngressController(t)

		// when
		sut := NewIngressUpdater(clientSetMock, doguInterfaceMock, globalConfigRepoMock, testNamespace, testIngressClassName, newMockEventRecorder(t), ingressControllerMock)

		// then
		assert.NotNil(t, sut)
	})
}

func Test_ingressUpdater_UpdateIngressOfService(t *testing.T) {
	t.Run("skipped as service has no ports", func(t *testing.T) {
		// given
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		}

		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)

		sut := ingressUpdater{
			globalConfigRepo: globalConfigRepoMock,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, service)

		// then
		require.NoError(t, err)
	})
	t.Run("skipped as no annotation exist", func(t *testing.T) {
		// given
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}

		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)

		sut := ingressUpdater{
			globalConfigRepo: globalConfigRepoMock,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, service)

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

		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)

		sut := ingressUpdater{
			globalConfigRepo: globalConfigRepoMock,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, &service)

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

		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: testNamespace,
				Annotations: map[string]string{
					CesServiceAnnotation: string(cesServiceString),
				},
				Labels: map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, service.Name, metav1.GetOptions{}).Return(nil, assert.AnError)

		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)

		sut := ingressUpdater{
			globalConfigRepo: globalConfigRepoMock,
			doguInterface:    doguInterfaceMock,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, service)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create ingress object for ces service [{Name:test Port:55 Location:/myLocation Pass:/myPass Rewrite:}]")
	})
	t.Run("error when updating service ingress object because deployment checker returns an error", func(t *testing.T) {
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
				Namespace: testNamespace,
				Annotations: map[string]string{
					CesServiceAnnotation: string(cesServiceString),
				},
				Labels: map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}
		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}
		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(testCtx, "test").Return(false, assert.AnError)
		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, dogu.Name, metav1.GetOptions{}).Return(dogu, nil)

		sut := ingressUpdater{
			globalConfigRepo:       globalConfigRepoMock,
			deploymentReadyChecker: deploymentReadyChecker,
			doguInterface:          doguInterfaceMock,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, &service)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create ingress object for ces service")
	})
	t.Run("successfully create ingress object", func(t *testing.T) {
		// given
		cesService := []CesService{
			{
				Name:     "test",
				Port:     55,
				Location: "/myLocation",
				Pass:     "/myPass",
				Rewrite:  "{\"pattern\":\"myPattern\",\"rewrite\":\"\"}",
			},
		}
		cesServiceString, _ := json.Marshal(cesService)

		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					CesServiceAnnotation: string(cesServiceString),
				},
				Namespace: testNamespace,
				Labels:    map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}

		expectedIngress := getTestIngress("test", "myPattern", service, service.Name, 55, map[string]string{
			"rewrite": "",
			"regex":   "true",
		})

		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&doguv2.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", "test")
		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(testCtx, "test").Return(true, nil)
		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, dogu.Name, metav1.GetOptions{}).Return(dogu, nil)
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().GetRewriteAnnotationKey().Return("rewrite")
		ingressControllerMock.EXPECT().GetUseRegexKey().Return("regex")
		ingressInterfaceMock := newMockIngressInterface(t)
		ingressInterfaceMock.EXPECT().Get(testCtx, expectedIngress.Name, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		ingressInterfaceMock.EXPECT().Create(testCtx, expectedIngress, metav1.CreateOptions{}).Return(nil, nil)

		sut := ingressUpdater{
			globalConfigRepo:       globalConfigRepoMock,
			deploymentReadyChecker: deploymentReadyChecker,
			eventRecorder:          recorderMock,
			doguInterface:          doguInterfaceMock,
			controller:             ingressControllerMock,
			ingressInterface:       ingressInterfaceMock,
			namespace:              testNamespace,
			ingressClassName:       testIngressClassName,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, &service)

		// then
		require.NoError(t, err)
	})

	t.Run("successfully update ingress object", func(t *testing.T) {
		// given
		cesService := []CesService{
			{
				Name:     "test",
				Port:     55,
				Location: "/myLocation",
				Pass:     "/myPass",
				Rewrite:  "{\"pattern\":\"myPattern\",\"rewrite\":\"\"}",
			},
		}
		cesServiceString, _ := json.Marshal(cesService)

		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					CesServiceAnnotation:                              string(cesServiceString),
					annotation.AdditionalIngressAnnotationsAnnotation: "{\"nginx.org/client-max-body-size\":\"100m\",\"example-annotation\":\"example-value\"}",
				},
				Namespace: testNamespace,
				Labels:    map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}

		existingIngress := getTestIngress("test", "myPattern", service, service.Name, 44, map[string]string{})

		expectedIngress := getTestIngress("test", "myPattern", service, service.Name, 55, map[string]string{
			"rewrite":                        "",
			"regex":                          "true",
			"nginx.org/client-max-body-size": "100m",
			"example-annotation":             "example-value",
		})

		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&doguv2.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", "test")
		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(testCtx, "test").Return(true, nil)
		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, dogu.Name, metav1.GetOptions{}).Return(dogu, nil)
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().GetRewriteAnnotationKey().Return("rewrite")
		ingressControllerMock.EXPECT().GetUseRegexKey().Return("regex")
		ingressInterfaceMock := newMockIngressInterface(t)
		ingressInterfaceMock.EXPECT().Get(testCtx, expectedIngress.Name, metav1.GetOptions{}).Return(existingIngress, nil)
		ingressInterfaceMock.EXPECT().Update(testCtx, mock.Anything, metav1.UpdateOptions{}).Return(nil, nil).Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.UpdateOptions) {
			assert.Equal(t, ingress, expectedIngress)
		})

		sut := ingressUpdater{
			globalConfigRepo:       globalConfigRepoMock,
			deploymentReadyChecker: deploymentReadyChecker,
			eventRecorder:          recorderMock,
			doguInterface:          doguInterfaceMock,
			controller:             ingressControllerMock,
			ingressInterface:       ingressInterfaceMock,
			namespace:              testNamespace,
			ingressClassName:       testIngressClassName,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, &service)

		// then
		require.NoError(t, err)
	})

	t.Run("fail to update ingress for invalid rewrite config", func(t *testing.T) {
		// given
		cesService := []CesService{
			{
				Name:     "test",
				Port:     55,
				Location: "/myLocation",
				Pass:     "/myPass",
				Rewrite:  "{\"pattern\": \"portainer\", \"rewrite\": \"\"",
			},
		}
		cesServiceString, _ := json.Marshal(cesService)

		service := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
				Annotations: map[string]string{
					CesServiceAnnotation: string(cesServiceString),
				},
				Namespace: testNamespace,
				Labels:    map[string]string{"dogu.name": "test"},
			},
			Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
				{Name: "testPort", Port: 55},
			}},
		}

		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}
		recorderMock := newMockEventRecorder(t)
		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(testCtx, "test").Return(true, nil)
		globalConfigRepoMock := getGlobalConfigRepoMockWithMaintenance(t, false)
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, dogu.Name, metav1.GetOptions{}).Return(dogu, nil)
		ingressControllerMock := newMockIngressController(t)
		ingressInterfaceMock := newMockIngressInterface(t)

		sut := ingressUpdater{
			globalConfigRepo:       globalConfigRepoMock,
			deploymentReadyChecker: deploymentReadyChecker,
			eventRecorder:          recorderMock,
			doguInterface:          doguInterfaceMock,
			controller:             ingressControllerMock,
			ingressInterface:       ingressInterfaceMock,
			namespace:              testNamespace,
			ingressClassName:       testIngressClassName,
		}

		// when
		err := sut.UpsertIngressForService(testCtx, &service)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create ingress object for ces service")
		assert.ErrorContains(t, err, "error getting rewrite-config from ces-service:")
	})
}

func Test_ingressUpdater_upsertIngressForCesService(t *testing.T) {
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
				Namespace: testNamespace,
				Labels:    map[string]string{"dogu.name": "test"},
				Annotations: map[string]string{
					annotation.AdditionalIngressAnnotationsAnnotation: "{{{{\"nginx.org/client-max-body-size\":\"100m\",\"example-annotation\":\"example-value\"}",
				},
			},
		}
		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, service.Name, metav1.GetOptions{}).Return(dogu, nil)

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(testCtx, "test").Return(true, nil)

		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().GetRewriteAnnotationKey().Return("rewrite")
		ingressControllerMock.EXPECT().GetUseRegexKey().Return("regex")

		sut := ingressUpdater{
			deploymentReadyChecker: deploymentReadyChecker,
			doguInterface:          doguInterfaceMock,
			controller:             ingressControllerMock,
		}

		// when
		err := sut.upsertIngressForCesService(testCtx, cesServiceWithOneWebapp, &service, false)

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
				Namespace: testNamespace,
				Labels:    map[string]string{"dogu.name": doguName}},
		}

		expectedIngress := getTestIngress(doguName, "/myLocation(/|$)(.*)", service, service.Name, 12345, map[string]string{
			"rewrite": "/myPass/$2",
			"regex":   "true",
		})

		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: doguName, Namespace: testNamespace}}
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, service.Name, metav1.GetOptions{}).Return(dogu, nil)
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&doguv2.Dogu{}), "Normal", "IngressCreation", "Created regular ingress for service [%s].", doguName)

		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().GetRewriteAnnotationKey().Return("rewrite")
		ingressControllerMock.EXPECT().GetUseRegexKey().Return("regex")

		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(testCtx, doguName).Return(true, nil)

		ingressInterfaceMock := newMockIngressInterface(t)
		ingressInterfaceMock.EXPECT().Get(testCtx, expectedIngress.Name, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		ingressInterfaceMock.EXPECT().Create(testCtx, expectedIngress, metav1.CreateOptions{}).Return(nil, nil)

		sut := ingressUpdater{
			deploymentReadyChecker: deploymentReadyChecker,
			doguInterface:          doguInterfaceMock,
			controller:             ingressControllerMock,
			ingressInterface:       ingressInterfaceMock,
			namespace:              testNamespace,
			ingressClassName:       testIngressClassName,
			eventRecorder:          recorderMock,
		}

		// when
		err := sut.upsertIngressForCesService(testCtx, cesServiceWithOneWebapp, &service, true)

		// then
		require.NoError(t, err)
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
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace},
		}

		expectedIngress := getTestIngress(
			"test",
			"/myLocation",
			service,
			"nginx-static",
			80,
			map[string]string{
				"rewrite": "/errors/503.html",
			},
		)

		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, service.Name, metav1.GetOptions{}).Return(dogu, nil)
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().GetRewriteAnnotationKey().Return("rewrite")
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(&doguv2.Dogu{}), "Normal", "IngressCreation", "Ingress for service [%s] has been updated to maintenance mode.", "test")
		ingressInterfaceMock := newMockIngressInterface(t)
		ingressInterfaceMock.EXPECT().Get(testCtx, expectedIngress.Name, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		ingressInterfaceMock.EXPECT().Create(testCtx, expectedIngress, metav1.CreateOptions{}).Return(nil, nil)

		sut := ingressUpdater{
			doguInterface:    doguInterfaceMock,
			controller:       ingressControllerMock,
			ingressInterface: ingressInterfaceMock,
			namespace:        testNamespace,
			ingressClassName: testIngressClassName,
			eventRecorder:    recorderMock,
		}

		// when
		err := sut.upsertIngressForCesService(testCtx, cesServiceWithOneWebapp, &service, true)

		// then
		require.NoError(t, err)
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
				Namespace: testNamespace,
				Labels:    map[string]string{"dogu.name": "test"}},
		}

		expectedIngress := getTestIngress("test", "/myLocation", service, "nginx-static", 80, map[string]string{
			"rewrite": "/errors/starting.html",
		})

		dogu := &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}
		doguInterfaceMock := newMockDoguInterface(t)
		doguInterfaceMock.EXPECT().Get(testCtx, service.Name, metav1.GetOptions{}).Return(dogu, nil)
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().GetRewriteAnnotationKey().Return("rewrite")
		deploymentReadyChecker := NewMockDeploymentReadyChecker(t)
		deploymentReadyChecker.EXPECT().IsReady(testCtx, "test").Return(false, nil).Once()
		ingressInterfaceMock := newMockIngressInterface(t)
		ingressInterfaceMock.EXPECT().Get(testCtx, expectedIngress.Name, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		ingressInterfaceMock.EXPECT().Create(testCtx, expectedIngress, metav1.CreateOptions{}).Return(nil, nil)

		sut := ingressUpdater{
			deploymentReadyChecker: deploymentReadyChecker,
			doguInterface:          doguInterfaceMock,
			controller:             ingressControllerMock,
			ingressInterface:       ingressInterfaceMock,
			namespace:              testNamespace,
			ingressClassName:       testIngressClassName,
		}

		// when
		err := sut.upsertIngressForCesService(testCtx, cesServiceWithOneWebapp, &service, false)

		// then
		require.NoError(t, err)
	})
}
func TestCesService_getRewriteConfig(t *testing.T) {
	tests := []struct {
		name    string
		rewrite string
		want    *serviceRewrite
		wantErr func(t *testing.T, err error)
	}{
		{
			name:    "should fail to unmarshal invalid rewrite",
			rewrite: "{\"pattern\": \"portainer\", \"rewrite\": \"\"",
			want:    nil,
			wantErr: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "failed to read service rewrite from ces service: unexpected end of JSON input")
			},
		},
		{
			name:    "should succeed to generate rewrite config",
			rewrite: "{\"pattern\": \"portainer\", \"rewrite\": \"p\"}",
			want:    &serviceRewrite{Pattern: "portainer", Rewrite: "p"},
			wantErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// given
			cs := CesService{Rewrite: tt.rewrite}
			// when
			got, err := cs.getRewriteConfig()
			// then
			tt.wantErr(t, err)
			assert.Equalf(t, tt.want, got, "getRewriteConfig()")
		})
	}
}

func getTestIngress(ingressName string, path string, service corev1.Service, targetServiceName string, targetPort int32, annotations map[string]string) *v1.Ingress {
	pathType := v1.PathTypePrefix
	ingressClassName := testIngressClassName
	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   testNamespace,
			Annotations: annotations,
			Labels:      util.K8sCesServiceDiscoveryLabels,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: service.APIVersion,
				Kind:       service.Kind,
				Name:       service.Name,
				UID:        service.UID,
			}},
		},
		Spec: v1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []v1.IngressRule{
				{
					IngressRuleValue: v1.IngressRuleValue{
						HTTP: &v1.HTTPIngressRuleValue{
							Paths: []v1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathType,
									Backend: v1.IngressBackend{
										Service: &v1.IngressServiceBackend{
											Name: targetServiceName,
											Port: v1.ServiceBackendPort{
												Number: targetPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
