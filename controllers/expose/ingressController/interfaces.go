package ingressController

import (
	"context"

	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
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

type AlternativeFQDNRedirector interface {
	RedirectAlternativeFQDN(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(targetObject metav1.Object) error) error
}

type PortExposer interface {
	ExposePorts(ctx context.Context, namespace string, exposedPorts types.ExposedPorts, owner *metav1.OwnerReference) error
}

type IngressController interface {
	GetName() string
	GetControllerSpec() string
	GetRewriteAnnotationKey() string
	GetUseRegexKey() string
	GetSelector() map[string]string
	AlternativeFQDNRedirector
	PortExposer
}
