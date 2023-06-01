package config

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	warpConfigMap    = "k8s-ces-warp-config"
	MenuConfigMap    = "k8s-ces-menu-json"
	EnvVarStage      = "STAGE"
	StageDevelopment = "development"
	DevConfigPath    = "k8s/dev-resources/k8s-ces-warp-config.yaml"
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

// Source in etcd
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
	if os.Getenv(EnvVarStage) == StageDevelopment {
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
