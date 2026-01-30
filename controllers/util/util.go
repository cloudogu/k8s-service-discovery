package util

import (
	"context"
	"fmt"
	"strings"

	doguv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var K8sCesServiceDiscoveryLabels = map[string]string{"app": "ces", "app.kubernetes.io/name": "k8s-service-discovery"}

const (
	appLabelKey      = "app"
	appLabelValueCes = "ces"
	legacyDoguLabel  = "dogu"
)

const (
	MaintenanceConfigMapName = "maintenance"
)

type ExposedPorts []ExposedPort

type ExposedPort struct {
	Protocol   corev1.Protocol `json:"protocol"`
	Port       int32           `json:"port"`
	TargetPort int32           `json:"targetPort"`
}

func (ep ExposedPort) String() string {
	return fmt.Sprintf("{Port: %d, TargetPort: %d, Protocol: %s}", ep.Port, ep.TargetPort, ep.Protocol)
}

func ContainsChars(s string) bool {
	return len(strings.TrimSpace(s)) != 0
}

func HasDoguLabel(deployment client.Object) bool {
	for label := range deployment.GetLabels() {
		if label == legacyDoguLabel || label == doguv2.DoguLabelName {
			return true
		}
	}

	return false
}

func GetAppLabel() map[string]string {
	return map[string]string{appLabelKey: appLabelValueCes}
}

func GetMaintenanceModeActive(ctx context.Context, client k8sClient, namespace string) (bool, error) {
	maintenanceConfig := &corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{Name: MaintenanceConfigMapName, Namespace: namespace}, maintenanceConfig)
	if errors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to get config for maintenance mode: %w", err)
	}

	return IsMaintenanceModeActive(maintenanceConfig), nil
}

func IsMaintenanceModeActive(config *corev1.ConfigMap) bool {
	activeString, ok := config.Data["active"]
	return ok && strings.TrimSpace(strings.ToLower(activeString)) == "true"
}
