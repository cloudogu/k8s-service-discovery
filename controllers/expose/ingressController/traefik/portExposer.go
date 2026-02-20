package traefik

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PortExposer struct {
	traefikInterface traefikInterface
	namespace        string
}

// ExposePorts materializes the given TCP/UDP port forwards for Traefik by
// creating or updating IngressRouteTCP / IngressRouteUDP CRDs per exposed port.
//
// This function is safe to call repeatedly (upsert semantics).
//
// Only TCP and UDP protocols are supported. Any other protocol values are logged and ignored.
func (p PortExposer) ExposePorts(ctx context.Context, namespace string, exposedPorts types.ExposedPorts, owner *metav1.OwnerReference) error {
	logger := log.FromContext(ctx)

	for _, port := range exposedPorts {
		switch port.Protocol {
		case corev1.ProtocolTCP:
			route := createIngressRouteTCP(namespace, port, owner)
			if err := p.upsertIngressRouteTCP(ctx, namespace, route); err != nil {
				return fmt.Errorf("failed to upsert IngressRouteTCP for port %s: %w", port.PortString(), err)
			}
		case corev1.ProtocolUDP:
			route := createIngressRouteUDP(namespace, port, owner)
			if err := p.upsertIngressRouteUDP(ctx, namespace, route); err != nil {
				return fmt.Errorf("failed to upsert IngressRouteUDP for port %s: %w", port.PortString(), err)
			}
		default:
			logger.Info("unsupported protocol for exposed port, port will be ignored", "name", port.Name, "protocol", port.Protocol)
		}
	}

	return nil
}

func (p PortExposer) upsertIngressRouteTCP(ctx context.Context, namespace string, route *traefikv1alpha1.IngressRouteTCP) error {
	client := p.traefikInterface.IngressRouteTCPs(namespace)

	_, err := client.Create(ctx, route, metav1.CreateOptions{})
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create IngressRouteTCP: %w", err)
	}

	existing, gErr := client.Get(ctx, route.Name, metav1.GetOptions{})
	if gErr != nil {
		return fmt.Errorf("failed to get existing IngressRouteTCP: %w", gErr)
	}

	route.ResourceVersion = existing.ResourceVersion
	_, uErr := client.Update(ctx, route, metav1.UpdateOptions{})
	if uErr != nil {
		return fmt.Errorf("failed to update IngressRouteTCP: %w", uErr)
	}

	return nil
}

func (p PortExposer) upsertIngressRouteUDP(ctx context.Context, namespace string, route *traefikv1alpha1.IngressRouteUDP) error {
	client := p.traefikInterface.IngressRouteUDPs(namespace)

	_, err := client.Create(ctx, route, metav1.CreateOptions{})
	if err == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create IngressRouteUDP: %w", err)
	}

	existing, gErr := client.Get(ctx, route.Name, metav1.GetOptions{})
	if gErr != nil {
		return fmt.Errorf("failed to get existing IngressRouteUDP: %w", gErr)
	}

	route.ResourceVersion = existing.ResourceVersion
	_, uErr := client.Update(ctx, route, metav1.UpdateOptions{})
	if uErr != nil {
		return fmt.Errorf("failed to update IngressRouteUDP: %w", uErr)
	}

	return nil
}

func createIngressRouteTCP(namespace string, port types.ExposedPort, owner *metav1.OwnerReference) *traefikv1alpha1.IngressRouteTCP {
	route := &traefikv1alpha1.IngressRouteTCP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-tcp", port.ServiceName, port.PortString()),
			Namespace: namespace,
			Labels:    util.K8sCesServiceDiscoveryLabels,
		},
		Spec: traefikv1alpha1.IngressRouteTCPSpec{
			EntryPoints: []string{fmt.Sprintf("tcp-%s", port.PortString())},
			Routes: []traefikv1alpha1.RouteTCP{
				{
					Match: "HostSNI(`*`)",
					Services: []traefikv1alpha1.ServiceTCP{
						{
							Name:      port.ServiceName,
							Namespace: namespace,
							Port:      intstr.FromInt32(port.TargetPort),
						},
					},
				},
			},
		},
	}

	if owner != nil {
		route.SetOwnerReferences([]metav1.OwnerReference{*owner})
	}

	return route
}

func createIngressRouteUDP(namespace string, port types.ExposedPort, owner *metav1.OwnerReference) *traefikv1alpha1.IngressRouteUDP {
	route := &traefikv1alpha1.IngressRouteUDP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-udp", port.ServiceName, port.PortString()),
			Namespace: namespace,
			Labels:    util.K8sCesServiceDiscoveryLabels,
		},
		Spec: traefikv1alpha1.IngressRouteUDPSpec{
			EntryPoints: []string{fmt.Sprintf("udp-%s", port.PortString())},
			Routes: []traefikv1alpha1.RouteUDP{
				{
					Services: []traefikv1alpha1.ServiceUDP{
						{
							Name:      port.ServiceName,
							Namespace: namespace,
							Port:      intstr.FromInt32(port.TargetPort),
						},
					},
				},
			},
		},
	}

	if owner != nil {
		route.SetOwnerReferences([]metav1.OwnerReference{*owner})
	}

	return route
}
