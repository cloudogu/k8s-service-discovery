package controllers

import (
	"context"
	"fmt"

	etcdclient "go.etcd.io/etcd/client/v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/cesregistry"
)

const (
	maintenanceModeWatchKey  = "/config/_global/maintenance"
	maintenanceModeGlobalKey = "maintenance"
)

const (
	maintenanceChangeEventReason = "Maintenance"
)

// maintenanceModeUpdater is responsible to update all ingress objects according to the desired maintenance mode.
type maintenanceModeUpdater struct {
	client         client.Client
	namespace      string
	registry       registry.Registry
	ingressUpdater IngressUpdater
	eventRecorder  record.EventRecorder
}

// NewMaintenanceModeUpdater creates a new maintenance mode updater.
func NewMaintenanceModeUpdater(client client.Client, namespace string, ingressUpdater IngressUpdater, recorder record.EventRecorder) (*maintenanceModeUpdater, error) {
	reg, err := cesregistry.Create(namespace)
	if err != nil {
		return nil, err
	}

	return &maintenanceModeUpdater{
		client:         client,
		namespace:      namespace,
		registry:       reg,
		ingressUpdater: ingressUpdater,
		eventRecorder:  recorder,
	}, nil
}

// Start starts the update process. This update process runs indefinitely and is designed to be started as goroutine.
func (scu maintenanceModeUpdater) Start(ctx context.Context) error {
	log.FromContext(ctx).Info("Starting maintenance mode watcher...")
	return scu.startEtcdWatch(ctx, scu.registry.RootConfig())
}

func (scu *maintenanceModeUpdater) startEtcdWatch(ctx context.Context, reg registry.WatchConfigurationContext) error {
	log.FromContext(ctx).Info("Start etcd watcher on maintenance key")

	warpChannel := make(chan *etcdclient.Response)
	go func() {
		log.FromContext(ctx).Info("Start etcd watcher for maintenance key")
		reg.Watch(ctx, maintenanceModeWatchKey, true, warpChannel)
		log.FromContext(ctx).Info("Stop etcd watcher for maintenance key")
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-warpChannel:
			err := scu.handleMaintenanceModeUpdate(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (scu *maintenanceModeUpdater) handleMaintenanceModeUpdate(ctx context.Context) error {
	log.FromContext(ctx).Info("Maintenance mode key changed in registry. Refresh ingress objects accordingly...")

	isActive, err := isMaintenanceModeActive(scu.registry)
	if err != nil {
		return err
	}

	if isActive {
		err := scu.activateMaintenanceMode(ctx)
		if err != nil {
			return err
		}
	} else {
		err := scu.deactivateMaintenanceMode(ctx)
		if err != nil {
			return err
		}
	}

	err = scu.restartStaticNginxPod(ctx)
	if err != nil {
		return err
	}

	deployment := &appsv1.Deployment{}
	err = scu.client.Get(ctx, types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: scu.namespace}, deployment)
	if err != nil {
		return fmt.Errorf("maintenance mode: failed to get deployment [%s]: %w", "k8s-service-discovery-controller-manager", err)
	}
	scu.eventRecorder.Eventf(deployment, v1.EventTypeNormal, maintenanceChangeEventReason, "Maintenance mode changed to %t.", isActive)

	return nil
}

func (scu *maintenanceModeUpdater) restartStaticNginxPod(ctx context.Context) error {
	podList := &v1.PodList{}
	staticNginxRequirement, _ := labels.NewRequirement("dogu.name", selection.Equals, []string{"nginx-static"})
	err := scu.client.List(ctx, podList, &client.ListOptions{Namespace: scu.namespace, LabelSelector: labels.NewSelector().Add(*staticNginxRequirement)})
	if err != nil {
		return fmt.Errorf("failed to list [%s] pods: %w", "nginx-static", err)
	}

	for _, pod := range podList.Items {
		err := scu.client.Delete(ctx, &pod)
		if err != nil {
			return fmt.Errorf("failed to delete pod [%s]: %w", pod.Name, err)
		}
	}

	return nil
}

func (scu *maintenanceModeUpdater) getAllServices(ctx context.Context) (*v1.ServiceList, error) {
	serviceList := &v1.ServiceList{}
	err := scu.client.List(ctx, serviceList, &client.ListOptions{Namespace: scu.namespace})
	if err != nil {
		return &v1.ServiceList{}, fmt.Errorf("failed to get list of all services in namespace [%s]: %w", scu.namespace, err)
	}

	return serviceList, nil
}

func (scu *maintenanceModeUpdater) deactivateMaintenanceMode(ctx context.Context) error {
	log.FromContext(ctx).Info("Deactivate maintenance mode...")

	serviceList, err := scu.getAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to deactivate maintenance mode: %w", err)
	}

	for _, service := range serviceList.Items {
		log.FromContext(ctx).Info(fmt.Sprintf("Updating ingress object [%s]", service.Name))
		err := scu.ingressUpdater.UpsertIngressForService(ctx, &service)
		if err != nil {
			return err
		}
	}

	return nil
}

func (scu *maintenanceModeUpdater) activateMaintenanceMode(ctx context.Context) error {
	log.FromContext(ctx).Info("Activating maintenance mode...")

	serviceList, err := scu.getAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to activate maintenance mode: %w", err)
	}

	for _, service := range serviceList.Items {
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating ingress object [%s]", service.Name))
		err := scu.ingressUpdater.UpsertIngressForService(ctx, &service)
		if err != nil {
			return err
		}
	}

	return nil
}
