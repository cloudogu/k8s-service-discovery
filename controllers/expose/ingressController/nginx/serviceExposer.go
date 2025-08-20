package nginx

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/retry-lib/retry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	portHTTP  = 80
	portHTTPS = 443
)

type ingressNginxTcpUpdExposer struct {
	configMapInterface configMapInterface
}

// NewIngressNginxTCPUDPExposer creates a new instance of the ingressNginxTcpUpdExposer.
func NewIngressNginxTCPUDPExposer(configMapInterface configMapInterface) *ingressNginxTcpUpdExposer {
	return &ingressNginxTcpUpdExposer{configMapInterface: configMapInterface}
}

// ExposeOrUpdateExposedPorts creates or updates the matching tcp/udp configmap for nginx routing.
// It also deletes all legacy entries from the service. Port 80 and 443 will be ignored.
//
// see: https://kubernetes.github.io/ingress-nginx/user-guide/exposing-tcp-udp-services/
func (intue *ingressNginxTcpUpdExposer) ExposeOrUpdateExposedPorts(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts) error {
	logger := log.FromContext(ctx)
	if len(exposedPorts) < 1 {
		logger.Info(fmt.Sprintf("Skipping tcp/udp port creation because the service %q has no exposed ports...", targetServiceName))
		return nil
	}

	err := intue.exposeOrUpdatePortsForProtocol(ctx, namespace, targetServiceName, exposedPorts, corev1.ProtocolTCP)
	if err != nil {
		return err
	}

	return intue.exposeOrUpdatePortsForProtocol(ctx, namespace, targetServiceName, exposedPorts, corev1.ProtocolUDP)
}

func (intue *ingressNginxTcpUpdExposer) exposeOrUpdatePortsForProtocol(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts, protocol corev1.Protocol) error {
	configMapName := getConfigMapNameForProtocol(protocol)
	err := retry.OnConflict(func() error {
		cm, err := intue.configMapInterface.Get(ctx, configMapName, metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get configmap %s: %w", getConfigMapNameForProtocol(protocol), err)
		} else if err != nil && apierrors.IsNotFound(err) {
			_, err = intue.createNginxExposeConfigMapForProtocol(ctx, namespace, targetServiceName, exposedPorts, protocol)
			return err
		}

		logger := log.FromContext(ctx)
		oldLen := len(cm.Data)
		cm.Data = filterServices(cm, namespace, targetServiceName)
		exposedPortsByType := getExposedPortsByType(exposedPorts, protocol)
		if oldLen == len(cm.Data) && len(exposedPortsByType) == 0 {
			logger.Info(fmt.Sprintf("Skipping %s port exposing for service %q because there are no changes...", string(protocol), targetServiceName))
			return nil
		}

		for _, port := range exposedPortsByType {
			cm.Data[getServiceEntryKey(port)] = getServiceEntryValue(namespace, targetServiceName, port)
		}

		logger.Info(fmt.Sprintf("Update %s port exposing for service %s...", string(protocol), targetServiceName))

		_, err = intue.configMapInterface.Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})

	if err != nil {
		return updateCmErr(configMapName, err)
	}

	return nil
}

func (intue *ingressNginxTcpUpdExposer) createNginxExposeConfigMapForProtocol(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts, protocol corev1.Protocol) (*corev1.ConfigMap, error) {
	exposedPortsByProtocol := getExposedPortsByType(exposedPorts, protocol)
	if len(exposedPortsByProtocol) < 1 {
		return nil, nil
	}

	cmName := getConfigMapNameForProtocol(protocol)
	cmData := map[string]string{}
	for _, port := range exposedPortsByProtocol {
		cmData[getServiceEntryKey(port)] = getServiceEntryValue(namespace, targetServiceName, port)
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: namespace},
		Data:       cmData,
	}

	_, err := intue.configMapInterface.Create(ctx, cm, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create configmap %s: %w", cmName, err)
	}

	return cm, nil
}

