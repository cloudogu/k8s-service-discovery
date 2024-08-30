package warp

import (
	"context"
	"fmt"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-registry-lib/repository"
	appsv1 "k8s.io/api/apps/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"

	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
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
	configuration    *config.Configuration
	registryToWatch  DoguVersionRegistry
	globalConfigRepo GlobalConfigRepository
	k8sClient        client.Client
	ConfigReader     Reader
	namespace        string
	eventRecorder    eventRecorder
}

// NewWatcher creates a new Watcher instance to build the warp menu
func NewWatcher(ctx context.Context, k8sClient client.Client, doguVersionRegistry DoguVersionRegistry, localDoguRepo LocalDoguRepo, namespace string, recorder eventRecorder, globalConfigRepo GlobalConfigRepository) (*Watcher, error) {
	warpConfig, err := config.ReadConfiguration(ctx, k8sClient, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to Read configuration: %w", err)
	}

	reader := &ConfigReader{
		globalConfigRepo:    globalConfigRepo,
		doguVersionRegistry: doguVersionRegistry,
		localDoguRepo:       localDoguRepo,
		configuration:       warpConfig,
		doguConverter:       &types.DoguConverter{},
		externalConverter:   &types.ExternalConverter{},
	}

	return &Watcher{
		configuration:    warpConfig,
		registryToWatch:  doguVersionRegistry,
		k8sClient:        k8sClient,
		namespace:        namespace,
		ConfigReader:     reader,
		eventRecorder:    recorder,
		globalConfigRepo: globalConfigRepo,
	}, nil
}

// Run creates the warp menu and update the menu whenever a relevant configuration key was changed
func (w *Watcher) Run(ctx context.Context) error {
	// trigger the warp-menu creation once on startup
	err := w.execute(ctx)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "error creating warp-menu")
	}

	for _, source := range w.configuration.Sources {
		if strings.HasPrefix(source.Path, "/dogu") || strings.HasPrefix(source.Path, "dogu") {
			w.startVersionRegistryWatch(ctx, source.Path)
			continue
		}

		if strings.HasPrefix(source.Path, "/config/_global") || strings.HasPrefix(source.Path, "config/_global") {
			w.startGlobalConfigWatch(ctx, source.Path)
			continue
		}
	}

	return nil
}

func (w *Watcher) startGlobalConfigWatch(ctx context.Context, sourcePath string) {
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("start global config watcher for source [%s]", sourcePath))
	filter := libconfig.KeyFilter(libconfig.Key(sourcePath))
	globalConfigWatchChannel, err := w.globalConfigRepo.Watch(ctx, filter)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to create global config watch for path %q", sourcePath)
		return
	}

	go func() {
		w.handleGlobalConfigUpdates(ctx, globalConfigWatchChannel)
	}()
}

func (w *Watcher) handleGlobalConfigUpdates(ctx context.Context, globalConfigWatchChannel <-chan repository.GlobalConfigWatchResult) {
	for {
		select {
		case <-ctx.Done():
			ctrl.LoggerFrom(ctx).Info("context done - stop global config watch for warp generation")
			return
		case result, open := <-globalConfigWatchChannel:
			if !open {
				ctrl.LoggerFrom(ctx).Info("global config watch channel canceled - stop watch for warp generation")
				return
			}
			if result.Err != nil {
				ctrl.LoggerFrom(ctx).Error(result.Err, "global config watch channel error for warp generation")
				continue
			}
			// Trigger refresh. Content of the result is not needed
			err := w.execute(ctx)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to update entries from global config in warp menu")
			}
		}
	}
}

func (w *Watcher) startVersionRegistryWatch(ctx context.Context, sourcePath string) {
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("start version registry watcher for source [%s]", sourcePath))
	versionChannel, doguVersionWatchError := w.registryToWatch.WatchAllCurrent(ctx)
	if doguVersionWatchError != nil {
		ctrl.LoggerFrom(ctx).Error(doguVersionWatchError, "failed to create dogu version registry watch")
		return
	}

	go func() {
		w.handleDoguVersionUpdates(ctx, versionChannel.ResultChan)
	}()

}

func (w *Watcher) handleDoguVersionUpdates(ctx context.Context, versionChannel <-chan dogu.CurrentVersionsWatchResult) {
	for {
		select {
		case <-ctx.Done():
			ctrl.LoggerFrom(ctx).Info("context done - stop dogu version registry watch for warp generation")
			return
		case result, open := <-versionChannel:
			if !open {
				ctrl.LoggerFrom(ctx).Info("dogu version watch channel canceled - stop watch")
				return
			}
			if result.Err != nil {
				ctrl.LoggerFrom(ctx).Error(result.Err, "dogu version watch channel error")
				continue
			}
			// Trigger refresh. Content of the result is not needed
			err := w.execute(ctx)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to update dogus in warp menu")
			}
		}
	}
}

func (w *Watcher) execute(ctx context.Context) error {
	deployment := &appsv1.Deployment{}
	err := w.k8sClient.Get(ctx, types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: w.namespace}, deployment)
	if err != nil {
		return fmt.Errorf("warp update: failed to get deployment [%s]: %w", "k8s-service-discovery-controller-manager", err)
	}

	categories, err := w.ConfigReader.Read(ctx, w.configuration)
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
