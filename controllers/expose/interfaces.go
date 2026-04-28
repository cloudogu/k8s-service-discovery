package expose

import (
	"context"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/definition"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	netv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
)

type maintenanceAdapter interface {
	GetStatus(ctx context.Context) (repository.MaintenanceModeDescription, bool, error)
}

// DeploymentReadyChecker checks the readiness from deployments.
type DeploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}

type serviceConverter interface {
	Convert(ctx context.Context, service *corev1.Service) (definition.ExpositionDefinition, error)
}

type expositionConverter interface {
	Convert(ctx context.Context, exposition *expositionv1.Exposition) (definition.ExpositionDefinition, error)
}

type ingressGenerator interface {
	GenerateWithMiddlewares(definition definition.ExpositionDefinition) ([]*networkingv1.Ingress, []*traefikapi.Middleware)
}

type upserter interface {
	Upsert(ctx context.Context, labelSelector labels.Selector, objects map[string]unstructured.Unstructured) error
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

type ingressController interface {
	GetName() string
	GetRewriteAnnotationKey() string
}

type networkPolicyInterface interface {
	netv1.NetworkPolicyInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type middlewareInterface interface {
	traefikv1alpha1.MiddlewareInterface
}

type traefikInterface interface {
	traefikv1alpha1.TraefikV1alpha1Interface
}
