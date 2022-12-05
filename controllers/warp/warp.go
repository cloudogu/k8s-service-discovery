package warp

import (
	"context"
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	etcdclient "go.etcd.io/etcd/client/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	warpMenuUpdateEventReason        = "WarpMenu"
	errorOnWarpMenuUpdateEventReason = "ErrUpdateWarpMenu"
)

// Watcher is used to watch a registry and for every change he reads from the registry a specific config path,
// build warp menu categories and writes them to a configmap.
type Watcher struct {
	configuration   *config.Configuration
	registryToWatch registry.WatchConfigurationContext
	k8sClient       client.Client
	ConfigReader    Reader
	namespace       string
	eventRecorder   record.EventRecorder
}

// Reader is used to fetch warp categories with a configuration
type Reader interface {
	Read(configuration *config.Configuration) (types.Categories, error)
}

// NewWatcher creates a new Watcher instance to build the warp menu
func NewWatcher(ctx context.Context, k8sClient client.Client, registry registry.Registry, namespace string, recorder record.EventRecorder) (*Watcher, error) {
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
		eventRecorder:   recorder,
	}, nil
}

// Run creates the warp menu and update the menu whenever a relevant etcd key was changed
func (w *Watcher) Run(ctx context.Context) {
	warpChannel := make(chan *etcdclient.Response)

	for _, source := range w.configuration.Sources {
		go func(source config.Source) {
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
	deployment := &appsv1.Deployment{}
	err := w.k8sClient.Get(context.Background(), types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: w.namespace}, deployment)
	if err != nil {
		ctrl.Log.Error(err, "warp update: failed to get deployment [%s]", "k8s-service-discovery-controller-manager")
		return
	}

	categories, err := w.ConfigReader.Read(w.configuration)
	if err != nil {
		w.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Updating warp menu failed: %w", err)
		ctrl.Log.Info("Error during Read:", err)
		return
	}
	ctrl.Log.Info(fmt.Sprintf("All found Categories: %v", categories))
	err = w.jsonWriter(categories)
	if err != nil {
		w.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Updating warp menu failed: %w", err)
		ctrl.Log.Info(fmt.Sprintf("failed to write warp menu as json: %s", err.Error()))
		return
	}
	w.eventRecorder.Event(deployment, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")
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

	err = w.k8sClient.Update(context.Background(), configmap)
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
