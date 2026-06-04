package controllers

import (
	"context"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MaintenanceAdapter interface {
	GetStatus(ctx context.Context) (repository.MaintenanceModeDescription, bool, error)
}

type GlobalConfigRepository interface {
	Get(context.Context) (libconfig.GlobalConfig, error)
	Watch(context.Context, ...libconfig.WatchFilter) (<-chan repository.GlobalConfigWatchResult, error)
	Update(ctx context.Context, globalConfig libconfig.GlobalConfig) (libconfig.GlobalConfig, error)
}

// IngressUpdater is responsible to create and update the actual ingress objects in the cluster.
type IngressUpdater interface {
	// UpsertForService creates or updates the ingress objects of the given service.
	UpsertForService(ctx context.Context, service *corev1.Service) error
	// UpsertForExposition creates or updates the ingress objects of the given exposition.
	UpsertForExposition(ctx context.Context, exposition *expositionv1.Exposition) error
}

type NetworkPolicyUpdater interface {
	UpsertNetworkPoliciesForService(ctx context.Context, service *corev1.Service) error
	UpsertNetworkPoliciesForExposition(ctx context.Context, exposition *expositionv1.Exposition) error
}

type certificateSynchronizer interface {
	Synchronize(ctx context.Context) error
}

type AlternativeFQDNRedirector interface {
	RedirectAlternativeFQDN(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(targetObject metav1.Object) error) error
}

type PortExposer interface {
	ExposePorts(ctx context.Context, namespace string, exposedPorts types.ExposedPorts) error
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

type k8sClient interface {
	client.Client
}

type ExpositionService interface {
	GetOwnableTypes() []client.Object
	ProcessExposition(ctx context.Context, exposition types.Exposition) error
}
