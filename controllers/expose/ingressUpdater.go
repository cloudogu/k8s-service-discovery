package expose

import (
	"context"
	"errors"
	"fmt"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/domain"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

type IngressUpdater struct {
	ingressDefinitionCreator ingressDefinitionCreator
	generator                ingressGenerator
	ingressUpserter          upserter
	middlewareUpserter       upserter
}

type IngressUpdaterDependencies struct {
	Namespace          string
	IngressClassName   string
	MaintenanceAdapter maintenanceAdapter
	ReadyChecker       DeploymentReadyChecker
	DynamicClient      dynamic.Interface
}

// NewIngressUpdater creates a new instance responsible for updating ingress objects.
func NewIngressUpdater(deps IngressUpdaterDependencies) *IngressUpdater {
	return &IngressUpdater{
		ingressDefinitionCreator: domain.NewIngressDefinitionCreator(deps.MaintenanceAdapter, deps.ReadyChecker),
		generator:                newIngressGenerator(deps.Namespace, deps.IngressClassName),
		ingressUpserter: util.NewDeclarativeUpserter(
			deps.DynamicClient,
			networkingv1.SchemeGroupVersion.WithResource("ingresses"),
			deps.Namespace,
		),
		middlewareUpserter: util.NewDeclarativeUpserter(
			deps.DynamicClient,
			traefikv1alpha1.SchemeGroupVersion.WithResource("middlewares"),
			deps.Namespace,
		),
	}
}

// UpsertForService creates or updates the ingress object of the given service.
func (i *IngressUpdater) UpsertForService(ctx context.Context, service *corev1.Service) error {
	expositionDefinition, err := i.ingressDefinitionCreator.CreateFromService(ctx, service)
	if err != nil {
		return fmt.Errorf("failed to convert service to exposition exposition definition: %w", err)
	}

	return i.upsertForDefinition(ctx, expositionDefinition)
}

func (i *IngressUpdater) UpsertForExposition(ctx context.Context, exposition *expositionv1.Exposition) error {
	expositionDefinition, err := i.ingressDefinitionCreator.CreateFromExposition(ctx, exposition)
	if err != nil {
		return fmt.Errorf("failed to convert exposition to exposition exposition definition: %w", err)
	}

	return i.upsertForDefinition(ctx, expositionDefinition)
}

func (i *IngressUpdater) upsertForDefinition(ctx context.Context, definition domain.IngressDefinition) error {
	// generate ingress objects
	ingresses, middlewares := i.generator.GenerateWithMiddlewares(definition)

	// upsert ingresses
	unstructuredIngresses, err := convertListToUnstructuredMap(ingresses)
	if err != nil {
		return fmt.Errorf("failed to convert ingresses to unstructured: %w", err)
	}

	selector := labels.Set{ownedByLabelKey: definition.BaseName}.AsSelector()
	err = i.ingressUpserter.Upsert(ctx, selector, unstructuredIngresses)
	if err != nil {
		return fmt.Errorf("failed to upsert ingresses: %w", err)
	}

	// upsert middlewares
	unstructuredMiddlewares, err := convertListToUnstructuredMap(middlewares)
	if err != nil {
		return fmt.Errorf("failed to convert middlewares to unstructured: %w", err)
	}

	err = i.middlewareUpserter.Upsert(ctx, selector, unstructuredMiddlewares)
	if err != nil {
		return fmt.Errorf("failed to upsert middlewares: %w", err)
	}

	return nil
}

func convertListToUnstructuredMap[T metav1.Object](objects []T) (map[string]unstructured.Unstructured, error) {
	var converterErrs []error
	unstructuredObjects := make(map[string]unstructured.Unstructured, len(objects))
	for _, object := range objects {
		unstructuredIngress, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
		if err != nil {
			converterErrs = append(converterErrs, err)
		}

		unstructuredObjects[object.GetName()] = unstructured.Unstructured{Object: unstructuredIngress}
	}

	return unstructuredObjects, errors.Join(converterErrs...)
}
