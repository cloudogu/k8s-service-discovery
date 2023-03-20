package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// warpMenuCreator used to create warp menu
type warpMenuCreator struct {
	client        client.Client
	registry      registry.Registry
	namespace     string
	eventRecorder record.EventRecorder
}

// NewWarpMenuCreator initialises a creator object to start the warp menu creation
func NewWarpMenuCreator(client client.Client, registry registry.Registry, namespace string, recorder record.EventRecorder) *warpMenuCreator {
	return &warpMenuCreator{
		client:        client,
		registry:      registry,
		namespace:     namespace,
		eventRecorder: recorder,
	}
}

// Start starts the runnable.
func (wmc warpMenuCreator) Start(ctx context.Context) error {
	return wmc.CreateWarpMenu(ctx)
}

// CreateWarpMenu reads the warp configuration and starts watchers to refresh the menu.json configmap
// in background.
func (wmc warpMenuCreator) CreateWarpMenu(ctx context.Context) error {
	warpWatcher, err := warp.NewWatcher(ctx, wmc.client, wmc.registry, wmc.namespace, wmc.eventRecorder)
	if err != nil {
		return fmt.Errorf("failed to create warp menu watcher: %w", err)
	}

	return warpWatcher.Run(ctx)
}
