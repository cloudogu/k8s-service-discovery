package traefik

import (
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	netv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
)

type configMapInterface interface {
	corev1.ConfigMapInterface
}

type ingressInterface interface {
	netv1.IngressInterface
}

type traefikInterface interface {
	v1alpha1.TraefikV1alpha1Interface
}
