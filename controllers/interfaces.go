package controllers

import (
	"context"
	"github.com/cloudogu/k8s-dogu-operator/v2/api/ecoSystem"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
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

type ExposedPortUpdater interface {
	UpsertCesLoadbalancerService(ctx context.Context, service *corev1.Service) error
	RemoveExposedPorts(ctx context.Context, serviceName string) error
}

type NetworkPolicyUpdater interface {
	UpsertNetworkPoliciesForService(ctx context.Context, service *corev1.Service) error
	RemoveExposedPorts(ctx context.Context, serviceName string) error
}

//nolint:unused
//goland:noinspection GoUnusedType
type doguInterace interface {
	ecoSystem.DoguInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type ingressController interface {
	GetName() string
	GetControllerSpec() string
	GetRewriteAnnotationKey() string
	GetAdditionalConfigurationKey() string
	tcpUpdServiceExposer
}

// tcpUpdServiceExposer is used to expose non http services.
type tcpUpdServiceExposer interface {
	// ExposeOrUpdateExposedPorts adds or updates the exposing of the exposed ports from the service in the cluster. These are typically
	// entries in a configmap.
	ExposeOrUpdateExposedPorts(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts) error
	// DeleteExposedPorts removes the exposing of the exposed ports from the service in the cluster. These are typically
	// entries in a configmap.
	DeleteExposedPorts(ctx context.Context, namespace string, targetServiceName string) error
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
