package expose

import (
	"context"
	"errors"
	"fmt"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/domain"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type NetworkPolicyHandler struct {
	generator networkPolicyGenerator
	client    client.Client
	namespace string
}

type NetworkPolicyHandlerDependencies struct {
	Namespace         string
	IngressController ingressController
	AllowedCIDR       string
	DynamicClient     dynamic.Interface
}

func NewNetworkPolicyHandler(
	disabled bool,
	namespace string,
	ingressController ingressController,
	allowedCidr string,
	client client.Client) *NetworkPolicyHandler {
	return &NetworkPolicyHandler{
		generator: newNetworkPolicyGenerator(disabled, namespace, ingressController, allowedCidr),
		client:    client,
		namespace: namespace,
	}
}

func (nph *NetworkPolicyHandler) UpsertNetworkPoliciesForService(ctx context.Context, service *corev1.Service) error {
	definition, err := domain.CreateExposedPortsDefinitionFromService(service)
	if err != nil {
		return fmt.Errorf("failed to create exposed ports definition from service: %w", err)
	}

	return nph.upsertForDefinition(ctx, definition)
}

func (nph *NetworkPolicyHandler) UpsertNetworkPoliciesForExposition(ctx context.Context, exposition *expositionv1.Exposition) error {
	definition := domain.CreateExposedPortsDefinitionFromExposition(exposition)
	return nph.upsertForDefinition(ctx, definition)
}

func (nph *NetworkPolicyHandler) upsertForDefinition(ctx context.Context, definition domain.ExposedPortsDefinition) error {
	desiredState := nph.generator.Generate(definition)

	var errs []error
	existing := &networkingv1.NetworkPolicyList{}
	err := nph.client.List(ctx, existing, &client.ListOptions{Namespace: nph.namespace, LabelSelector: selectorFromBaseName(definition.BaseName)})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to list existing ingresses: %w", err))
	}

	var existingMap map[string]networkingv1.NetworkPolicy
	for _, existingObject := range existing.Items {
		existingMap[existingObject.Name] = existingObject
	}

	for _, desiredObject := range desiredState {
		updateRef := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: desiredObject.Name, Namespace: desiredObject.Namespace}}
		// only keep track of those that are not in the desired state to delete later
		delete(existingMap, desiredObject.Name)

		_, err := controllerutil.CreateOrUpdate(ctx, nph.client, updateRef, func() error {
			updateRef.Annotations = desiredObject.Annotations
			updateRef.OwnerReferences = desiredObject.OwnerReferences
			updateRef.Labels = desiredObject.Labels
			updateRef.Spec = desiredObject.Spec
			return nil
		})
		if err != nil {
			errs = append(errs, err)
		}
	}

	// delete objects not in desired state
	for _, existingObject := range existingMap {
		err := nph.client.Delete(ctx, &existingObject)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to upsert network policies for %s %q: %w", definition.Type, definition.BaseName, errors.Join(errs...))
	}

	return nil
}
