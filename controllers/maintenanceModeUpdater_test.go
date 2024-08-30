package controllers

import (
	"context"
	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"testing"
	"time"
)

var testCtx = context.Background()

func TestNewMaintenanceModeUpdater(t *testing.T) {
	t.Run("successfully create updater", func(t *testing.T) {
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		creator, err := NewMaintenanceModeUpdater(clientMock, "test", NewMockIngressUpdater(t), newMockEventRecorder(t), globalConfigRepoMock)

		require.NoError(t, err)
		require.NotNil(t, creator)
	})
}

func Test_maintenanceModeUpdater_handleMaintenanceModeUpdate(t *testing.T) {
	t.Run("activate maintenance mode with error", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"maintenance": "{\"title\": \"titel\", \"text\":\"text\"}",
		})
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		ingressUpdater := NewMockIngressUpdater(t)
		ingressUpdater.EXPECT().UpsertIngressForService(mock.Anything, mock.Anything).Return(assert.AnError)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		maintenanceUpdater := &maintenanceModeUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
			ingressUpdater:   ingressUpdater,
		}

		// when
		err := maintenanceUpdater.handleMaintenanceModeUpdate(context.Background())

		// then
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("deactivate maintenance mode with error", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{})
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		ingressUpdater := NewMockIngressUpdater(t)
		ingressUpdater.EXPECT().UpsertIngressForService(mock.Anything, mock.Anything).Return(assert.AnError)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		maintenanceUpdater := &maintenanceModeUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
			ingressUpdater:   ingressUpdater,
		}

		// when
		err := maintenanceUpdater.handleMaintenanceModeUpdate(context.Background())

		// then
		require.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should return error on get maintenance mode error", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{})
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, assert.AnError)

		maintenanceUpdater := &maintenanceModeUpdater{
			globalConfigRepo: globalConfigRepoMock,
		}

		// when
		err := maintenanceUpdater.handleMaintenanceModeUpdate(context.Background())

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get global config for maintenance mode")
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
		mockRecorder := newMockEventRecorder(t)
		mockRecorder.EXPECT().Eventf(svc, corev1.EventTypeNormal, "Maintenance", "Maintenance mode was activated, rewriting exposed service %s", "nexus")
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
		mockRecorder := newMockEventRecorder(t)
		mockRecorder.EXPECT().Eventf(svc, corev1.EventTypeNormal, "Maintenance", "Maintenance mode was deactivated, restoring exposed service %s", "nexus")
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
		mockRecorder := newMockEventRecorder(t)
		mockRecorder.EXPECT().Eventf(svc, corev1.EventTypeNormal, "Maintenance", "Maintenance mode was deactivated, restoring exposed service %s", "nexus")
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().Update(testCtx, svc).Return(assert.AnError)

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
		mockRecorder := newMockEventRecorder(t)
		clientMock := newMockK8sClient(t)

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
		mockRecorder := newMockEventRecorder(t)
		clientMock := newMockK8sClient(t)

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
		mockRecorder := newMockEventRecorder(t)
		clientMock := newMockK8sClient(t)

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
		mockRecorder := newMockEventRecorder(t)
		mockRecorder.EXPECT().Eventf(mock.AnythingOfType("*v1.Service"), corev1.EventTypeNormal, "Maintenance", "Maintenance mode was deactivated, restoring exposed service %s", "nexus")
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().Update(testCtx, &svc).Return(assert.AnError)

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
		mockRecorder := newMockEventRecorder(t)
		mockRecorder.EXPECT().Eventf(mock.AnythingOfType("*v1.Service"), corev1.EventTypeNormal, "Maintenance", "Maintenance mode was activated, rewriting exposed service %s", "nexus")
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().Update(testCtx, &svc).Return(assert.AnError)

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
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().List(testCtx, mock.Anything, mock.Anything).Return(assert.AnError)

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

	t.Run("should return multiple services", func(t *testing.T) {
		// given
		clientMock := newMockK8sClient(t)
		serviceA := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "A and not equal B"}}
		serviceB := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "B and not equal A"}}
		services := []corev1.Service{serviceA, serviceB}
		serviceList := &corev1.ServiceList{Items: services}
		clientMock.EXPECT().List(testCtx, mock.Anything, mock.Anything).Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
			c := list.(*corev1.ServiceList)
			c.Items = serviceList.Items
		}).Return(nil)

		sut := &maintenanceModeUpdater{
			client:    clientMock,
			namespace: "el-espacio-del-nombre",
		}

		// when
		result, err := sut.getAllServices(testCtx)

		// then
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.NotEqual(t, result[0].Name, result[1].Name)
	})
}
func Test_maintenanceModeUpdater_deactivateMaintenanceMode(t *testing.T) {
	t.Run("should error on listing services", func(t *testing.T) {
		// given
		noopRec := newMockEventRecorder(t)
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().List(testCtx, mock.Anything, mock.Anything).Return(assert.AnError)
		updateMock := NewMockIngressUpdater(t)
		svcRewriter := newMockServiceRewriter(t)

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
		noopRec := newMockEventRecorder(t)
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().List(testCtx, mock.Anything, mock.Anything).Return(nil)
		updateMock := NewMockIngressUpdater(t)
		svcRewriter := newMockServiceRewriter(t)
		svcRewriter.EXPECT().rewrite(testCtx, mock.Anything, mock.Anything).Return(assert.AnError)

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
		noopRec := newMockEventRecorder(t)
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().List(testCtx, mock.Anything, mock.Anything).Return(assert.AnError)
		updateMock := NewMockIngressUpdater(t)
		svcRewriter := newMockServiceRewriter(t)

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
		noopRec := newMockEventRecorder(t)
		clientMock := newMockK8sClient(t)
		clientMock.EXPECT().List(testCtx, mock.Anything, mock.Anything).Return(nil)
		updateMock := NewMockIngressUpdater(t)
		svcRewriter := newMockServiceRewriter(t)
		svcRewriter.EXPECT().rewrite(testCtx, mock.Anything, mock.Anything).Return(assert.AnError)

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

func Test_maintenanceModeUpdater_Start(t *testing.T) {
	t.Run("success with inactive maintenance mode", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		globalConfigMock := NewMockGlobalConfigRepository(t)
		channel := make(chan repository.GlobalConfigWatchResult)
		globalConfigMock.EXPECT().Watch(cancelCtx, mock.Anything).Return(channel, nil)
		globalConfigMock.EXPECT().Get(cancelCtx).Return(config.GlobalConfig{}, nil)

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().List(cancelCtx, mock.Anything, mock.Anything).Return(nil).Times(1)
		k8sClientMock.EXPECT().List(cancelCtx, mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, list client.ObjectList, option ...client.ListOption) error {
			list.(*corev1.PodList).Items = []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "nginx-static", Namespace: testNamespace}}}
			return nil
		}).Times(1)
		k8sClientMock.EXPECT().Delete(cancelCtx, mock.Anything).Return(nil).Times(1)
		k8sClientMock.EXPECT().Get(cancelCtx, types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(nil)

		eventRecorderMock := newMockEventRecorder(t)
		eventRecorderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeNormal, "Maintenance", "Maintenance mode changed to %t.", false).Run(func(object runtime.Object, eventtype string, reason string, messageFmt string, args ...interface{}) {
			cancelFunc()
		})

		serviceRewriterMock := newMockServiceRewriter(t)
		serviceRewriterMock.EXPECT().rewrite(cancelCtx, mock.Anything, mock.Anything).Return(nil)

		sut := maintenanceModeUpdater{
			namespace:        testNamespace,
			globalConfigRepo: globalConfigMock,
			client:           k8sClientMock,
			eventRecorder:    eventRecorderMock,
			serviceRewriter:  serviceRewriterMock,
		}

		// when
		err := sut.Start(cancelCtx)
		channel <- repository.GlobalConfigWatchResult{}

		// then
		require.NoError(t, err)
		<-cancelCtx.Done()
	})

	t.Run("success with active maintenance mode", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		globalConfigMock := NewMockGlobalConfigRepository(t)
		channel := make(chan repository.GlobalConfigWatchResult)
		globalConfigMock.EXPECT().Watch(cancelCtx, mock.Anything).Return(channel, nil)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"maintenance": "{\"title\": \"titel\", \"text\":\"text\"}",
		})
		globalConfigMock.EXPECT().Get(cancelCtx).Return(globalConfig, nil)

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().List(cancelCtx, mock.Anything, mock.Anything).Return(nil).Times(1)
		k8sClientMock.EXPECT().List(cancelCtx, mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, list client.ObjectList, option ...client.ListOption) error {
			list.(*corev1.PodList).Items = []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "nginx-static", Namespace: testNamespace}}}
			return nil
		}).Times(1)
		k8sClientMock.EXPECT().Delete(cancelCtx, mock.Anything).Return(nil).Times(1)
		k8sClientMock.EXPECT().Get(cancelCtx, types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(nil)

		eventRecorderMock := newMockEventRecorder(t)
		eventRecorderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeNormal, "Maintenance", "Maintenance mode changed to %t.", true).Run(func(object runtime.Object, eventtype string, reason string, messageFmt string, args ...interface{}) {
			cancelFunc()
		})

		serviceRewriterMock := newMockServiceRewriter(t)
		serviceRewriterMock.EXPECT().rewrite(cancelCtx, mock.Anything, mock.Anything).Return(nil)

		sut := maintenanceModeUpdater{
			namespace:        testNamespace,
			globalConfigRepo: globalConfigMock,
			client:           k8sClientMock,
			eventRecorder:    eventRecorderMock,
			serviceRewriter:  serviceRewriterMock,
		}

		// when
		err := sut.Start(cancelCtx)
		channel <- repository.GlobalConfigWatchResult{}

		// then
		require.NoError(t, err)
		<-cancelCtx.Done()
	})

	t.Run("should return error on get watch error", func(t *testing.T) {
		// given
		globalConfigMock := NewMockGlobalConfigRepository(t)
		globalConfigMock.EXPECT().Watch(testCtx, mock.Anything).Return(nil, assert.AnError)

		sut := maintenanceModeUpdater{
			namespace:        testNamespace,
			globalConfigRepo: globalConfigMock,
		}

		// when
		err := sut.Start(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to start maintenance watch")
	})
}

