package ingressController

import (
	"context"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type configMapInterface interface {
	corev1.ConfigMapInterface
}

// tcpUpdServiceExposer is used to expose non http services.
type tcpUpdServiceExposer interface {
	// ExposeOrUpdateExposedPorts adds or updates the exposing of the exposed ports in the dogu from the cluster. These are typically
	// entries in a configmap.
	ExposeOrUpdateExposedPorts(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts) error
	// DeleteExposedPorts removes the exposing of the exposed ports in the dogu from the cluster. These are typically
	// entries in a configmap.
	DeleteExposedPorts(ctx context.Context, namespace string, targetServiceName string) error
}

type IngressController interface {
	GetName() string
	GetControllerSpec() string
	GetRewriteAnnotationKey() string
	GetUseRegexKey() string
	tcpUpdServiceExposer
}
