package expose

import (
	"context"
	"github.com/cloudogu/k8s-dogu-operator/v3/api/ecoSystem"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
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

type serviceInterface interface {
	corev1.ServiceInterface
}

type ingressController interface {
	GetName() string
	GetControllerSpec() string
	GetRewriteAnnotationKey() string
	GetUseRegexKey() string
	tcpUpdServiceExposer
}

// tcpUpdServiceExposer is used to expose non http services.
type tcpUpdServiceExposer interface {
	// ExposeOrUpdateExposedPorts adds or updates the exposing of the exposed ports from the service in the cluster. These are typically
	// entries in a configmap.
	ExposeOrUpdateExposedPorts(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts) error
	// DeleteExposedPorts removes the exposing of the exposed ports from the service in the cluster. These are typically
	// entries in a configmap.
	DeleteExposedPorts(ctx context.Context, namespace string, targetServiceName string) error
}

type networkPolicyInterface interface {
	netv1.NetworkPolicyInterface
}
