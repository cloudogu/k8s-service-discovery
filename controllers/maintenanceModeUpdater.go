package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"strings"

	doguv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	maintenanceModeGlobalKey = "maintenance"
)

const (
	maintenanceChangeEventReason = "Maintenance"
)

const exposedServiceMaintenanceSelectorKey = "deactivatedDuringMaintenance"

type v1ServiceList []*v1.Service

type serviceRewriter interface {
	rewrite(ctx context.Context, serviceList v1ServiceList, activateMaintenanceMode bool) error
}

type k8sClient interface {
	client.Client
}

// maintenanceModeUpdater is responsible to update all ingress objects according to the desired maintenance mode.
type maintenanceModeUpdater struct {
	client           k8sClient
	namespace        string
	ingressUpdater   IngressUpdater
	eventRecorder    eventRecorder
	serviceRewriter  serviceRewriter
	globalConfigRepo GlobalConfigRepository
}

// NewMaintenanceModeUpdater creates a new maintenance mode updater.
func NewMaintenanceModeUpdater(client k8sClient, namespace string, ingressUpdater IngressUpdater, recorder eventRecorder, globalConfigRepo GlobalConfigRepository) (*maintenanceModeUpdater, error) {
	rewriter := &defaultServiceRewriter{client: client, eventRecorder: recorder, namespace: namespace}

	return &maintenanceModeUpdater{
		client:           client,
		namespace:        namespace,
		ingressUpdater:   ingressUpdater,
		eventRecorder:    recorder,
		serviceRewriter:  rewriter,
		globalConfigRepo: globalConfigRepo,
	}, nil
}

// Start starts the update process. This update process runs indefinitely and is designed to be started as goroutine.
func (mmu *maintenanceModeUpdater) Start(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Starting maintenance mode watcher...")
	return mmu.startGlobalConfigWatch(ctx)
}

func (mmu *maintenanceModeUpdater) startGlobalConfigWatch(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Start global config watcher on maintenance key")

	maintenanceWatchChannel, err := mmu.globalConfigRepo.Watch(ctx, config.KeyFilter(maintenanceModeGlobalKey))
	if err != nil {
		return fmt.Errorf("failed to start maintenance watch: %w", err)
	}

	go func() {
		mmu.startMaintenanceWatch(ctx, maintenanceWatchChannel)
	}()

	return nil
}

func (mmu *maintenanceModeUpdater) startMaintenanceWatch(ctx context.Context, maintenanceWatchChannel <-chan repository.GlobalConfigWatchResult) {
	for {
		select {
		case <-ctx.Done():
			ctrl.LoggerFrom(ctx).Info("context done - stop global config watcher for maintenance")
			return
		case result, open := <-maintenanceWatchChannel:
			if !open {
				ctrl.LoggerFrom(ctx).Info("maintenance watch channel canceled - stop watch")
				return
			}
			if result.Err != nil {
				ctrl.LoggerFrom(ctx).Error(result.Err, "maintenance watch channel error")
				continue
			}

			err := mmu.handleMaintenanceModeUpdate(ctx)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to handle maintenance update")
			}
		}
	}
}

func (mmu *maintenanceModeUpdater) handleMaintenanceModeUpdate(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Maintenance mode key changed in registry. Refresh ingress objects accordingly...")

	isActive, err := util.IsMaintenanceModeActive(ctx, mmu.globalConfigRepo)
	if err != nil {
		return err
	}

	if isActive {
		err := mmu.activateMaintenanceMode(ctx)
		if err != nil {
			return err
		}
	} else {
		err := mmu.deactivateMaintenanceMode(ctx)
		if err != nil {
			return err
		}
	}

	err = mmu.restartStaticNginxPod(ctx)
	if err != nil {
		return err
	}

	deployment := &appsv1.Deployment{}
	err = mmu.client.Get(ctx, types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: mmu.namespace}, deployment)
	if err != nil {
		return fmt.Errorf("maintenance mode: failed to get deployment [%s]: %w", "k8s-service-discovery-controller-manager", err)
	}
	mmu.eventRecorder.Eventf(deployment, v1.EventTypeNormal, maintenanceChangeEventReason, "Maintenance mode changed to %t.", isActive)

	return nil
}

