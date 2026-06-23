package adapter

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	NetworkPolicyConditionType = "NetworkPolicyReady"

	networkPolicyCreatedConditionReason  = "Created"
	networkPolicyCreatedConditionMessage = "Network policy has been created."

	networkPolicyDeletionFailedConditionReason       = "DeletionFailed"
	networkPolicyGenerationFailedConditionReason     = "GenerationFailed"
	networkPolicyCreateOrUpdateFailedConditionReason = "CreateOrUpdateFailed"
)

type NetworkPolicy struct {
	Client        client.Client
	LabelSelector metav1.LabelSelector
	AllowedCIDR   string
}

func (n NetworkPolicy) GetOwnableTypes() []client.Object {
	return []client.Object{&networkingv1.NetworkPolicy{}}
}

func (n NetworkPolicy) ProcessExposition(ctx context.Context, exposition types.Exposition) error {
	name := n.createNetworkPolicyName(exposition.Name)

	if len(exposition.TcpRoutes)+len(exposition.UdpRoutes) == 0 {
		stub := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: exposition.Namespace}}

		if err := n.Client.Delete(ctx, stub); err != nil && !apierrors.IsNotFound(err) {
			return handleErrorCondition(ctx, exposition,
				NetworkPolicyConditionType, networkPolicyDeletionFailedConditionReason,
				fmt.Errorf("failed to delete network policy %q: %w", name, err))
		}

		return nil
	}

	desired, err := n.generateNetworkPolicy(exposition)
	if err != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPolicyConditionType, networkPolicyGenerationFailedConditionReason,
			fmt.Errorf("failed to generate network policy: %w", err))
	}

	target := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: exposition.Namespace}}
	if _, cuErr := controllerutil.CreateOrUpdate(ctx, n.Client, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		target.OwnerReferences = desired.OwnerReferences
		target.Spec = desired.Spec

		return nil
	}); cuErr != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPolicyConditionType, networkPolicyCreateOrUpdateFailedConditionReason,
			fmt.Errorf("failed to create or update network policy %q: %w", name, cuErr))
	}

	return exposition.SetCondition(ctx, NetworkPolicyConditionType, true,
		networkPolicyCreatedConditionReason, networkPolicyCreatedConditionMessage)
}

func (n NetworkPolicy) generateNetworkPolicy(exposition types.Exposition) (*networkingv1.NetworkPolicy, error) {
	totalPortSize := len(exposition.TcpRoutes) + len(exposition.UdpRoutes)
	exposedPorts := make([]types.ExposedPort, 0, totalPortSize)

	exposedPorts = append(exposedPorts, exposition.TcpRoutes...)
	exposedPorts = append(exposedPorts, exposition.UdpRoutes...)

	netpol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      n.createNetworkPolicyName(exposition.Name),
			Namespace: exposition.Namespace,
			Labels:    util.K8sCesServiceDiscoveryLabels,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: n.LabelSelector,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: n.mapExposedPorts(exposedPorts),
					From: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: n.AllowedCIDR,
							},
						},
					},
				},
			},
		},
	}

	if oErr := exposition.SetOwner(netpol); oErr != nil {
		return nil, fmt.Errorf("failed to set owner for network policy: %w", oErr)
	}

	return netpol, nil
}

func (n NetworkPolicy) mapExposedPorts(exposedPorts []types.ExposedPort) []networkingv1.NetworkPolicyPort {
	networkPolicyPorts := make([]networkingv1.NetworkPolicyPort, 0, len(exposedPorts))

	for _, e := range exposedPorts {
		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Protocol: new(e.Protocol),
			Port:     new(intstr.FromInt32(e.RequestedExternalPort)),
		})
	}

	return networkPolicyPorts
}

func (n NetworkPolicy) createNetworkPolicyName(expositionName string) string {
	return fmt.Sprintf("%s-exposed-ports", expositionName)
}
