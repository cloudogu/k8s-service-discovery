package util

import (
	"context"
	"fmt"
	doguv2 "github.com/cloudogu/k8s-dogu-operator/v2/api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

var K8sCesServiceDiscoveryLabels = map[string]string{"app": "ces", "app.kubernetes.io/name": "k8s-service-discovery"}

func ContainsChars(s string) bool {
	return len(strings.TrimSpace(s)) != 0
}

const legacyDoguLabel = "dogu"

func HasDoguLabel(deployment client.Object) bool {
	for label := range deployment.GetLabels() {
		if label == legacyDoguLabel || label == doguv2.DoguLabelName {
			return true
		}
	}

	return false
}

func IsMaintenanceModeActive(ctx context.Context, globalConfigRepo GlobalConfigRepository) (bool, error) {
	globalConfig, err := globalConfigRepo.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get global config for maintenance mode: %w", err)
	}

	get, ok := globalConfig.Get("maintenance")
	if !ok || !ContainsChars(get.String()) {
		return false, nil
	}

	return true, nil
}
