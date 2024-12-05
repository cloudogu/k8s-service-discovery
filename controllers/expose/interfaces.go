package expose

import (
	"context"
	"github.com/cloudogu/k8s-dogu-operator/v2/api/ecoSystem"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"k8s.io/client-go/kubernetes"
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

type ingressController interface {
	GetControllerSpec() string
	GetRewriteAnnotationKey() string
	GetAdditionalConfigurationKey() string
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

//nolint:unused
//goland:noinspection GoUnusedType
type clientSetInterface interface {
	kubernetes.Interface
}

//nolint:unused
//goland:noinspection GoUnusedType
type appsv1Interface interface {
	appsv1.AppsV1Interface
}

//nolint:unused
//goland:noinspection GoUnusedType
type netInterface interface {
	netv1.NetworkingV1Interface
}

type doguInterface interface {
	ecoSystem.DoguInterface
}
