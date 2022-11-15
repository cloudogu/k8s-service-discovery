package dogustart

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fake2 "k8s.io/client-go/kubernetes/fake"
	"testing"
	"time"
)

func TestNewDeploymentReadyChecker(t *testing.T) {
	t.Run("Create new deployment ready checker", func(t *testing.T) {
		// given
		client := fake2.NewSimpleClientset()

		// when
		checker := NewDeploymentReadyChecker(client, "myNamespace")

		// then
		assert.NotNil(t, checker)
		assert.Equal(t, client, checker.client)
		assert.Equal(t, "myNamespace", checker.namespace)
	})
}

func Test_deploymentPodChecker_IsReady(t *testing.T) {
	ctx := context.Background()
	deploymentNamespace := "myNamespace"
	deploymentName := "myDeployment"

	t.Run("false when no deployment is found in the cluster", func(t *testing.T) {
		// given
		client := fake2.NewSimpleClientset()
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)

		// when
		ok, err := checker.IsReady(ctx, deploymentName)

		// then
		require.NoError(t, err)
		assert.False(t, ok)
	})
	t.Run("Not ready for zero ready replicas", func(t *testing.T) {
		// given
		deployment := &v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: deploymentNamespace,
			},
		}
		client := fake2.NewSimpleClientset(deployment)
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)

		// when
		isReady, err := checker.IsReady(ctx, deploymentName)

		// then
		require.NoError(t, err)
		assert.False(t, isReady)
	})
	t.Run("Ready for at least one ready replica", func(t *testing.T) {
		// given
		deployment := &v1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: deploymentNamespace,
			},
			Status: v1.DeploymentStatus{
				Replicas:          1,
				ReadyReplicas:     1,
				AvailableReplicas: 1,
			},
		}
		client := fake2.NewSimpleClientset(deployment)
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)

		// when
		isReady, err := checker.IsReady(ctx, deploymentName)

		// then
		require.NoError(t, err)
		assert.True(t, isReady)
	})
}

func Test_deploymentReadyChecker_WaitForReady(t *testing.T) {
	ctx := context.Background()
	deploymentNamespace := "myNamespace"
	deploymentName := "myDeployment"
	deployment := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: deploymentNamespace,
		},
		Status: v1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}

	t.Run("Instantly terminate when deployment is already ready", func(t *testing.T) {
		// given
		deployment.Status.ReadyReplicas = 1
		client := fake2.NewSimpleClientset(deployment)
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)
		waitOptions := WaitOptions{Timeout: time.Second, TickRate: time.Second}

		// when
		onReadyCalled := false
		err := checker.WaitForReady(ctx, deploymentName, waitOptions, func(ctx context.Context) {
			onReadyCalled = true
		})

		// then
		require.NoError(t, err)
		assert.True(t, onReadyCalled)
	})
	t.Run("Terminate after timeout while waiting for pod readiness", func(t *testing.T) {
		// given
		deployment.Status.ReadyReplicas = 0
		client := fake2.NewSimpleClientset(deployment)
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)
		waitOptions := WaitOptions{Timeout: time.Millisecond, TickRate: time.Millisecond}

		// when
		onReadyCalled := false
		err := checker.WaitForReady(ctx, deploymentName, waitOptions, func(ctx context.Context) {
			onReadyCalled = true
		})

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to wait for deployment readiness: timeout after [1ms] while waiting of pod being ready")
		assert.False(t, onReadyCalled)
	})
	t.Run("Terminate after closing context while waiting for pod readiness", func(t *testing.T) {
		// given
		deployment.Status.ReadyReplicas = 0
		client := fake2.NewSimpleClientset(deployment)
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)
		waitOptions := WaitOptions{Timeout: time.Hour, TickRate: time.Second}

		// when
		cancelCtx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancelFunc()

		onReadyCalled := false
		err := checker.WaitForReady(cancelCtx, deploymentName, waitOptions, func(ctx context.Context) {
			onReadyCalled = true
		})

		// then
		require.NoError(t, err)
		assert.False(t, onReadyCalled)
	})
	t.Run("successfully react when the deployment is ready after a certain amount of time", func(t *testing.T) {
		// given
		deployment.Status.ReadyReplicas = 0
		client := fake2.NewSimpleClientset(deployment)
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)
		waitOptions := WaitOptions{Timeout: time.Hour, TickRate: time.Millisecond * 10}

		// change the deployment after 250 ms to trigger our wait function
		go func() {
			timer := time.NewTimer(250 * time.Millisecond)

			<-timer.C

			deployment.Status.ReadyReplicas = 1
			_, err := checker.client.AppsV1().Deployments(deploymentNamespace).Update(ctx, deployment, metav1.UpdateOptions{})
			require.NoError(t, err)
		}()

		// when
		onReadyCalled := false
		err := checker.WaitForReady(ctx, deploymentName, waitOptions, func(ctx context.Context) {
			onReadyCalled = true
		})

		// then
		require.NoError(t, err)
		assert.True(t, onReadyCalled)
	})
}
