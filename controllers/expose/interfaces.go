package expose

import (
	"context"

	"github.com/cloudogu/k8s-dogu-operator/v3/api/ecoSystem"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	netv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
	"k8s.io/client-go/tools/record"
)

// DeploymentReadyChecker checks the readiness from deployments.
type DeploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}

type eventRecorder interface {
	record.EventRecorder
}

type GlobalConfigRepository interface {
	Get(context.Context) (libconfig.GlobalConfig, error)
	Watch(context.Context, ...libconfig.WatchFilter) (<-chan repository.GlobalConfigWatchResult, error)
	Update(ctx context.Context, globalConfig libconfig.GlobalConfig) (libconfig.GlobalConfig, error)
}

// used for mocks

//nolint:unused
//goland:noinspection GoUnusedType
type deploymentInterface interface {
	appsv1.DeploymentInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type ingressClassInterface interface {
	netv1.IngressClassInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type ingressInterface interface {
	netv1.IngressInterface
}

type doguInterface interface {
	ecoSystem.DoguInterface
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
