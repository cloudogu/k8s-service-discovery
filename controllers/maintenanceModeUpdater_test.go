package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	etcdclient "go.etcd.io/etcd/client/v2"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	cesmocks "github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/cloudogu/k8s-service-discovery/controllers/mocks"
)

var testCtx = context.Background()

func TestNewMaintenanceModeUpdater(t *testing.T) {
	t.Run("failed to create registry", func(t *testing.T) {
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, err := NewMaintenanceModeUpdater(clientMock, "%!%*Ä'%'!%'", &mocks.IngressUpdater{}, mocks.NewEventRecorder(t))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create etcd client")
		require.Nil(t, creator)
	})

	t.Run("successfully create updater", func(t *testing.T) {
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, err := NewMaintenanceModeUpdater(clientMock, "test", &mocks.IngressUpdater{}, mocks.NewEventRecorder(t))

		require.NoError(t, err)
		require.NotNil(t, creator)
	})
}

func Test_maintenanceModeUpdater_Start(t *testing.T) {
	t.Run("error on maintenance update", func(t *testing.T) {
		// given
		regMock := cesmocks.NewRegistry(t)
		watchContextMock := cesmocks.NewWatchConfigurationContext(t)
		watchContextMock.On("Watch", mock.Anything, "/config/_global/maintenance", true, mock.Anything).Run(func(args mock.Arguments) {
			channelobject := args.Get(3)
			sendChannel, ok := channelobject.(chan *etcdclient.Response)

			if ok {
				testResponse := &etcdclient.Response{}
				sendChannel <- testResponse
			}
		}).Return()

		globalConfigMock := cesmocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "maintenance").Return("", assert.AnError)
		regMock.On("RootConfig").Return(watchContextMock, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := &mocks.IngressUpdater{}

		namespace := "myTestNamespace"
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects().Build()
		svcRewriter := &NoopServiceRewriter{}

		maintenanceUpdater := &maintenanceModeUpdater{
			client:          clientMock,
			namespace:       namespace,
			registry:        regMock,
			ingressUpdater:  ingressUpdater,
			serviceRewriter: svcRewriter,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*100)

		// when
		err := maintenanceUpdater.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("fail to get deployment", func(t *testing.T) {
		// given
		regMock := cesmocks.NewRegistry(t)
		watchContextMock := cesmocks.NewWatchConfigurationContext(t)
		watchContextMock.On("Watch", mock.Anything, "/config/_global/maintenance", true, mock.Anything).Run(func(args mock.Arguments) {
			channelobject := args.Get(3)
			sendChannel, ok := channelobject.(chan *etcdclient.Response)

			if ok {
				testResponse := &etcdclient.Response{}
				sendChannel <- testResponse
			}
		}).Return()

		globalConfigMock := cesmocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "maintenance").Return("false", nil)
		regMock.On("RootConfig").Return(watchContextMock, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := &mocks.IngressUpdater{}

		namespace := "myTestNamespace"
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects().Build()
		svcRewriter := &NoopServiceRewriter{}

		maintenanceUpdater := &maintenanceModeUpdater{
			client:          clientMock,
			namespace:       namespace,
			registry:        regMock,
			ingressUpdater:  ingressUpdater,
			serviceRewriter: svcRewriter,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*100)

		// when
		err := maintenanceUpdater.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "maintenance mode: failed to get deployment [k8s-service-discovery-controller-manager]")
	})

	t.Run("run and terminate without any problems", func(t *testing.T) {
		// given
		regMock := cesmocks.NewRegistry(t)
		watchContextMock := cesmocks.NewWatchConfigurationContext(t)
		watchContextMock.On("Watch", mock.Anything, "/config/_global/maintenance", true, mock.Anything).Run(func(args mock.Arguments) {
			channelobject := args.Get(3)
			sendChannel, ok := channelobject.(chan *etcdclient.Response)

			if ok {
				testResponse := &etcdclient.Response{}
				sendChannel <- testResponse
			}
		}).Return()

		globalConfigMock := cesmocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "maintenance").Return("false", nil)
		regMock.On("RootConfig").Return(watchContextMock, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := &mocks.IngressUpdater{}

		namespace := "myTestNamespace"
		deployment := &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()

		eventRecorderMock := mocks.NewEventRecorder(t)
		eventRecorderMock.On("Eventf", mock.IsType(deployment), "Normal", "Maintenance", "Maintenance mode changed to %t.", true)
		svcRewriter := &NoopServiceRewriter{}

		maintenanceUpdater := &maintenanceModeUpdater{
			client:          clientMock,
			namespace:       namespace,
			registry:        regMock,
			ingressUpdater:  ingressUpdater,
			eventRecorder:   eventRecorderMock,
			serviceRewriter: svcRewriter,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*100)

		// when
		err := maintenanceUpdater.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
	})
}

func Test_maintenanceModeUpdater_handleMaintenanceModeUpdate(t *testing.T) {
	t.Run("activate maintenance mode with error", func(t *testing.T) {
		// given
		regMock := cesmocks.NewRegistry(t)
		globalConfigMock := cesmocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "maintenance").Return("true", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := mocks.NewIngressUpdater(t)
		ingressUpdater.On("UpsertIngressForService", mock.Anything, mock.Anything).Return(assert.AnError)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		maintenanceUpdater := &maintenanceModeUpdater{
			client:         clientMock,
			namespace:      namespace,
			registry:       regMock,
			ingressUpdater: ingressUpdater,
		}

		// when
		err := maintenanceUpdater.handleMaintenanceModeUpdate(context.Background())

		// then
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("deactivate maintenance mode with error", func(t *testing.T) {
		// given
		regMock := cesmocks.NewRegistry(t)
		globalConfigMock := cesmocks.NewConfigurationContext(t)

		keyNotFoundErr := etcdclient.Error{Code: etcdclient.ErrorCodeKeyNotFound}
		globalConfigMock.On("Get", "maintenance").Return("", keyNotFoundErr)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := mocks.NewIngressUpdater(t)
		ingressUpdater.On("UpsertIngressForService", mock.Anything, mock.Anything).Return(assert.AnError)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		maintenanceUpdater := &maintenanceModeUpdater{
			client:         clientMock,
			namespace:      namespace,
			registry:       regMock,
			ingressUpdater: ingressUpdater,
		}

		// when
		err := maintenanceUpdater.handleMaintenanceModeUpdate(context.Background())

		// then
		require.ErrorIs(t, err, assert.AnError)
	})
}

func Test_isServiceNginxRelated(t *testing.T) {
	t.Run("should return true for nginx-prefixed dogu service", func(t *testing.T) {
		svc := &corev1.Service{Spec: corev1.ServiceSpec{
			Selector: map[string]string{"dogu.name": "nginx-static"},
		}}

		assert.True(t, isServiceNginxRelated(svc))
	})
	t.Run("should return false for other dogu services", func(t *testing.T) {
		svc := &corev1.Service{Spec: corev1.ServiceSpec{
			Selector: map[string]string{"dogu.name": "totally-not-an-nginx-static-service"},
		}}

		assert.False(t, isServiceNginxRelated(svc))
	})
}

func Test_rewriteNonSimpleServiceRoute(t *testing.T) {
	testNS := "test-namespace"

	t.Run("should rewrite selector of dogu service to a non-existing target for maintenance mode activation", func(t *testing.T) {
		// given
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nexus",
				Labels:    map[string]string{"dogu.name": "nexus"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "nexus"}},
		}
		mockRecorder := mocks.NewEventRecorder(t)
		mockRecorder.On("Eventf", svc, corev1.EventTypeNormal, "Maintenance", "Maintenance mode was activated, rewriting exposed service %s", "nexus")
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(svc).Build()

		// when
		err := rewriteNonSimpleServiceRoute(testCtx, clientMock, mockRecorder, svc, true)

		// then
		require.NoError(t, err)
		actualSvc := corev1.Service{}
		err = clientMock.Get(testCtx, types.NamespacedName{
			Namespace: testNS,
			Name:      "nexus",
		}, &actualSvc)
		require.NoError(t, err)

		expectedSvc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nexus",
				Labels:    map[string]string{"dogu.name": "nexus"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "deactivatedDuringMaintenance"}},
		}
		// ignore version which the client introduces
		expectedSvc.ResourceVersion = "1000"
		actualSvc.ResourceVersion = "1000"
		assert.Equal(t, expectedSvc.Spec, actualSvc.Spec)
		assert.Equal(t, expectedSvc.ObjectMeta, actualSvc.ObjectMeta)
	})

	t.Run("should rewrite selector of dogu service to a non-existing target for maintenance mode activation", func(t *testing.T) {
		// given
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nexus",
				Labels:    map[string]string{"dogu.name": "nexus"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "deactivatedDuringMaintenance"}},
		}
		mockRecorder := mocks.NewEventRecorder(t)
		mockRecorder.On("Eventf", svc, corev1.EventTypeNormal, "Maintenance", "Maintenance mode was deactivated, restoring exposed service %s", "nexus")
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(svc).Build()

		// when
		err := rewriteNonSimpleServiceRoute(testCtx, clientMock, mockRecorder, svc, false)

		// then
		require.NoError(t, err)
		actualSvc := corev1.Service{}
		err = clientMock.Get(testCtx, types.NamespacedName{
			Namespace: testNS,
			Name:      "nexus",
		}, &actualSvc)
		require.NoError(t, err)

		expectedSvc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nexus",
				Labels:    map[string]string{"dogu.name": "nexus"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "nexus"}},
		}
		// ignore version which the client introduces
		expectedSvc.ResourceVersion = "1000"
		actualSvc.ResourceVersion = "1000"
		assert.Equal(t, expectedSvc.Spec, actualSvc.Spec)
		assert.Equal(t, expectedSvc.ObjectMeta, actualSvc.ObjectMeta)
	})

	t.Run("should error when API request fails", func(t *testing.T) {
		// given
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nexus",
				Labels:    map[string]string{"dogu.name": "nexus"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "deactivatedDuringMaintenance"}},
		}
		mockRecorder := mocks.NewEventRecorder(t)
		mockRecorder.On("Eventf", svc, corev1.EventTypeNormal, "Maintenance", "Maintenance mode was deactivated, restoring exposed service %s", "nexus")
		clientMock := mocks.NewClient(t)
		clientMock.On("Update", testCtx, svc).Return(assert.AnError)

		// when
		err := rewriteNonSimpleServiceRoute(testCtx, clientMock, mockRecorder, svc, false)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "could not rewrite service nexus")
	})
	t.Run("should exit early on ClusterIP service", func(t *testing.T) {
		// given
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nexus",
				Labels:    map[string]string{"dogu.name": "nexus"},
			},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		}
		mockRecorder := mocks.NewEventRecorder(t)
		clientMock := mocks.NewClient(t)

		// when
		err := rewriteNonSimpleServiceRoute(testCtx, clientMock, mockRecorder, svc, false)

		// then
		require.NoError(t, err)
	})
	t.Run("should exit early on non-dogu service", func(t *testing.T) {
		// given
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nexus",
				Labels:    map[string]string{"a-fun-label": "goes-here"},
			},
		}
		mockRecorder := mocks.NewEventRecorder(t)
		clientMock := mocks.NewClient(t)

		// when
		err := rewriteNonSimpleServiceRoute(testCtx, clientMock, mockRecorder, svc, false)

		// then
		require.NoError(t, err)
	})
	t.Run("should exit early on nginx services so we don't lock us self out", func(t *testing.T) {
		// given
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNS,
				Name:      "nginx-ingress",
				Labels:    map[string]string{"dogu.name": "nginx-ingress"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "nginx-ingress"}},
		}
		mockRecorder := mocks.NewEventRecorder(t)
		clientMock := mocks.NewClient(t)

		// when
		err := rewriteNonSimpleServiceRoute(testCtx, clientMock, mockRecorder, svc, false)

		// then
		require.NoError(t, err)
	})
}

