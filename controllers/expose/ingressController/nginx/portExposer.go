package nginx

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PortExposer struct {
	configMapInterface configMapInterface
}

// ExposePorts materializes the given TCP/UDP port forwards for ingress-nginx by
// writing the controller's well-known "tcp-services" / "udp-services" ConfigMaps.
//
// This function is safe to call repeatedly; it overwrites the ConfigMaps
// with the current desired mappings (actual upsert semantics depend on intue.upsertConfigMap).
//
// Only TCP and UDP protocols are supported. Any other protocol values are logged and ignored.
// see: https://kubernetes.github.io/ingress-nginx/user-guide/exposing-tcp-udp-services/
func (p PortExposer) ExposePorts(ctx context.Context, namespace string, exposedPorts types.ExposedPorts, owner *metav1.OwnerReference) error {
	logger := log.FromContext(ctx)

	tcpMap := make(map[string]string, len(exposedPorts))
	udpMap := make(map[string]string, len(exposedPorts))

	for _, port := range exposedPorts {
		targetService := fmt.Sprintf("%s/%s:%d", namespace, port.ServiceName, port.TargetPort)

		switch port.Protocol {
		case corev1.ProtocolTCP:
			tcpMap[port.PortString()] = targetService
		case corev1.ProtocolUDP:
			udpMap[port.PortString()] = targetService
		default:
			logger.Info("unsupported protocol for exposed port, port will be ignored", "name", port.Name, "protocol", port.Protocol)
		}
	}

	tcpCfgMap := createExposeConfigMap(namespace, corev1.ProtocolTCP, tcpMap, owner)
	udpCfgMap := createExposeConfigMap(namespace, corev1.ProtocolUDP, udpMap, owner)

	if uErr := p.upsertConfigMap(ctx, tcpCfgMap); uErr != nil {
		return fmt.Errorf("failed to upsert exposed ports for protocol %s: %w", corev1.ProtocolTCP, uErr)
	}

	if uErr := p.upsertConfigMap(ctx, udpCfgMap); uErr != nil {
		return fmt.Errorf("failed to upsert exposed ports for protocol %s: %w", corev1.ProtocolUDP, uErr)
	}

	return nil
}

func (p PortExposer) upsertConfigMap(ctx context.Context, cm *corev1.ConfigMap) error {
	_, cErr := p.configMapInterface.Create(ctx, cm, metav1.CreateOptions{})
	if cErr == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(cErr) {
		return fmt.Errorf("failed to create configMap: %w", cErr)
	}

	_, uErr := p.configMapInterface.Update(ctx, cm, metav1.UpdateOptions{})
	if uErr != nil {
		return fmt.Errorf("failed to update configMap: %w", uErr)
	}

	return nil
}

func createExposeConfigMap(namespace string, protocol corev1.Protocol, data map[string]string, owner *metav1.OwnerReference) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getConfigMapNameForProtocol(protocol),
			Namespace: namespace,
			Labels:    util.K8sCesServiceDiscoveryLabels,
		},
		Data: data,
	}

	if owner != nil {
		cm.SetOwnerReferences([]metav1.OwnerReference{*owner})
	}

	return cm
}

func getConfigMapNameForProtocol(protocol corev1.Protocol) string {
	return fmt.Sprintf("%s-services", strings.ToLower(string(protocol)))
}
