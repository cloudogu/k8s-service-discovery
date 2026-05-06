package expose

import (
	"fmt"
	"maps"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/domain"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

type defaultNetworkPolicyGenerator struct {
	networkPoliciesDisabled bool
	namespace               string
	ingressController       ingressController
	allowedCIDR             string
}

func newNetworkPolicyGenerator(
	networkPoliciesDisabled bool,
	namespace string,
	ingressController ingressController,
	allowedCIDR string,
) *defaultNetworkPolicyGenerator {
	return &defaultNetworkPolicyGenerator{
		networkPoliciesDisabled: networkPoliciesDisabled,
		namespace:               namespace,
		ingressController:       ingressController,
		allowedCIDR:             allowedCIDR,
	}
}

func (n *defaultNetworkPolicyGenerator) Generate(definition domain.ExposedPortsDefinition) []*networkingv1.NetworkPolicy {
	if n.networkPoliciesDisabled ||
		len(definition.TcpRoutes)+len(definition.UdpRoutes) == 0 {
		return nil
	}

	labels := map[string]string{ownedByLabelKey: definition.BaseName}
	maps.Insert(labels, maps.All(util.K8sCesServiceDiscoveryLabels))

	return []*networkingv1.NetworkPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-exposed-ports", definition.BaseName),
				Namespace: n.namespace,
				Labels:    labels,
			},
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: n.ingressController.GetSelector()},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
				Ingress: []networkingv1.NetworkPolicyIngressRule{
					{
						Ports: getNetworkPolicyPorts(definition),
						From: []networkingv1.NetworkPolicyPeer{
							{
								IPBlock: &networkingv1.IPBlock{
									CIDR: n.allowedCIDR,
								},
							},
						},
					},
				},
			},
		},
	}
}

func getNetworkPolicyPorts(definition domain.ExposedPortsDefinition) []networkingv1.NetworkPolicyPort {
	var networkPolicyPorts []networkingv1.NetworkPolicyPort
	for _, tcpRoute := range definition.TcpRoutes {
		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Protocol: ptr.To(corev1.ProtocolTCP),
			Port:     ptr.To(intstr.FromInt32(tcpRoute.Port)),
		})
	}
	for _, udpRoute := range definition.UdpRoutes {
		networkPolicyPorts = append(networkPolicyPorts, networkingv1.NetworkPolicyPort{
			Protocol: ptr.To(corev1.ProtocolUDP),
			Port:     ptr.To(intstr.FromInt32(udpRoute.Port)),
		})
	}
	return networkPolicyPorts
}
