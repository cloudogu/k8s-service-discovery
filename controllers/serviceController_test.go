package controllers

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func Test_serviceReconciler_Reconcile(t *testing.T) {
	t.Run("missing service results in no cluster change", func(t *testing.T) {
		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)

		sut := NewServiceReconciler(clientMock, ingressUpdaterMock)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "my-namespace", Name: "my-service"}}

		// when
		actualResult, err := sut.Reconcile(testCtx, request)

		// then
		assert.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, actualResult)
	})

	t.Run("failed to create ingress object of service", func(t *testing.T) {
		// given
		service := &corev1.Service{
			TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
			ObjectMeta: metav1.ObjectMeta{Name: "my-service", Namespace: "my-namespace"},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(service).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)
		ingressUpdaterMock.EXPECT().UpsertIngressForService(testCtx, service).Return(assert.AnError)

		sut := NewServiceReconciler(clientMock, ingressUpdaterMock)

		request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "my-namespace", Name: "my-service"}}

		// when
		_, err := sut.Reconcile(testCtx, request)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create/update ingress object of service [my-service]: assert.AnError general error for testing")
	})
}
