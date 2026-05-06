package controllers

import (
	"context"
	"errors"
	"fmt"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cloudogu/k8s-registry-lib/repository"
)

type v1ServiceList []*v1.Service

type k8sClient interface {
	client.Client
}

// NewMaintenanceModeController creates a new maintenance mode updater.
func NewMaintenanceModeController(client k8sClient, namespace string, ingressUpdater IngressUpdater, maintenanceAdapter MaintenanceAdapter, recorder eventRecorder) *maintenanceModeController {
	return &maintenanceModeController{
		client:             client,
		namespace:          namespace,
		ingressUpdater:     ingressUpdater,
		eventRecorder:      recorder,
		maintenanceAdapter: maintenanceAdapter,
	}
}

// maintenanceModeController is responsible to update all ingress objects according to the desired maintenance mode.
type maintenanceModeController struct {
	client             k8sClient
	namespace          string
	ingressUpdater     IngressUpdater
	eventRecorder      eventRecorder
	maintenanceAdapter MaintenanceAdapter
}

// TODO maybe do this in the service and exposition controllers?

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

	_, isActive, err := mmu.maintenanceAdapter.GetStatus(ctx)
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

func (mmu *maintenanceModeController) getAllExpositions(ctx context.Context) ([]*expositionv1.Exposition, error) {
	expositionList := &expositionv1.ExpositionList{}
	err := mmu.client.List(ctx, expositionList, &client.ListOptions{Namespace: mmu.namespace})
	if err != nil {
		return nil, fmt.Errorf("failed to get list of all services in namespace [%s]: %w", mmu.namespace, err)
	}

	var modifiableExpositionList []*expositionv1.Exposition
	for _, exp := range expositionList.Items {
		copyExp := exp
		modifiableExpositionList = append(modifiableExpositionList, &copyExp)
	}

	return modifiableExpositionList, nil
}

func (mmu *maintenanceModeController) setMaintenanceMode(ctx context.Context, activate bool) error {
	verb := "deactivate"
	if activate {
		verb = "activate"
	}
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("%s maintenance mode...", cases.Title(language.English).String(verb)))

	serviceList, err := mmu.getAllServices(ctx)
	if err != nil {
		return fmt.Errorf("failed get services to %s maintenance mode: %w", verb, err)
	}

	var errs []error
	for _, service := range serviceList {
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating ingress objects for service [%s]", service.Name))
		err := mmu.ingressUpdater.UpsertForService(ctx, service)
		if err != nil {
			errs = append(errs, err)
		}
	}

	expositionList, err := mmu.getAllExpositions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get expositions to %s maintenance mode: %w", verb, err)
	}

	for _, exposition := range expositionList {
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating ingress objects for exposition [%s]", exposition.Name))
		err := mmu.ingressUpdater.UpsertForExposition(ctx, exposition)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to %s maintenance mode: %w", verb, errors.Join(errs...))
	}

	return nil
}

// SetupWithManager sets up the maintenance configmap controller with the Manager.
// The controller watches for changes to the maintenance configmap.
func (mmu *maintenanceModeController) SetupWithManager(mgr k8sManager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}, builder.WithPredicates(maintenancePredicate())).
		Named("maintenance").
		Complete(mmu)
}

func maintenancePredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == repository.MaintenanceConfigMapName
	})
}
