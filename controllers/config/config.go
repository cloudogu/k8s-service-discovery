package config

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
	"strconv"
)

const (
	warpConfigMap           = "k8s-ces-warp-config"
	MenuConfigMap           = "k8s-ces-menu-json"
	StageLocal              = "local"
	DevConfigPath           = "k8s/dev-resources/k8s-ces-warp-config.yaml"
	StageEnvVar             = "STAGE"
	ingressControllerEnvVar = "INGRESS_CONTROLLER"
	// namespaceEnvVar defines the name of the environment variables given into the service discovery to define the
	// namespace that should be watched by the service discovery.
	namespaceEnvVar = "WATCH_NAMESPACE"

	// networkPolicyCIDREnvVar define the ip range which is allowed to access the ingress controller if networkpolicies are enabled.
	networkPolicyCIDREnvVar    = "NETWORK_POLICIES_CIDR"
	networkPolicyEnabledEnvVar = "NETWORK_POLICIES_ENABLED"
)

var (
	logger = ctrl.Log.WithName("k8s-service-discovery.config")
)

// Order can be used to modify ordering via configuration
type Order map[string]int

// Configuration for warp menu creation
type Configuration struct {
	Sources []Source
	Target  string
	Order   Order
	Support []SupportSource
}

// Source in global config
type Source struct {
	Path string
	Type string
	Tag  string
}

// SupportSource for SupportEntries from yaml
type SupportSource struct {
	Identifier string
	External   bool
	Href       string
}

// ReadConfiguration reads the service discovery configuration. Either from file in development mode with environment
// variable stage=development or from the cluster state
func ReadConfiguration(ctx context.Context, client client.Client, namespace string) (*Configuration, error) {
	if os.Getenv(StageEnvVar) == StageLocal {
		return readWarpConfigFromFile(DevConfigPath)
	}
	return readWarpConfigFromCluster(ctx, client, namespace)
}

func readWarpConfigFromFile(path string) (*Configuration, error) {
	config := &Configuration{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("could not find configuration at %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration %s: %w", path, err)
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration %s: %w", path, err)
	}

	return config, nil
}

func readWarpConfigFromCluster(ctx context.Context, client client.Client, namespace string) (*Configuration, error) {
	configmap := &corev1.ConfigMap{}
	objectKey := types.NamespacedName{
		Namespace: namespace,
		Name:      warpConfigMap,
	}
	err := client.Get(ctx, objectKey, configmap)
	if err != nil {
		return nil, fmt.Errorf("failed to get warp menu configmap: %w", err)
	}

	data := configmap.Data["warp"]
	conf := &Configuration{}
	err = yaml.Unmarshal([]byte(data), conf)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml from warp config: %w", err)
	}

	return conf, nil
}

func ReadIngressController() string {
	envIngressController := os.Getenv(ingressControllerEnvVar)
	return envIngressController
}

func ReadWatchNamespace() (string, error) {
	watchNamespace, found := os.LookupEnv(namespaceEnvVar)
	if !found {
		return "", fmt.Errorf("failed to read namespace to watch from environment variable [%s], please set the variable and try again", namespaceEnvVar)
	}
	logger.Info(fmt.Sprintf("found target namespace: [%s]", watchNamespace))

	return watchNamespace, nil
}

func ReadNetworkPolicyCIDR() (string, error) {
	cidr, found := os.LookupEnv(networkPolicyCIDREnvVar)
	if !found {
		return "", fmt.Errorf("failed to read cidr from environment variable [%s], please set the variable and try again", networkPolicyCIDREnvVar)
	}
	logger.Info(fmt.Sprintf("found ingress controller network policy cidr: [%s]", cidr))

	return cidr, nil
}

func ReadNetworkPolicyEnabled() (bool, error) {
	enabled, found := os.LookupEnv(networkPolicyEnabledEnvVar)
	if !found {
		return true, fmt.Errorf("failed to read flag network policy enabled from environment variable [%s], please set the variable and try again", networkPolicyEnabledEnvVar)
	}
	parseBool, err := strconv.ParseBool(enabled)
	if err != nil {
		return true, err
	}

	logger.Info(fmt.Sprintf("network policies enabled: [%s]", enabled))

	return parseBool, nil
}
