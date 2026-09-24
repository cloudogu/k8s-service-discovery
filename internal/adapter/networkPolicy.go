package adapter

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	NetworkPoliciesConditionType = "NetworkPoliciesReady"

	networkPoliciesCreatedConditionReason  = "Created"
	networkPoliciesCreatedConditionMessage = "Network policies have been created."

	networkPoliciesDeletionFailedConditionReason       = "DeletionFailed"
	networkPoliciesGenerationFailedConditionReason     = "GenerationFailed"
	networkPoliciesCreateOrUpdateFailedConditionReason = "CreateOrUpdateFailed"
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
	err, done := n.processExternalPortsNetworkPolicy(ctx, exposition)
	if done {
		return err
	}

	err = n.processInternalRoutesNetworkPolicies(ctx, exposition)
	if err != nil {
		return err
	}

	return exposition.SetCondition(ctx, NetworkPoliciesConditionType, true,
		networkPoliciesCreatedConditionReason, networkPoliciesCreatedConditionMessage)
}

func (n NetworkPolicy) processExternalPortsNetworkPolicy(ctx context.Context, exposition types.Exposition) (error, bool) {
	name := n.createNameForExternalPorts(exposition.Name)

	if len(exposition.TcpRoutes)+len(exposition.UdpRoutes) == 0 {
		return n.deleteByNameIfExists(ctx, exposition, name), true
	}

	desired, err := n.generateForExternalPorts(exposition)
	if err != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPoliciesConditionType, networkPoliciesGenerationFailedConditionReason,
			fmt.Errorf("failed to generate network policy %q: %w", name, err)), true
	}

	err = n.upsertSingle(ctx, exposition, name, desired)
	if err != nil {
		return handleErrorCondition(ctx, exposition,
			NetworkPoliciesConditionType, networkPoliciesCreateOrUpdateFailedConditionReason,
			fmt.Errorf("failed to create or update network policy %q: %w", name, err)), true
	}

	return nil, false
}

func (n NetworkPolicy) processInternalRoutesNetworkPolicies(ctx context.Context, exposition types.Exposition) error {
	var errs []error
	desiredForHttpRoutes, err := n.generateForHttpRoutes(ctx, exposition)
	if err != nil {
		errs = append(errs, err)
	}

	desiredForTcpPorts, err := n.generateForExposedPorts(ctx, corev1.ProtocolTCP, exposition, exposition.TcpRoutes)
	if err != nil {
		errs = append(errs, err)
	}

	desiredForUdpPorts, err := n.generateForExposedPorts(ctx, corev1.ProtocolUDP, exposition, exposition.UdpRoutes)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return handleErrorCondition(ctx, exposition,
			NetworkPoliciesConditionType, networkPoliciesGenerationFailedConditionReason,
			fmt.Errorf("failed to generate network policies for internal routes: %w", errors.Join(errs...)))
	}

	desired := make([]*networkingv1.NetworkPolicy, 0, len(desiredForHttpRoutes)+len(desiredForTcpPorts)+len(desiredForUdpPorts))
	desired = append(desired, desiredForHttpRoutes...)
	desired = append(desired, desiredForTcpPorts...)
	desired = append(desired, desiredForUdpPorts...)

	err = n.upsertMultiple(ctx, exposition, desired)
	if err != nil {
		return err
	}

	return nil
}

func (n NetworkPolicy) upsertSingle(ctx context.Context, exposition types.Exposition, name string, desired *networkingv1.NetworkPolicy) error {
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

func (n NetworkPolicy) deleteByNameIfExists(ctx context.Context, exposition types.Exposition, name string) error {
	stub := &networkingv1.NetworkPolicy{Name: name, Namespace: exposition.Namespace}

	if err := n.Client.Delete(ctx, stub); err != nil && !apierrors.IsNotFound(err) {
		return handleErrorCondition(ctx, exposition,
			NetworkPoliciesConditionType, networkPoliciesDeletionFailedConditionReason,
			fmt.Errorf("failed to delete network policy %q: %w", name, err))
	}

	return nil
}

func (n NetworkPolicy) generateForExternalPorts(exposition types.Exposition) (*networkingv1.NetworkPolicy, error) {
	totalPortSize := len(exposition.TcpRoutes) + len(exposition.UdpRoutes)
	exposedPorts := make([]types.ExposedPort, 0, totalPortSize)

	exposedPorts = append(exposedPorts, exposition.TcpRoutes...)
	exposedPorts = append(exposedPorts, exposition.UdpRoutes...)

	netpol := &networkingv1.NetworkPolicy{
		Name:      n.createNameForExternalPorts(exposition.Name),
		Namespace: exposition.Namespace,
		Labels:    util.K8sCesServiceDiscoveryLabels,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: n.GatewayLabelSelector,
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: n.mapExternalPorts(exposedPorts),
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

func (n NetworkPolicy) mapExternalPorts(exposedPorts []types.ExposedPort) []networkingv1.NetworkPolicyPort {
	networkPolicyPorts := make([]networkingv1.NetworkPolicyPort, 0, len(exposedPorts))

	for _, e := range exposedPorts {
		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Protocol: new(e.Protocol),
			Port:     new(intstr.FromInt32(e.RequestedExternalPort)),
		})
	}

	return networkPolicyPorts
}

func (n NetworkPolicy) createNameForExternalPorts(expositionName string) string {
	return fmt.Sprintf("%s-exposed-ports", expositionName)
}

func (n NetworkPolicy) generateForHttpRoutes(ctx context.Context, exposition types.Exposition) ([]*networkingv1.NetworkPolicy, error) {
	var errs []error
	netpols := make([]*networkingv1.NetworkPolicy, 0, len(exposition.HttpRoutes))
	for _, route := range exposition.HttpRoutes {
		netpol, err := n.generateForHttpRoute(ctx, exposition, route)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = exposition.SetOwner(netpol)
		if err != nil {
			errs = append(errs, err)
		}

		netpols = append(netpols, netpol)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("generate network policies for http routes: %w", errors.Join(errs...))
	}

	return netpols, nil
}

func (n NetworkPolicy) generateForHttpRoute(ctx context.Context, exposition types.Exposition, route types.HttpRoute) (*networkingv1.NetworkPolicy, error) {
	service, err := n.getService(ctx, route.Service, exposition.Namespace)
	if err != nil {
		return nil, fmt.Errorf("generate for http route %q: %w", route.Name, err)
	}

	selectionLabels := map[string]string{ownedByLabelKey: exposition.Name}
	maps.Insert(selectionLabels, maps.All(util.K8sCesServiceDiscoveryLabels))

	return &networkingv1.NetworkPolicy{
		Name:      fmt.Sprintf("%s-%s-http-route", exposition.Name, route.Name),
		Namespace: exposition.Namespace,
		Labels:    selectionLabels,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: service.Spec.Selector},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: mapServiceTargetPortsToNetworkPolicyPorts(corev1.ProtocolTCP, service.Spec.Ports),
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &n.GatewayLabelSelector,
						},
					},
				},
			},
		},
	}, nil
}

