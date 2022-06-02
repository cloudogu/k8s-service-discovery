package warp

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	coreosclient "github.com/coreos/etcd/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Watcher is used to watch a registry and for every change he reads from the registry a specific config path,
// build warp menu categories and writes them to a configmap.
type Watcher struct {
	configuration   *config.Configuration
	registryToWatch registry.WatchConfigurationContext
	k8sClient       client.Client
	ConfigReader    Reader
	namespace       string
}

// Reader is used to fetch warp categories with a configuration
type Reader interface {
	Read(configuration *config.Configuration) (types.Categories, error)
}

// NewWatcher creates a new Watcher instance to build the warp menu
func NewWatcher(ctx context.Context, k8sClient client.Client, registry registry.Registry, namespace string) (*Watcher, error) {
	warpConfig, err := config.ReadConfiguration(ctx, k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to Read configuration: %w", err)
	}

	reader := &ConfigReader{
		registry:          registry.RootConfig(),
		configuration:     warpConfig,
		doguConverter:     &types.DoguConverter{},
		externalConverter: &types.ExternalConverter{},
	}

	return &Watcher{
		configuration:   warpConfig,
		registryToWatch: registry.RootConfig(),
		k8sClient:       k8sClient,
		namespace:       namespace,
		ConfigReader:    reader,
	}, nil
}

// Run creates the warp menu and update the menu whenever a relevant etcd key was changed
func (w *Watcher) Run(ctx context.Context) {
	warpChannel := make(chan *coreosclient.Response)

	for _, source := range w.configuration.Sources {
		go func(source config.Source) {
			w.execute()
			ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("start etcd watcher for source [%s]", source))
			w.registryToWatch.Watch(ctx, source.Path, true, warpChannel)
			ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("stop etcd watcher for source [%s]", source))
		}(source)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-warpChannel:
			w.execute()
		}
	}
}

func (w *Watcher) execute() {
	categories, err := w.ConfigReader.Read(w.configuration)
	if err != nil {
		ctrl.Log.Info("Error during Read:", err)
		return
	}
	ctrl.Log.Info("all found Categories: %v", categories)
	err = w.jsonWriter(categories)
	if err != nil {
		ctrl.Log.Info("failed to write warp menu as json: %v", err)
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
