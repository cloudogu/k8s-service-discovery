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
	Client               client.Client
	GatewayLabelSelector metav1.LabelSelector
	ExposedAllowedCIDR   string
}

func (n NetworkPolicy) GetOwnableTypes() []client.Object {
	return []client.Object{&networkingv1.NetworkPolicy{}}
}

func (n NetworkPolicy) ProcessExposition(ctx context.Context, exposition types.Exposition) error {
	err, done := n.processExposedPortNetworkPolicy(ctx, exposition)
	if done {
		return err
	}

	err, done = n.processHttpGatewayNetworkPolicy(ctx, exposition)
	if done {
		return err
	}

	return exposition.SetCondition(ctx, NetworkPolicyConditionType, true,
		networkPolicyCreatedConditionReason, networkPolicyCreatedConditionMessage)
}

func (n NetworkPolicy) processExposedPortNetworkPolicy(ctx context.Context, exposition types.Exposition) (error, bool) {
	name := n.createNameForExposedPorts(exposition.Name)

	if len(exposition.TcpRoutes)+len(exposition.UdpRoutes) == 0 {
		return n.deleteIfExists(ctx, exposition, name), true
	}

	desired, err := n.generateForExposedPorts(exposition)
	if err != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPolicyConditionType, networkPolicyGenerationFailedConditionReason,
			fmt.Errorf("failed to generate network policy %q: %w", name, err)), true
	}

	err = n.apply(ctx, exposition, name, desired)
	if err != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPolicyConditionType, networkPolicyCreateOrUpdateFailedConditionReason,
			fmt.Errorf("failed to create or update network policy %q: %w", name, err)), true
	}

	return nil, false
}

func (n NetworkPolicy) processHttpGatewayNetworkPolicy(ctx context.Context, exposition types.Exposition) (error, bool) {
	name := n.createNameForHttpGateway(exposition.Name)

	if len(exposition.HttpRoutes) == 0 {
		return n.deleteIfExists(ctx, exposition, name), true
	}

	desired, err := n.generateForHttpGateway(exposition)
	if err != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPolicyConditionType, networkPolicyGenerationFailedConditionReason,
			fmt.Errorf("failed to generate network policy %q: %w", name, err)), true
	}

	err = n.apply(ctx, exposition, name, desired)
	if err != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPolicyConditionType, networkPolicyCreateOrUpdateFailedConditionReason,
			fmt.Errorf("failed to create or update network policy %q: %w", name, err)), true
	}

	return nil, false
}

func (n NetworkPolicy) apply(ctx context.Context, exposition types.Exposition, name string, desired *networkingv1.NetworkPolicy) error {
	target := &networkingv1.NetworkPolicy{Name: name, Namespace: exposition.Namespace}
	_, cuErr := controllerutil.CreateOrUpdate(ctx, n.Client, target, func() error {
		target.Labels = desired.Labels
		target.Annotations = desired.Annotations
		target.OwnerReferences = desired.OwnerReferences
		target.Spec = desired.Spec

		return nil
	})

	return cuErr
}

func (n NetworkPolicy) deleteIfExists(ctx context.Context, exposition types.Exposition, name string) error {
	stub := &networkingv1.NetworkPolicy{Name: name, Namespace: exposition.Namespace}

	if err := n.Client.Delete(ctx, stub); err != nil && !apierrors.IsNotFound(err) {
		return handleErrorCondition(ctx, exposition,
			NetworkPolicyConditionType, networkPolicyDeletionFailedConditionReason,
			fmt.Errorf("failed to delete network policy %q: %w", name, err))
	}

	return nil
}

func (n NetworkPolicy) generateForExposedPorts(exposition types.Exposition) (*networkingv1.NetworkPolicy, error) {
	totalPortSize := len(exposition.TcpRoutes) + len(exposition.UdpRoutes)
	exposedPorts := make([]types.ExposedPort, 0, totalPortSize)

	exposedPorts = append(exposedPorts, exposition.TcpRoutes...)
	exposedPorts = append(exposedPorts, exposition.UdpRoutes...)

	netpol := &networkingv1.NetworkPolicy{
		Name:      n.createNameForExposedPorts(exposition.Name),
		Namespace: exposition.Namespace,
		Labels:    util.K8sCesServiceDiscoveryLabels,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: n.GatewayLabelSelector,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: n.mapExposedPorts(exposedPorts),
					From: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: n.ExposedAllowedCIDR,
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

func (n NetworkPolicy) createNameForExposedPorts(expositionName string) string {
	return fmt.Sprintf("%s-exposed-ports", expositionName)
}

func (n NetworkPolicy) createNameForHttpGateway(expositionName string) string {
	return fmt.Sprintf("%s-http-gateway", expositionName)
}

func (n NetworkPolicy) generateForHttpGateway(exposition types.Exposition) (*networkingv1.NetworkPolicy, error) {
	panic("not implemented")
}