func getConfigMapNameForProtocol(protocol corev1.Protocol) string {
	return fmt.Sprintf("%s-services", strings.ToLower(string(protocol)))
}

func getServiceEntryKey(port util.ExposedPort) string {
	return fmt.Sprintf("%d", port.Port)
}

func getServiceEntryValue(namespace string, targetServiceName string, port util.ExposedPort) string {
	return fmt.Sprintf("%s:%d", getServiceEntryValuePrefix(namespace, targetServiceName), port.TargetPort)
}

func getServiceEntryValuePrefix(namespace string, targetServiceName string) string {
	return fmt.Sprintf("%s/%s", namespace, targetServiceName)
}

// filterServices removes all entries from the data map which route traffic to the given service.
func filterServices(cm *corev1.ConfigMap, namespace string, targetServiceName string) map[string]string {
	data := cm.Data
	if data == nil {
		return map[string]string{}
	}

	for key, value := range data {
		if strings.Contains(value, getServiceEntryValuePrefix(namespace, targetServiceName)) {
			delete(data, key)
		}
	}

	return data
}

func getExposedPortsByType(exposedPorts util.ExposedPorts, protocol corev1.Protocol) util.ExposedPorts {
	var result util.ExposedPorts
	for _, port := range exposedPorts {
		if port.Port == portHTTP || port.Port == portHTTPS {
			continue
		}

		if strings.EqualFold(string(port.Protocol), string(protocol)) {
			result = append(result, port)
		}
	}

	return result
}

// DeleteExposedPorts removes all service related entries in the corresponding tcp/udp configmaps.
// If the configmap has no entries left this method won't delete the configmap. This would lead to numerous
// errors in the nginx log.
func (intue *ingressNginxTcpUpdExposer) DeleteExposedPorts(ctx context.Context, namespace string, targetServiceName string) error {
	err := intue.deletePortsForProtocolWithRetry(ctx, namespace, targetServiceName, corev1.ProtocolTCP)
	if err != nil {
		return err
	}

	return intue.deletePortsForProtocolWithRetry(ctx, namespace, targetServiceName, corev1.ProtocolUDP)
}

func (intue *ingressNginxTcpUpdExposer) deletePortsForProtocolWithRetry(ctx context.Context, namespace string, targetServiceName string, protocol corev1.Protocol) error {
	configMapName := getConfigMapNameForProtocol(protocol)
	err := retry.OnConflict(func() error {
		return intue.deletePortsForProtocol(ctx, namespace, targetServiceName, protocol)
	})

	if err != nil {
		return updateCmErr(configMapName, err)
	}

	return nil
}

func (intue *ingressNginxTcpUpdExposer) deletePortsForProtocol(ctx context.Context, namespace string, targetServiceName string, protocol corev1.Protocol) error {
	configMapName := getConfigMapNameForProtocol(protocol)
	cm, err := intue.configMapInterface.Get(ctx, configMapName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get configmap %s: %w", getConfigMapNameForProtocol(protocol), err)
		} else {
			return nil
		}
	}

	if len(cm.Data) == 0 {
		return nil
	}

	changed := false
	for key, value := range cm.Data {
		if strings.Contains(value, getServiceEntryValuePrefix(namespace, targetServiceName)) {
			changed = true
			delete(cm.Data, key)
		}
	}

	if !changed {
		log.FromContext(ctx).Info(fmt.Sprintf("Skipping exposed port deletion because the service %q has no exposed ports for protocol %s...", targetServiceName, protocol))
		return nil
	}

	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("Update %s port exposing for service %q...", string(protocol), targetServiceName))
	// Do not delete the configmap, even it contains no ports. That would throw errors in nginx-ingress log.
	_, err = intue.configMapInterface.Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func updateCmErr(configMapName string, err error) error {
	return fmt.Errorf("failed to update configmap %s: %w", configMapName, err)
}
