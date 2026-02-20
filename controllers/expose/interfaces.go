package expose

import (
	"context"

	"github.com/cloudogu/k8s-dogu-operator/v3/api/ecoSystem"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
type ingressInterface interface {
	netv1.IngressInterface
}

type doguInterface interface {
	ecoSystem.DoguInterface
}

type ingressController interface {
	GetName() string
	GetRewriteAnnotationKey() string
}

type networkPolicyInterface interface {
	netv1.NetworkPolicyInterface
}

type middlewareManager interface {
	createOrUpdateReplacePathMiddleware(ctx context.Context, serviceName string, cesService CesService, ownerReferences []v1.OwnerReference) (string, error)
	CreateOrUpdateAlternativeFQDNRedirectMiddleware(ctx context.Context, alternativeFQDNs []string, primaryFQDN string, ownerReferences []v1.OwnerReference) (string, error)
}

//nolint:unused
//goland:noinspection GoUnusedType
type middlewareInterface interface {
	traefikv1alpha1.MiddlewareInterface
}
