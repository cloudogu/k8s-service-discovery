package expose

import (
	"context"

	doguClient "github.com/cloudogu/k8s-dogu-lib/v2/client"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	netv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/client-go/tools/record"
)

type maintenanceAdapter interface {
	IsActive(ctx context.Context) (bool, error)
}

// DeploymentReadyChecker checks the readiness from deployments.
type DeploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}

type eventRecorder interface {
	record.EventRecorder
}

// used for mocks

//nolint:unused
//goland:noinspection GoUnusedType
type deploymentInterface interface {
	appsv1.DeploymentInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type ingressInterface interface {
	netv1.IngressInterface
}

type doguInterface interface {
	doguClient.DoguInterface
}

type ingressController interface {
	GetName() string
	GetControllerSpec() string
	GetRewriteAnnotationKey() string
	GetUseRegexKey() string
}

type networkPolicyInterface interface {
	netv1.NetworkPolicyInterface
}