func Test_defaultServiceRewriter_rewrite(t *testing.T) {
	t.Run("should error during maintenance deactivation", func(t *testing.T) {
		// given
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "nexus",
				Labels: map[string]string{"dogu.name": "nexus"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "deactivatedDuringMaintenance"}},
		}
		internalSvcList := []*corev1.Service{&svc}
		mockRecorder := mocks.NewEventRecorder(t)
		mockRecorder.On("Eventf", mock.AnythingOfType("*v1.Service"), corev1.EventTypeNormal, "Maintenance", "Maintenance mode was deactivated, restoring exposed service %s", "nexus")
		clientMock := mocks.NewClient(t)
		clientMock.On("Update", testCtx, &svc).Return(assert.AnError)

		sut := &defaultServiceRewriter{
			client:        clientMock,
			namespace:     "el-espacio-del-nombre",
			eventRecorder: mockRecorder,
		}

		// when
		err := sut.rewrite(testCtx, internalSvcList, false)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "could not rewrite service nexus")
	})
	t.Run("should error during maintenance activation", func(t *testing.T) {
		// given
		svc := corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "nexus",
				Labels: map[string]string{"dogu.name": "deactivatedDuringMaintenance"},
			},
			Spec: corev1.ServiceSpec{Selector: map[string]string{"dogu.name": "nexus"}},
		}
		internalSvcList := []*corev1.Service{&svc}
		mockRecorder := mocks.NewEventRecorder(t)
		mockRecorder.On("Eventf", mock.AnythingOfType("*v1.Service"), corev1.EventTypeNormal, "Maintenance", "Maintenance mode was activated, rewriting exposed service %s", "nexus")
		clientMock := mocks.NewClient(t)
		clientMock.On("Update", testCtx, &svc).Return(assert.AnError)

		sut := &defaultServiceRewriter{
			client:        clientMock,
			namespace:     "el-espacio-del-nombre",
			eventRecorder: mockRecorder,
		}

		// when
		err := sut.rewrite(testCtx, internalSvcList, true)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "could not rewrite service nexus")
	})
}

