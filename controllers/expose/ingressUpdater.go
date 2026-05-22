package expose

import (
	"context"
	"errors"
	"fmt"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/domain"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type IngressUpdater struct {
	ingressDefinitionCreator ingressDefinitionCreator
	generator                ingressGenerator
	client                   k8sClient
	namespace                string
}

type IngressUpdaterDependencies struct {
	Namespace          string
	IngressClassName   string
	MaintenanceAdapter maintenanceAdapter
	ReadyChecker       DeploymentReadyChecker
	Client             client.Client
}

// NewIngressUpdater creates a new instance responsible for updating ingress objects.
func NewIngressUpdater(deps IngressUpdaterDependencies) *IngressUpdater {
	return &IngressUpdater{
		ingressDefinitionCreator: &domain.IngressDefinitionCreator{
			Maintenance:  deps.MaintenanceAdapter,
			ReadyChecker: deps.ReadyChecker,
		},
		generator: newIngressGenerator(deps.Namespace, deps.IngressClassName),
		client:    deps.Client,
		namespace: deps.Namespace,
	}
}

// UpsertForService creates or updates the ingress object of the given service.
func (i *IngressUpdater) UpsertForService(ctx context.Context, service *corev1.Service) error {
	ingressDefinition, err := i.ingressDefinitionCreator.CreateFromService(ctx, service)
	if err != nil {
		return fmt.Errorf("failed to convert service to exposition definition: %w", err)
	}

	return i.upsertForDefinition(ctx, ingressDefinition)
}

func (i *IngressUpdater) UpsertForExposition(ctx context.Context, exposition *expositionv1.Exposition) error {
	ingressDefinition, err := i.ingressDefinitionCreator.CreateFromExposition(ctx, exposition)
	if err != nil {
		return fmt.Errorf("failed to convert exposition to exposition definition: %w", err)
	}

	return i.upsertForDefinition(ctx, ingressDefinition)
}

func (i *IngressUpdater) upsertForDefinition(ctx context.Context, definition domain.IngressDefinition) error {
	// generate ingress objects
	ingresses, middlewares := i.generator.GenerateWithMiddlewares(definition)

	var errs []error
	err := i.upsertIngresses(ctx, definition, ingresses)
	if err != nil {
		errs = append(errs, err)
	}

	err = i.upsertMiddlewares(ctx, definition, middlewares)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to upsert ingresses or middlewares for %s %q: %w", definition.Type, definition.BaseName, errors.Join(errs...))
	}

	return nil
}

func (i *IngressUpdater) upsertIngresses(ctx context.Context, definition domain.IngressDefinition, desiredState []*networkingv1.Ingress) error {
	var errs []error
	existing := &networkingv1.IngressList{}
	err := i.client.List(ctx, existing, &client.ListOptions{Namespace: i.namespace, LabelSelector: selectorFromBaseName(definition.BaseName)})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to list existing ingresses: %w", err))
	}

	existingMap := make(map[string]networkingv1.Ingress, len(existing.Items))
	for _, existingObject := range existing.Items {
		existingMap[existingObject.Name] = existingObject
	}

	for _, desiredObject := range desiredState {
		updateRef := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: desiredObject.Name, Namespace: desiredObject.Namespace}}
		// only keep track of those that are not in the desired state to delete later
		delete(existingMap, desiredObject.Name)

		_, err := controllerutil.CreateOrUpdate(ctx, i.client, updateRef, func() error {
			updateRef.Annotations = desiredObject.Annotations
			updateRef.OwnerReferences = desiredObject.OwnerReferences
			updateRef.Labels = desiredObject.Labels
			updateRef.Spec = desiredObject.Spec
			return nil
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create or update ingress %q: %w", desiredObject.Name, err))
		}
	}

	// delete objects not in desired state
	for _, existingObject := range existingMap {
		err := i.client.Delete(ctx, &existingObject)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to delete outdated ingress %q: %w", existingObject.Name, err))
		}
	}

	return errors.Join(errs...)
}

func (i *IngressUpdater) upsertMiddlewares(ctx context.Context, definition domain.IngressDefinition, desiredState []*traefikv1alpha1.Middleware) error {
	var errs []error
	existing := &traefikv1alpha1.MiddlewareList{}
	err := i.client.List(ctx, existing, &client.ListOptions{Namespace: i.namespace, LabelSelector: selectorFromBaseName(definition.BaseName)})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to list existing middlewares: %w", err))
	}

	existingMap := make(map[string]traefikv1alpha1.Middleware, len(existing.Items))
	for _, existingObject := range existing.Items {
		existingMap[existingObject.Name] = existingObject
	}

	for _, desiredObject := range desiredState {
		updateRef := &traefikv1alpha1.Middleware{ObjectMeta: metav1.ObjectMeta{Name: desiredObject.Name, Namespace: desiredObject.Namespace}}
		// only keep track of those that are not in the desired state to delete later
		delete(existingMap, desiredObject.Name)

		_, err := controllerutil.CreateOrUpdate(ctx, i.client, updateRef, func() error {
			updateRef.Annotations = desiredObject.Annotations
			updateRef.OwnerReferences = desiredObject.OwnerReferences
			updateRef.Labels = desiredObject.Labels
			updateRef.Spec = desiredObject.Spec
			return nil
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create or update middleware %q: %w", desiredObject.Name, err))
		}
	}

	// delete objects not in desired state
	for _, existingObject := range existingMap {
		err := i.client.Delete(ctx, &existingObject)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to delete outdated middleware %q: %w", existingObject.Name, err))
		}
	}

	return errors.Join(errs...)
}

func selectorFromBaseName(baseName string) labels.Selector {
	return labels.Set{ownedByLabelKey: baseName}.AsSelector()
}