func Test_maintenanceModeUpdater_startMaintenanceWatch(t *testing.T) {
	t.Run("should return and log if the channel will be closed", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		controllerruntime.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			controllerruntime.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Info(0, "maintenance watch channel canceled - stop watch").Run(func(level int, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		sut := maintenanceModeUpdater{}
		channel := make(chan repository.GlobalConfigWatchResult)

		// when
		go func() {
			sut.startMaintenanceWatch(cancelCtx, channel)
		}()
		close(channel)
		<-cancelCtx.Done()
	})

	t.Run("should continue and log error on watch result error", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		controllerruntime.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			controllerruntime.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Info(0, "context done - stop global config watcher for maintenance")
		mockLogSink.EXPECT().Error(assert.AnError, "maintenance watch channel error").Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		sut := maintenanceModeUpdater{}
		channel := make(chan repository.GlobalConfigWatchResult)

		// when
		go func() {
			sut.startMaintenanceWatch(cancelCtx, channel)
		}()
		channel <- repository.GlobalConfigWatchResult{Err: assert.AnError}
		<-cancelCtx.Done()
		// Wait for last log
		timer := time.NewTimer(time.Millisecond * 500)
		<-timer.C
	})

	t.Run("should return error on error executing global config update on watch event", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfigRepoMock.EXPECT().Get(cancelCtx).Return(config.GlobalConfig{}, assert.AnError)

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		controllerruntime.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			controllerruntime.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		// One update log gets send initially without updating anything
		mockLogSink.EXPECT().Info(0, "Maintenance mode key changed in registry. Refresh ingress objects accordingly...")
		mockLogSink.EXPECT().Info(0, "context done - stop global config watcher for maintenance")
		mockLogSink.EXPECT().Error(mock.Anything, "failed to handle maintenance update").Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		sut := maintenanceModeUpdater{
			namespace:        testNamespace,
			globalConfigRepo: globalConfigRepoMock,
		}
		channel := make(chan repository.GlobalConfigWatchResult)

		// when
		go func() {
			sut.startMaintenanceWatch(cancelCtx, channel)
		}()
		channel <- repository.GlobalConfigWatchResult{}
		<-cancelCtx.Done()
		// Wait for last log
		timer := time.NewTimer(time.Millisecond * 500)
		<-timer.C
	})
}