func Test_maintenanceModeUpdater_getAllServices(t *testing.T) {
	t.Run("should return error from the API", func(t *testing.T) {
		// given
		clientMock := mocks.NewClient(t)
		clientMock.On("List", testCtx, mock.Anything, mock.Anything).Return(assert.AnError)

		sut := &maintenanceModeUpdater{
			client:    clientMock,
			namespace: "el-espacio-del-nombre",
		}

		// when
		_, err := sut.getAllServices(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get list of all services in namespace [el-espacio-del-nombre]:")
	})
}

func Test_maintenanceModeUpdater_deactivateMaintenanceMode(t *testing.T) {
	t.Run("should error on listing services", func(t *testing.T) {
		// given
		noopRec := &NoopEventRecorder{}
		clientMock := mocks.NewClient(t)
		clientMock.On("List", testCtx, mock.Anything, mock.Anything).Return(assert.AnError)
		updateMock := mocks.NewIngressUpdater(t)
		svcRewriter := NewServiceRewriter(t)

		sut := &maintenanceModeUpdater{
			client:          clientMock,
			namespace:       "el-espacio-del-nombre",
			eventRecorder:   noopRec,
			ingressUpdater:  updateMock,
			serviceRewriter: svcRewriter,
		}

		// when
		err := sut.deactivateMaintenanceMode(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get list of all services")
	})
	t.Run("should error on rewriting service", func(t *testing.T) {
		// given
		noopRec := &NoopEventRecorder{}
		clientMock := mocks.NewClient(t)
		clientMock.On("List", testCtx, mock.Anything, mock.Anything).Return(nil)
		updateMock := mocks.NewIngressUpdater(t)
		svcRewriter := NewServiceRewriter(t)
		svcRewriter.On("rewrite", testCtx, mock.Anything, mock.Anything).Return(assert.AnError)

		sut := &maintenanceModeUpdater{
			client:          clientMock,
			namespace:       "el-espacio-del-nombre",
			eventRecorder:   noopRec,
			ingressUpdater:  updateMock,
			serviceRewriter: svcRewriter,
		}

		// when
		err := sut.deactivateMaintenanceMode(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to rewrite services during maintenance mode deactivation")
	})
}

func Test_maintenanceModeUpdater_activateMaintenanceMode(t *testing.T) {
	t.Run("should error on listing services", func(t *testing.T) {
		// given
		noopRec := &NoopEventRecorder{}
		clientMock := mocks.NewClient(t)
		clientMock.On("List", testCtx, mock.Anything, mock.Anything).Return(assert.AnError)
		updateMock := mocks.NewIngressUpdater(t)
		svcRewriter := NewServiceRewriter(t)

		sut := &maintenanceModeUpdater{
			client:          clientMock,
			namespace:       "el-espacio-del-nombre",
			eventRecorder:   noopRec,
			ingressUpdater:  updateMock,
			serviceRewriter: svcRewriter,
		}

		// when
		err := sut.activateMaintenanceMode(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get list of all services")
	})
	t.Run("should error on rewriting service", func(t *testing.T) {
		// given
		noopRec := &NoopEventRecorder{}
		clientMock := mocks.NewClient(t)
		clientMock.On("List", testCtx, mock.Anything, mock.Anything).Return(nil)
		updateMock := mocks.NewIngressUpdater(t)
		svcRewriter := NewServiceRewriter(t)
		svcRewriter.On("rewrite", testCtx, mock.Anything, mock.Anything).Return(assert.AnError)

		sut := &maintenanceModeUpdater{
			client:          clientMock,
			namespace:       "el-espacio-del-nombre",
			eventRecorder:   noopRec,
			ingressUpdater:  updateMock,
			serviceRewriter: svcRewriter,
		}

		// when
		err := sut.activateMaintenanceMode(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to rewrite services during maintenance mode activation")
	})
}

type NoopEventRecorder struct{}

func (n *NoopEventRecorder) Event(_ runtime.Object, _, _, _ string)                    {}
func (n *NoopEventRecorder) Eventf(_ runtime.Object, _, _, _ string, _ ...interface{}) {}
func (n *NoopEventRecorder) AnnotatedEventf(_ runtime.Object, _ map[string]string, _, _, _ string, _ ...interface{}) {
}

type NoopServiceRewriter struct{}

func (n *NoopServiceRewriter) rewrite(_ context.Context, _ v1ServiceList, _ bool) error { return nil }
