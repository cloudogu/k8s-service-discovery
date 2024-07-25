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
	eventRecorder   eventRecorder
}

type eventRecorder interface {
	record.EventRecorder
}

type cesRegistry interface {
	registry.Registry
}

type watchConfigurationContext interface {
	registry.WatchConfigurationContext
}

// Reader is used to fetch warp categories with a configuration
type Reader interface {
	Read(configuration *config.Configuration) (types.Categories, error)
}

// NewWatcher creates a new Watcher instance to build the warp menu
func NewWatcher(ctx context.Context, k8sClient client.Client, registry cesRegistry, namespace string, recorder eventRecorder) (*Watcher, error) {
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
func (w *Watcher) Run(ctx context.Context) error {
	// trigger the warp-menu creation once on startup
	err := w.execute(ctx)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "error creating warp-menu")
	}

	// start watches
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
			return nil
		case <-warpChannel:
			err := w.execute(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (w *Watcher) execute(ctx context.Context) error {
	deployment := &appsv1.Deployment{}
	//FIXME: do not hardcode deployment names
	//FIXME: Why is this even needed?
	discoveryName := fmt.Sprintf("%s-%s", w.namespace, "k8s-service-discovery-controller-manager")
	err := w.k8sClient.Get(ctx, types2.NamespacedName{Name: discoveryName, Namespace: w.namespace}, deployment)
	if err != nil {
		return fmt.Errorf("warp update: failed to get deployment [%s]: %w", discoveryName, err)
	}

	categories, err := w.ConfigReader.Read(w.configuration)
	if err != nil {
		w.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Updating warp menu failed: %w", err)
		return fmt.Errorf("error during read: %w", err)
	}
	ctrl.Log.Info(fmt.Sprintf("All found Categories: %v", categories))
	err = w.jsonWriter(ctx, categories)
	if err != nil {
		w.eventRecorder.Eventf(deployment, corev1.EventTypeWarning, errorOnWarpMenuUpdateEventReason, "Updating warp menu failed: %w", err)
		return fmt.Errorf("failed to write warp menu as json: %w", err)
	}
	w.eventRecorder.Event(deployment, corev1.EventTypeNormal, warpMenuUpdateEventReason, "Warp menu updated.")
	return nil
}

func (w *Watcher) jsonWriter(ctx context.Context, data interface{}) error {
	configmap, err := w.getMenuConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("failed to get menu json config map: %w", err)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal warp data: %w", err)
	}

	configmap.Data["menu.json"] = string(jsonData)
	err = w.k8sClient.Update(ctx, configmap)
	if err != nil {
		return fmt.Errorf("failed to update menu json config map: %w", err)
	}

	return nil
}

func (w *Watcher) getMenuConfigMap(ctx context.Context) (*corev1.ConfigMap, error) {
	configmap := &corev1.ConfigMap{}
	objectKey := client.ObjectKey{Name: config.MenuConfigMap, Namespace: w.namespace}
	err := w.k8sClient.Get(ctx, objectKey, configmap)

	return configmap, err
}
