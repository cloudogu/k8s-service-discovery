package controllers

import (
	"context"

	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
)

type eventRecorder interface {
	record.EventRecorder
}

type GlobalConfigRepository interface {
	Get(context.Context) (libconfig.GlobalConfig, error)
	Watch(context.Context, ...libconfig.WatchFilter) (<-chan repository.GlobalConfigWatchResult, error)
	Update(ctx context.Context, globalConfig libconfig.GlobalConfig) (libconfig.GlobalConfig, error)
}

// IngressUpdater is responsible to create and update the actual ingress objects in the cluster.
type IngressUpdater interface {
	// UpsertIngressForService creates or updates the ingress object of the given service.
	UpsertIngressForService(ctx context.Context, service *corev1.Service) error
}

type NetworkPolicyUpdater interface {
	UpsertNetworkPoliciesForService(ctx context.Context, service *corev1.Service) error
	RemoveExposedPorts(ctx context.Context, serviceName string) error
	RemoveNetworkPolicy(ctx context.Context) error
}

type certificateSynchronizer interface {
	Synchronize(ctx context.Context) error
}

type AlternativeFQDNRedirector interface {
	RedirectAlternativeFQDN(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(targetObject metav1.Object) error, middlewareManager *expose.MiddlewareManager) error
}

type PortExposer interface {
	ExposePorts(ctx context.Context, namespace string, exposedPorts types.ExposedPorts, owner *metav1.OwnerReference) error
}

type IngressControllerSelector interface {
	GetSelector() map[string]string
}

type IngressController interface {
	AlternativeFQDNRedirector
	IngressControllerSelector
	PortExposer
}

type secretClient interface {
	corev1client.SecretInterface
}

type serviceClient interface {
	corev1client.ServiceInterface
}

type traefikInterface interface {
	traefikv1alpha1.TraefikV1alpha1Interface
}
