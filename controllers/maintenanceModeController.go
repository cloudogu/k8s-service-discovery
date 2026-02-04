package controllers

import (
	"context"
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"strings"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	doguv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"

	"github.com/hashicorp/go-multierror"
	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// NewMaintenanceModeController creates a new maintenance mode updater.
func NewMaintenanceModeController(client k8sClient, namespace string, ingressUpdater IngressUpdater, recorder eventRecorder) *maintenanceModeController {
	rewriter := &defaultServiceRewriter{client: client, eventRecorder: recorder, namespace: namespace}

	return &maintenanceModeController{
		client:          client,
		namespace:       namespace,
		ingressUpdater:  ingressUpdater,
		eventRecorder:   recorder,
		serviceRewriter: rewriter,
	}
}

// maintenanceModeController is responsible to update all ingress objects according to the desired maintenance mode.
type maintenanceModeController struct {
	client          k8sClient
	namespace       string
	ingressUpdater  IngressUpdater
	eventRecorder   eventRecorder
	serviceRewriter serviceRewriter
}

func (mmu *maintenanceModeController) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	err := mmu.handleMaintenanceModeUpdate(ctx)
	if err != nil {
		ctrl.LoggerFrom(ctx).Error(err, "failed to handle maintenance update")
	}

	return reconcile.Result{}, err
}

func (mmu *maintenanceModeController) handleMaintenanceModeUpdate(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Maintenance mode key changed in registry. Refresh ingress objects accordingly...")

	isActive, err := util.GetMaintenanceModeActive(ctx, mmu.client, mmu.namespace)
	if err != nil {
		return err
	}

	err = mmu.setMaintenanceMode(ctx, isActive)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("Maintenance mode changed to %t.", isActive))
	return nil
}

func (mmu *maintenanceModeController) getAllServices(ctx context.Context) (v1ServiceList, error) {
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

func (mmu *maintenanceModeController) setMaintenanceMode(ctx context.Context, activate bool) error {
	verb := "deactivate"
	if activate {
		verb = "activate"
	}
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("%s maintenance mode...", cases.Title(language.English).String(verb)))

	serviceList, err := mmu.getAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed to %s maintenance mode: %w", verb, err)
	}

	for _, service := range serviceList {
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating ingress object [%s]", service.Name))
		err := mmu.ingressUpdater.UpsertIngressForService(ctx, service)
		if err != nil {
			return fmt.Errorf("failed to %s maintenance mode: %w", verb, err)
		}
	}

	err = mmu.serviceRewriter.rewrite(ctx, serviceList, activate)
	if err != nil {
		return fmt.Errorf("failed to rewrite services on %s maintenance mode: %w", verb, err)
	}

	return nil
}

// SetupWithManager sets up the maintenance configmap controller with the Manager.
// The controller watches for changes to the maintenance configmap.
func (mmu *maintenanceModeController) SetupWithManager(mgr k8sManager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}, builder.WithPredicates(maintenancePredicate())).
		Complete(mmu)
}

func getMaintenanceConfig(object client.Object) *v1.ConfigMap {
	configMap, ok := object.(*v1.ConfigMap)
	if !ok {
		return nil
	}

	if configMap.Name != util.MaintenanceConfigMapName {
		return nil
	}

	return configMap
}

func maintenancePredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return getMaintenanceConfig(e.Object) != nil
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			configOld := getMaintenanceConfig(e.ObjectOld)
			configNew := getMaintenanceConfig(e.ObjectNew)

			if configOld == nil && configNew == nil {
				return false
			}

			if configOld == nil || configNew == nil {
				return true
			}

			return util.IsMaintenanceModeActive(configOld) !=
				util.IsMaintenanceModeActive(configNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return getMaintenanceConfig(e.Object) != nil
		},
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			return getMaintenanceConfig(e.Object) != nil
		},
	}
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
