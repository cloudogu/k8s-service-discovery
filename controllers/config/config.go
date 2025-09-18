package config

import (
	"fmt"
	"os"
	"strconv"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
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
