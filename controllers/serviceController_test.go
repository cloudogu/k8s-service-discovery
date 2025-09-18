package controllers

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func Test_serviceReconciler_Reconcile(t *testing.T) {
	t.Run("missing service results in no cluster change but log INFO message for weird behavior", func(t *testing.T) {
		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)
		networkPolicyUpdaterMock := NewMockNetworkPolicyUpdater(t)

		// mock logger to catch log messages
		mockLogSink := NewMockLogSink(t)
		logger := logr.Logger{}
		logger = logger.WithSink(mockLogSink) // overwrite original logger with the given LogSink
		mockLogSink.EXPECT().WithValues().Return(mockLogSink)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Info(0, `service my-namespace/my-service not found`)
		mockLogSink.EXPECT().Info(0, `remove exposed ports`)
		mockLogSink.EXPECT().Info(0, `remove network policy ports`)
		// inject logger into context this way because the context search key is private to the logging framework
		valuedTestCtx := log.IntoContext(testCtx, logger)
		networkPolicyUpdaterMock.EXPECT().RemoveExposedPorts(valuedTestCtx, "my-service").Return(nil)

		sut := NewServiceReconciler(clientMock, ingressUpdaterMock, networkPolicyUpdaterMock, true)
		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "my-service"}}

		// when
		actualResult, err := sut.Reconcile(valuedTestCtx, request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, actualResult)
	})

	t.Run("failed to create ingress object of service", func(t *testing.T) {
		// given
		service := &corev1.Service{
			TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "my-service", Namespace: testNamespace},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(service).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)
		ingressUpdaterMock.EXPECT().UpsertIngressForService(testCtx, service).Return(assert.AnError)
		networkPolicyUpdaterMock := NewMockNetworkPolicyUpdater(t)

		sut := NewServiceReconciler(clientMock, ingressUpdaterMock, networkPolicyUpdaterMock, true)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "my-service"}}

		// when
		_, err := sut.Reconcile(testCtx, request)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create/update ingress object of service [my-service]: assert.AnError general error for testing")
	})

	t.Run("failed to update networkpolicy", func(t *testing.T) {
		// given
		service := &corev1.Service{
			TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "my-service", Namespace: testNamespace},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(service).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)
		ingressUpdaterMock.EXPECT().UpsertIngressForService(testCtx, service).Return(nil)
		networkPolicyUpdaterMock := NewMockNetworkPolicyUpdater(t)
		networkPolicyUpdaterMock.EXPECT().UpsertNetworkPoliciesForService(testCtx, service).Return(assert.AnError)

		sut := NewServiceReconciler(clientMock, ingressUpdaterMock, networkPolicyUpdaterMock, true)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "my-service"}}

		// when
		_, err := sut.Reconcile(testCtx, request)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create/update network policies for service [my-service]: assert.AnError general error for testing")
	})

	t.Run("should remove networkpolicy if disabled", func(t *testing.T) {
		// given
		service := &corev1.Service{
			TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "my-service", Namespace: testNamespace},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(service).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)
		ingressUpdaterMock.EXPECT().UpsertIngressForService(testCtx, service).Return(nil)
		networkPolicyUpdaterMock := NewMockNetworkPolicyUpdater(t)
		networkPolicyUpdaterMock.EXPECT().RemoveNetworkPolicy(testCtx).Return(nil)

		sut := NewServiceReconciler(clientMock, ingressUpdaterMock, networkPolicyUpdaterMock, false)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "my-service"}}

		// when
		_, err := sut.Reconcile(testCtx, request)

		// then
		require.NoError(t, err)
	})
}
