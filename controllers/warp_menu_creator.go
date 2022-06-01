package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/cesapp-lib/core"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"sigs.k8s.io/yaml"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const warpConfig = "k8s-ces-warp-config"
const EnvVarStage = "STAGE"
const StageDevelopment = "development"
const DevConfigPath = "k8s/dev-resources/k8s-ces-warp-config.yaml"

// Configuration main configuration object
type Configuration struct {
	Endpoint string
	Warp     warp.Configuration
}

// WarpMenuCreator
type WarpMenuCreator struct {
	Client        client.Client  `json:"client"`
	Namespace     string         `json:"namespace"`
	Endpoint      string         `json:"endpoint"`
	Configuration *Configuration `json:"configuration"`
}

// NewWarpMenuCreator
func NewWarpMenuCreator(client client.Client, namespace string) WarpMenuCreator {
	endpoint := fmt.Sprintf("http://etcd.%s.svc.cluster.local:4001", namespace)
	return WarpMenuCreator{
		Client:    client,
		Namespace: namespace,
		Endpoint:  endpoint,
	}
}

// Start starts the runnable.
func (wmc WarpMenuCreator) Start(ctx context.Context) error {
	return wmc.CreateWarpMenu(ctx)
}

// CreateWarpMenu reads the warp configuration and starts watchers to refresh the menu.json configmap
// in background.
func (wmc WarpMenuCreator) CreateWarpMenu(ctx context.Context) error {
	config, err := wmc.ReadConfiguration(ctx, wmc.Client)
	if err != nil {
		return fmt.Errorf("failed to read configuration: %w", err)
	}

	cesRegistry, err := wmc.createEtcdRegistry()
	if err != nil {
		return fmt.Errorf("failed to create etcd registry: %w", err)
	}

	var syncWaitGroup sync.WaitGroup
	syncWaitGroup.Add(1)
	go func() {
		warp.Run(config, cesRegistry.RootConfig(), wmc.Client)
		syncWaitGroup.Done()
	}()
	syncWaitGroup.Wait()

	return nil
}

func (wmc *WarpMenuCreator) ReadConfiguration(ctx context.Context, client client.Client) (*warp.Configuration, error) {
	if os.Getenv(EnvVarStage) == StageDevelopment {
		return wmc.ReadWarpConfigFromFile(DevConfigPath)
	}
	return wmc.ReadWarpConfigFromCluster(ctx, client)
}

func (wmc *WarpMenuCreator) ReadWarpConfigFromCluster(ctx context.Context, client client.Client) (*warp.Configuration, error) {
	configmap := &corev1.ConfigMap{}
	objectKey := types.NamespacedName{
		Namespace: wmc.Namespace,
		Name:      warpConfig,
	}
	err := client.Get(ctx, objectKey, configmap)
	if err != nil {
		return nil, fmt.Errorf("failed to get warp menu configmap: %w", err)
	}

	confStr := configmap.Data["warp"]
	conf := &warp.Configuration{}
	err = yaml.Unmarshal([]byte(confStr), conf)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml from warp config: %w", err)
	}

	return conf, nil
}

func (wmc *WarpMenuCreator) createEtcdRegistry() (registry.Registry, error) {
	r, err := registry.New(core.Registry{
		Type:      "etcd",
		Endpoints: []string{wmc.Endpoint},
	})

	return r, err
}

func (wmc *WarpMenuCreator) ReadWarpConfigFromFile(path string) (*warp.Configuration, error) {
	config := &warp.Configuration{}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return config, fmt.Errorf("could not find configuration at %s", path)
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration %s: %w", path, err)
	}

	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration %s: %w", path, err)
	}

	logrus.Info(fmt.Sprintf("%v", config))

	return config, nil
}
