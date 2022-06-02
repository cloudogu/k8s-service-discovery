package warp

import (
	"context"
	"fmt"
	"github.com/cloudogu/cesapp-lib/core"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	coreosclient "github.com/coreos/etcd/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"log"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Watcher is used to watch a registry and for every change he reads from the registry a specific config path,
// build warp menu categories and writes them to a configmap.
type Watcher struct {
	configuration   *config.Configuration
	registryToWatch registry.WatchConfigurationContext
	k8sClient       client.Client
	configReader    *ConfigReader
	namespace       string
	doneChannel     <-chan struct{}
}

// NewWatcher creates a new Watcher instance to build the warp menu
func NewWatcher(ctx context.Context, k8sClient client.Client, namespace string) (*Watcher, error) {
	warpConfig, err := config.ReadConfiguration(ctx, k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration: %w", err)
	}

	cesRegistry, err := createEtcdRegistry(namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd registry: %w", err)
	}

	reader := &ConfigReader{
		registry:          cesRegistry.RootConfig(),
		configuration:     warpConfig,
		doguConverter:     &types.DoguConverter{},
		externalConverter: &types.ExternalConverter{},
	}

	return &Watcher{
		configuration:   warpConfig,
		registryToWatch: cesRegistry.RootConfig(),
		k8sClient:       k8sClient,
		namespace:       namespace,
		configReader:    reader,
		doneChannel:     ctx.Done(),
	}, nil
}

func createEtcdRegistry(namespace string) (registry.Registry, error) {
	r, err := registry.New(core.Registry{
		Type:      "etcd",
		Endpoints: []string{fmt.Sprintf("http://etcd.%s.svc.cluster.local:4001", namespace)},
	})

	return r, err
}

// Run creates the warp menu and update the menu whenever a relevant etcd key was changed
func (w *Watcher) Run() {
	log.Println("start watcher for warp entries")
	warpChannel := make(chan *coreosclient.Response)

	for _, source := range w.configuration.Sources {
		go func(source config.Source) {
			for {
				w.execute()
				w.registryToWatch.Watch(source.Path, true, warpChannel, w.doneChannel)
			}
		}(source)
	}

	for {
		select {
		case <-w.doneChannel:
			return
		case <-warpChannel:
			w.execute()
		}
	}
}

func (w *Watcher) execute() {
	categories, err := w.configReader.readFromConfig(w.configuration)
	if err != nil {
		log.Println("Error during read:", err)
		return
	}
	log.Printf("all found Categories: %v", categories)
	err = w.jsonWriter(categories)
	if err != nil {
		log.Printf("failed to write warp menu as json: %v", err)
	}
}

func (w *Watcher) jsonWriter(data interface{}) error {
	configmap, err := w.getMenuConfigMap()
	if err != nil {
		return fmt.Errorf("failed to get menu json config map: %w", err)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal warp data: %w", err)
	}

	configmap.Data["menu.json"] = string(jsonData)

	err = w.k8sClient.Update(context.TODO(), configmap)
	if err != nil {
		return fmt.Errorf("failed to update menu json config map: %w", err)
	}

	return nil
}

func (w *Watcher) getMenuConfigMap() (*corev1.ConfigMap, error) {
	configmap := &corev1.ConfigMap{}
	objectKey := client.ObjectKey{Name: config.MenuConfigMap, Namespace: w.namespace}
	err := w.k8sClient.Get(context.Background(), objectKey, configmap)

	return configmap, err
}
