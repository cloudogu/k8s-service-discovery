package controllers

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestNewDeploymentReconciler(t *testing.T) {
	t.Run("successfully create deployment reconciler", func(t *testing.T) {
		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)

		// when
		reconciler := NewDeploymentReconciler(clientMock, ingressUpdaterMock)

		// then
		assert.NotNil(t, reconciler)
		assert.NotNil(t, reconciler.client)
		assert.NotNil(t, reconciler.updater)
	})
}

func TestDeploymentReconciler_getDeployment(t *testing.T) {
	name := "my-app"
	namespace := "myNamespace"
	t.Run("successfully get deployment", func(t *testing.T) {
		// given
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)

		reconciler := NewDeploymentReconciler(clientMock, ingressUpdaterMock)

		// when
		result, err := reconciler.getDeployment(testCtx,
			ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
		// then
		require.NoError(t, err)
		assert.Equal(t, name, result.Name)
		assert.Equal(t, namespace, result.Namespace)
	})

	t.Run("failed to get deployment if no deployment has been created", func(t *testing.T) {
		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		ingressUpdaterMock := NewMockIngressUpdater(t)

		reconciler := NewDeploymentReconciler(clientMock, ingressUpdaterMock)

		// when
		result, err := reconciler.getDeployment(testCtx,
			ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: name}})
		// then
		require.ErrorContains(t, err, "failed to get deployment: deployments.apps \"my-app\" not found")
		require.Nil(t, result)
	})
}