func (mmu *maintenanceModeUpdater) restartStaticNginxPod(ctx context.Context) error {
	podList := &v1.PodList{}
	staticNginxRequirement, _ := labels.NewRequirement(doguv2.DoguLabelName, selection.Equals, []string{"nginx-static"})
	err := mmu.client.List(ctx, podList, &client.ListOptions{Namespace: mmu.namespace, LabelSelector: labels.NewSelector().Add(*staticNginxRequirement)})
	if err != nil {
		return fmt.Errorf("failed to list [%s] pods: %w", "nginx-static", err)
	}

	for _, pod := range podList.Items {
		err := mmu.client.Delete(ctx, &pod)
		if err != nil {
			return fmt.Errorf("failed to delete pod [%s]: %w", pod.Name, err)
		}
	}

	return nil
}

func (mmu *maintenanceModeUpdater) getAllServices(ctx context.Context) (v1ServiceList, error) {
	serviceList := &v1.ServiceList{}
	err := mmu.client.List(ctx, serviceList, &client.ListOptions{Namespace: mmu.namespace})
	if err != nil {
		return nil, fmt.Errorf("failed to get list of all services in namespace [%s]: %w", mmu.namespace, err)
	}

	var modifiableServiceList v1ServiceList
	for _, svc := range serviceList.Items {
		copySvc := svc
		modifiableServiceList = append(modifiableServiceList, &copySvc)
	}

	return modifiableServiceList, nil
}

func (mmu *maintenanceModeUpdater) deactivateMaintenanceMode(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Deactivate maintenance mode...")

	serviceList, err := mmu.getAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to deactivate maintenance mode: %w", err)
	}

	for _, service := range serviceList {
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating ingress object [%s]", service.Name))
		err := mmu.ingressUpdater.UpsertIngressForService(ctx, service)
		if err != nil {
			return err
		}
	}

	err = mmu.serviceRewriter.rewrite(ctx, serviceList, false)
	if err != nil {
		return fmt.Errorf("failed to rewrite services during maintenance mode deactivation: %w", err)
	}

	return nil
}

func (mmu *maintenanceModeUpdater) activateMaintenanceMode(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Activating maintenance mode...")

	serviceList, err := mmu.getAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to activate maintenance mode: %w", err)
	}

	for _, service := range serviceList {
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating ingress object [%s]", service.Name))
		err := mmu.ingressUpdater.UpsertIngressForService(ctx, service)
		if err != nil {
			return err
		}
	}

	err = mmu.serviceRewriter.rewrite(ctx, serviceList, true)
	if err != nil {
		return fmt.Errorf("failed to rewrite services during maintenance mode activation: %w", err)
	}

	return err
}

type defaultServiceRewriter struct {
	client        k8sClient
	eventRecorder eventRecorder
	namespace     string
}

func (sw *defaultServiceRewriter) rewrite(ctx context.Context, serviceList v1ServiceList, activateMaintenanceMode bool) error {
	var err error
	for _, service := range serviceList {
		rewriteErr := rewriteNonSimpleServiceRoute(ctx, sw.client, sw.eventRecorder, service, activateMaintenanceMode)
		if rewriteErr != nil {
			err = multierror.Append(err, rewriteErr)
		}
	}

	return err
}

func rewriteNonSimpleServiceRoute(ctx context.Context, cli k8sClient, recorder eventRecorder, service *v1.Service, rewriteToMaintenance bool) error {
	if service.Spec.Type == v1.ServiceTypeClusterIP {
		return nil
	}

	if service.Spec.Selector[doguv2.DoguLabelName] == "" {
		return nil
	}

	if isServiceNginxRelated(service) {
		return nil
	}

	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating service object [%s]", service.Name))

	var serviceEventMsg string
	if rewriteToMaintenance {
		serviceEventMsg = "Maintenance mode was activated, rewriting exposed service %s"
		service.Spec.Selector = map[string]string{doguv2.DoguLabelName: exposedServiceMaintenanceSelectorKey}
	} else {
		serviceEventMsg = "Maintenance mode was deactivated, restoring exposed service %s"
		service.Spec.Selector = map[string]string{doguv2.DoguLabelName: service.Labels[doguv2.DoguLabelName]}
	}
	recorder.Eventf(service, v1.EventTypeNormal, maintenanceChangeEventReason, serviceEventMsg, service.Name)

	err := cli.Update(ctx, service)
	if err != nil {
		return fmt.Errorf("could not rewrite service %s: %w", service.Name, err)
	}

	return nil
}

func isServiceNginxRelated(service *v1.Service) bool {
	return strings.HasPrefix(service.Spec.Selector[doguv2.DoguLabelName], "nginx-")
}
