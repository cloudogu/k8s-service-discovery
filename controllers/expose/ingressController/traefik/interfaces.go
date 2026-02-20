package traefik

import (
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	netv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
)

type configMapInterface interface {
	corev1.ConfigMapInterface
}

type ingressInterface interface {
	netv1.IngressInterface
}
