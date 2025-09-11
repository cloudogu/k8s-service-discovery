package controllers

import (
	"context"

	"github.com/cloudogu/k8s-dogu-operator/v3/api/ecoSystem"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	netv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
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

//nolint:unused
//goland:noinspection GoUnusedType
type doguInterface interface {
	ecoSystem.DoguInterface
}

type AlternativeFQDNRedirector interface {
	RedirectAlternativeFQDN(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(targetObject metav1.Object) error) error
}

type PortExposer interface {
	ExposePorts(ctx context.Context, namespace string, exposedPorts types.ExposedPorts, owner *metav1.OwnerReference) error
}

type IngressControllerSelector interface {
	GetSelector() map[string]string
}

//nolint:unused
//goland:noinspection GoUnusedType
type ingressInterface interface {
	netv1.IngressInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type clientSetInterface interface {
	kubernetes.Interface
}

//nolint:unused
//goland:noinspection GoUnusedType
type appsv1Interface interface {
	appsv1.AppsV1Interface
}

//nolint:unused
//goland:noinspection GoUnusedType
type netInterface interface {
	netv1.NetworkingV1Interface
}

//nolint:unused
//goland:noinspection GoUnusedType
type deploymentInterface interface {
	appsv1.DeploymentInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type ingressClassInterface interface {
	netv1.IngressClassInterface
}

type secretClient interface {
	corev1client.SecretInterface
}

type serviceClient interface {
	corev1client.ServiceInterface
}
