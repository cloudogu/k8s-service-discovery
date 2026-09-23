package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const PortExposerLabel = "k8s.cloudogu.com/custom-routing"

// isExposerService reports whether the object is a is at least a k8s.cloudogu.com/port-exposer Service: a ClusterIP
// corev1.Service carrying the port-exposer label. Other Labels such as DoguName can be valid to by passing them as parameters
func isExposerService(object client.Object, labels ...string) bool {
	exposerService, ok := object.(*corev1.Service)
	if !ok {
		return false
	}

	if exposerService.Spec.Type != corev1.ServiceTypeClusterIP {
		return false
	}

	serviceLabels := exposerService.GetLabels()
	if len(serviceLabels) == 0 {
		return false
	}

	labels = append(labels, PortExposerLabel)

	for _, label := range labels {
		if _, ok := serviceLabels[label]; ok {
			return true
		}
	}

	return false
}
