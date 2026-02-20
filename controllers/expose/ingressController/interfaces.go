package ingressController

import (
	"context"

	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

type AlternativeFQDNRedirector interface {
	RedirectAlternativeFQDN(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(targetObject metav1.Object) error) error
}

type PortExposer interface {
	ExposePorts(ctx context.Context, namespace string, exposedPorts types.ExposedPorts, owner *metav1.OwnerReference) error
}

type IngressController interface {
	GetName() string
	GetRewriteAnnotationKey() string
	GetSelector() map[string]string
	AlternativeFQDNRedirector
	PortExposer
}