func (n NetworkPolicy) getService(ctx context.Context, svcName, namespace string) (*corev1.Service, error) {
	svc := &corev1.Service{}
	err := n.Client.Get(ctx, types2.NamespacedName{Name: svcName, Namespace: namespace}, svc)
	if err != nil {
		return nil, fmt.Errorf("get service %q: %w", svcName, err)
	}

	return svc, nil
}

func (n NetworkPolicy) generateForExposedPorts(ctx context.Context, protocol corev1.Protocol, exposition types.Exposition, routes types.ExposedPorts) ([]*networkingv1.NetworkPolicy, error) {
	var errs []error
	netpols := make([]*networkingv1.NetworkPolicy, 0, len(exposition.HttpRoutes))
	for _, route := range routes {
		netpol, err := n.generateForExposedPort(ctx, exposition, route)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		err = exposition.SetOwner(netpol)
		if err != nil {
			errs = append(errs, err)
		}

		netpols = append(netpols, netpol)
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("generate network policies for %s routes: %w", protocol, errors.Join(errs...))
	}

	return netpols, nil
}

func (n NetworkPolicy) generateForExposedPort(ctx context.Context, exposition types.Exposition, route types.ExposedPort) (*networkingv1.NetworkPolicy, error) {
	service, err := n.getService(ctx, route.ServiceName, exposition.Namespace)
	if err != nil {
		return nil, fmt.Errorf("generate for %s route %q: %w", route.Protocol, route.Name, err)
	}

	selectionLabels := map[string]string{ownedByLabelKey: exposition.Name}
	maps.Insert(selectionLabels, maps.All(util.K8sCesServiceDiscoveryLabels))

	return &networkingv1.NetworkPolicy{
		Name:      fmt.Sprintf("%s-%s-%s-route", exposition.Name, route.Name, strings.ToLower(string(route.Protocol))),
		Namespace: exposition.Namespace,
		Labels:    selectionLabels,
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: service.Spec.Selector},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: mapServiceTargetPortsToNetworkPolicyPorts(corev1.ProtocolTCP, service.Spec.Ports),
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &n.GatewayLabelSelector,
						},
					},
				},
			},
		},
	}, nil
}

//nolint:dupl
func (n NetworkPolicy) upsertMultiple(ctx context.Context, exposition types.Exposition, desiredState []*networkingv1.NetworkPolicy) error {
	var errs []error
	existing := &networkingv1.NetworkPolicyList{}
	err := n.Client.List(ctx, existing, &client.ListOptions{Namespace: exposition.Namespace, LabelSelector: selectorFromExpositionName(exposition.Name)})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to list existing network policies: %w", err))
	}

	existingMap := make(map[string]networkingv1.NetworkPolicy, len(existing.Items))
	for _, existingObject := range existing.Items {
		existingMap[existingObject.Name] = existingObject
	}

	for _, desiredObject := range desiredState {
		updateRef := &networkingv1.NetworkPolicy{Name: desiredObject.Name, Namespace: desiredObject.Namespace}
		// only keep track of those that are not in the desired state to delete later
		delete(existingMap, desiredObject.Name)

		_, err := controllerutil.CreateOrUpdate(ctx, n.Client, updateRef, func() error {
			updateRef.Annotations = desiredObject.Annotations
			updateRef.OwnerReferences = desiredObject.OwnerReferences
			updateRef.Labels = desiredObject.Labels
			updateRef.Spec = desiredObject.Spec
			return nil
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create or update network policy %q: %w", desiredObject.Name, err))
		}
	}

	// delete objects not in desired state
	for _, existingObject := range existingMap {
		err := n.Client.Delete(ctx, &existingObject)
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete outdated network policy %q: %w", existingObject.Name, err))
		}
	}

	return errors.Join(errs...)
}

func mapServiceTargetPortsToNetworkPolicyPorts(protocol corev1.Protocol, ports []corev1.ServicePort) []networkingv1.NetworkPolicyPort {
	var result []networkingv1.NetworkPolicyPort
	for _, port := range ports {
		result = append(result, networkingv1.NetworkPolicyPort{
			Protocol: &protocol,
			Port:     &port.TargetPort,
		})
	}
	return result
}
