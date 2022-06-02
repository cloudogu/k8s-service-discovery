package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WarpMenuCreator used to create warp menu
type WarpMenuCreator struct {
	client    client.Client
	registry  registry.Registry
	namespace string
}

// NewWarpMenuCreator initialises a creator object to start the warp menu creation
func NewWarpMenuCreator(client client.Client, registry registry.Registry, namespace string) WarpMenuCreator {
	return WarpMenuCreator{
		client:    client,
		registry:  registry,
		namespace: namespace,
	}
}

// Start starts the runnable.
func (wmc WarpMenuCreator) Start(ctx context.Context) error {
	return wmc.CreateWarpMenu(ctx)
}

// CreateWarpMenu reads the warp configuration and starts watchers to refresh the menu.json configmap
// in background.
func (wmc WarpMenuCreator) CreateWarpMenu(ctx context.Context) error {
	warpWatcher, err := warp.NewWatcher(ctx, wmc.client, wmc.registry, wmc.namespace)
	if err != nil {
		return fmt.Errorf("failed to create warp menu watcher: %w", err)
	}

	warpWatcher.Run(ctx)

	return nil
}
