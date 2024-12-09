package nginx

import corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

type configMapInterface interface {
	corev1.ConfigMapInterface
}
