package expose

import (
	"context"
	"fmt"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/domain"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/dynamic"
)

type NetworkPolicyHandler struct {
	generator networkPolicyGenerator
	upserter  upserter
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
	dynamicClient dynamic.Interface) *NetworkPolicyHandler {
	return &NetworkPolicyHandler{
		generator: newNetworkPolicyGenerator(disabled, namespace, ingressController, allowedCidr),
		upserter: util.NewDeclarativeUpserter(
			dynamicClient,
			networkingv1.SchemeGroupVersion.WithResource("networkpolicies"),
			namespace,
		),
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
	networkPolicies := nph.generator.Generate(definition)
	unstructuredNetworkPolicies, err := convertListToUnstructuredMap(networkPolicies)
	if err != nil {
		return fmt.Errorf("failed to convert network policies to unstructured: %w", err)
	}

	selector := labels.Set{ownedByLabelKey: definition.BaseName}.AsSelector()
	err = nph.upserter.Upsert(ctx, selector, unstructuredNetworkPolicies)
	if err != nil {
		return fmt.Errorf("failed to upsert network policies: %w", err)
	}

	return nil
}
