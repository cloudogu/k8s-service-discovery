package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/warp"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// warpMenuCreator used to create warp menu
type warpMenuCreator struct {
	client              client.Client
	doguVersionRegistry warp.DoguVersionRegistry
	localDoguRepo       warp.LocalDoguRepo
	namespace           string
	eventRecorder       eventRecorder
	globalConfig        warp.GlobalConfigRepository
}

// NewWarpMenuCreator initialises a creator object to start the warp menu creation
func NewWarpMenuCreator(client client.Client, doguVersionRegistry warp.DoguVersionRegistry, localDoguRepo warp.LocalDoguRepo, namespace string, recorder eventRecorder, globalConfig warp.GlobalConfigRepository) *warpMenuCreator {
	return &warpMenuCreator{
		client:              client,
		doguVersionRegistry: doguVersionRegistry,
		localDoguRepo:       localDoguRepo,
		namespace:           namespace,
		eventRecorder:       recorder,
		globalConfig:        globalConfig,
	}
}

// Start starts the runnable.
func (wmc warpMenuCreator) Start(ctx context.Context) error {
	return wmc.CreateWarpMenu(ctx)
}

// CreateWarpMenu reads the warp configuration and starts watchers to refresh the menu.json configmap
// in background.
func (wmc warpMenuCreator) CreateWarpMenu(ctx context.Context) error {
	warpWatcher, err := warp.NewWatcher(ctx, wmc.client, wmc.doguVersionRegistry, wmc.localDoguRepo, wmc.namespace, wmc.eventRecorder, wmc.globalConfig)
	if err != nil {
		return fmt.Errorf("failed to create warp menu watcher: %w", err)
	}

	return warpWatcher.Run(ctx)
}
