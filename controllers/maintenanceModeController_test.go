package controllers

import (
	"context"
	"testing"

	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var testCtx = context.Background()

func TestNewMaintenanceModeUpdater(t *testing.T) {
	t.Run("successfully create updater", func(t *testing.T) {
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator := NewMaintenanceModeController(clientMock, "test", NewMockIngressUpdater(t), NewMockMaintenanceAdapter(t), newMockEventRecorder(t))

		require.NotEmpty(t, creator)
	})
}

func Test_maintenanceModeUpdater_Reconcile(t *testing.T) {
	t.Run("fail to get maintenance mode config", func(t *testing.T) {
		// given
		maintenanceAdapterMock := NewMockMaintenanceAdapter(t)
		maintenanceAdapterMock.EXPECT().GetStatus(testCtx).Return(repository.MaintenanceModeDescription{}, false, assert.AnError)

		maintenanceUpdater := &maintenanceModeController{
			maintenanceAdapter: maintenanceAdapterMock,
		}

		// when
		_, err := maintenanceUpdater.Reconcile(context.Background(), reconcile.Request{})

		// then
		require.ErrorIs(t, err, assert.AnError)
	})
	t.Run("fail to list services", func(t *testing.T) {
		// given
		maintenanceAdapterMock := NewMockMaintenanceAdapter(t)
		maintenanceAdapterMock.EXPECT().GetStatus(testCtx).Return(repository.MaintenanceModeDescription{}, true, nil)

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().List(testCtx, &corev1.ServiceList{}, &client.ListOptions{Namespace: testNamespace}).Return(assert.AnError)

		maintenanceUpdater := &maintenanceModeController{
			namespace:          testNamespace,
			client:             k8sClientMock,
			maintenanceAdapter: maintenanceAdapterMock,
		}

		// when
		_, err := maintenanceUpdater.Reconcile(context.Background(), reconcile.Request{})

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to activate maintenance mode")
		assert.ErrorContains(t, err, "failed to get list of all services in namespace [my-namespace]")
	})
	t.Run("fail to upsert ingress", func(t *testing.T) {
		// given
		maintenanceAdapterMock := NewMockMaintenanceAdapter(t)
		maintenanceAdapterMock.EXPECT().GetStatus(testCtx).Return(repository.MaintenanceModeDescription{}, false, nil)

		ingressUpdater := NewMockIngressUpdater(t)
		ingressUpdater.EXPECT().UpsertIngressForService(mock.Anything, mock.Anything).Return(assert.AnError)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		maintenanceUpdater := &maintenanceModeController{
			client:             clientMock,
			namespace:          namespace,
			ingressUpdater:     ingressUpdater,
			maintenanceAdapter: maintenanceAdapterMock,
		}

		// when
		_, err := maintenanceUpdater.Reconcile(context.Background(), reconcile.Request{})

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to deactivate maintenance mode")
	})
	t.Run("fail to rewrite service", func(t *testing.T) {
		// given
		maintenanceAdapterMock := NewMockMaintenanceAdapter(t)
		maintenanceAdapterMock.EXPECT().GetStatus(testCtx).Return(repository.MaintenanceModeDescription{}, false, nil)

		ingressUpdater := NewMockIngressUpdater(t)
		ingressUpdater.EXPECT().UpsertIngressForService(mock.Anything, mock.Anything).Return(nil)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace, ResourceVersion: "999"}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		maintenanceConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "maintenance", Namespace: namespace}, Data: map[string]string{"active": "false"}}
		configMapList := &corev1.ConfigMapList{Items: []corev1.ConfigMap{*maintenanceConfigMap}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList, configMapList).Build()

		rewriterMock := newMockServiceRewriter(t)
		rewriterMock.EXPECT().rewrite(testCtx, v1ServiceList{testService}, false).Return(assert.AnError)

		maintenanceUpdater := &maintenanceModeController{
			client:             clientMock,
			namespace:          namespace,
			ingressUpdater:     ingressUpdater,
			serviceRewriter:    rewriterMock,
			maintenanceAdapter: maintenanceAdapterMock,
		}

		// when
		_, err := maintenanceUpdater.Reconcile(context.Background(), reconcile.Request{})

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to rewrite services on deactivate maintenance mode")
	})
	t.Run("success", func(t *testing.T) {
		// given
		maintenanceAdapterMock := NewMockMaintenanceAdapter(t)
		maintenanceAdapterMock.EXPECT().GetStatus(testCtx).Return(repository.MaintenanceModeDescription{}, true, nil)

		ingressUpdater := NewMockIngressUpdater(t)
		ingressUpdater.EXPECT().UpsertIngressForService(mock.Anything, mock.Anything).Return(nil)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace, ResourceVersion: "999"}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		rewriterMock := newMockServiceRewriter(t)
		rewriterMock.EXPECT().rewrite(testCtx, v1ServiceList{testService}, true).Return(nil)

		maintenanceUpdater := &maintenanceModeController{
			client:             clientMock,
			namespace:          namespace,
			ingressUpdater:     ingressUpdater,
			serviceRewriter:    rewriterMock,
			maintenanceAdapter: maintenanceAdapterMock,
		}

		// when
		_, err := maintenanceUpdater.Reconcile(context.Background(), reconcile.Request{})

		// then
		require.NoError(t, err)
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

func Test_maintenanceModeController_SetupWithManager(t *testing.T) {
	sut := &maintenanceModeController{}
	managerMock := newMockK8sManager(t)
	managerMock.EXPECT().GetControllerOptions().Return(config.Controller{})
	managerMock.EXPECT().GetScheme().Return(getScheme())
	managerMock.EXPECT().GetLogger().Return(logr.New(nil))
	managerMock.EXPECT().Add(mock.Anything).Return(nil)
	managerMock.EXPECT().GetCache().Return(nil)

	err := sut.SetupWithManager(managerMock)
	assert.NoError(t, err)
}

func Test_maintenancePredicate(t *testing.T) {
	sut := maintenancePredicate()
	assert.False(t, sut.Generic(event.GenericEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "nginx-config"}}}))
	assert.True(t, sut.Delete(event.DeleteEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "maintenance"}}}))
}
