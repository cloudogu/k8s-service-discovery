package dogustart

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fake2 "k8s.io/client-go/kubernetes/fake"
	"testing"
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

	t.Run("Error when retrieving deployment with checker", func(t *testing.T) {
		// given
		client := fake2.NewSimpleClientset()
		checker := NewDeploymentReadyChecker(client, deploymentNamespace)

		// when
		_, err := checker.IsReady(ctx, deploymentName)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "deployments.apps \"myDeployment\" not found")
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
