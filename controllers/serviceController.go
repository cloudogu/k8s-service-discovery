package controllers

import (
	"context"
	"fmt"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-registry-lib/repository"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ServiceReconciler watches Dogu Services, maps each one to a domain Exposition
// and delegates processing to an ExpositionService. It also re-enqueues Services
// on Dogu health-condition changes and on maintenance ConfigMap updates.
type ServiceReconciler struct {
	Client            client.Client
	ExpositionService ExpositionService
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For a Dogu Service it builds an Exposition from the service's annotations
// and forwards it to the ExpositionService for processing.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	doguService := &corev1.Service{}
	if err := r.Client.Get(ctx, req.NamespacedName, doguService); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get dogu service: %w", err)
	}

	exposition, err := mapServiceToExposition(doguService, r.Client.Scheme())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to map dogu service to exposition: %w", err)
	}

	if err := r.ExpositionService.ProcessExposition(ctx, exposition); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to process exposition from dogu service: %w", err)
	}

	logger.Info("Successfully processed exposition from dogu service.", "service", doguService.Name)

	return ctrl.Result{}, nil
}

// SetupWithManager registers the reconciler with the Manager. It watches Dogu
// Services (filtered to annotation changes), Dogu CRs (filtered to healthy
// condition changes), and the maintenance ConfigMap, plus any resource types
// reported by ExpositionService.GetOwnableTypes() as Owns sources.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctrlBuilder := ctrl.NewControllerManagedBy(mgr).
		For(
			&corev1.Service{},
			// only reconcile Services that are Dogu Services and whose annotations changed
			builder.WithPredicates(predicate.And(
				doguServicePredicate(),
				predicate.AnnotationChangedPredicate{},
			)),
		).
		Watches(
			&doguv2.Dogu{},
			handler.EnqueueRequestsFromMapFunc(r.mapRequestsFromDoguCR),
			builder.WithPredicates(doguHealthyConditionChangedPredicate()),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.mapRequestsFromMaintenanceConfigMap),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		)

	for _, res := range r.ExpositionService.GetOwnableTypes() {
		ctrlBuilder.Owns(res)
	}

	return ctrlBuilder.Complete(r)
}

// doguHealthyConditionChangedPredicate triggers reconciliation only when the
// Dogu's healthy status condition is added, removed, or changes value.
func doguHealthyConditionChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldDogu, okOld := e.ObjectOld.(*doguv2.Dogu)
			newDogu, okNew := e.ObjectNew.(*doguv2.Dogu)

			if !okOld || !okNew {
				return false
			}

			oldHealthy := apimeta.FindStatusCondition(oldDogu.Status.Conditions, doguv2.ConditionHealthy)
			newHealthy := apimeta.FindStatusCondition(newDogu.Status.Conditions, doguv2.ConditionHealthy)

			// Condition has been added or removed
			if (oldHealthy == nil) != (newHealthy == nil) {
				return true
			}

			// Both nil: nothing to react to
			if oldHealthy == nil {
				return false
			}

			// Both set: react only when the status value changed
			return oldHealthy.Status != newHealthy.Status
		},
		CreateFunc: func(e event.CreateEvent) bool {
			// Ignore: when a DoguCR has been created, wait for the health condition to be written
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Ignore: the Service has the Dogu as owner and will be garbage-collected
			return false
		},
	}
}

// mapRequestsFromDoguCR enqueues the Dogu's Service for reconciliation. The
// Service shares the Dogu's name by convention.
func (r *ServiceReconciler) mapRequestsFromDoguCR(_ context.Context, obj client.Object) []reconcile.Request {
	dogu, ok := obj.(*doguv2.Dogu)
	if !ok {
		return nil
	}

	return []reconcile.Request{
		{
			NamespacedName: client.ObjectKey{
				Name:      dogu.Name,
				Namespace: dogu.Namespace,
			},
		},
	}
}

// mapRequestsFromMaintenanceConfigMap enqueues every Dogu Service in the
// ConfigMap's namespace whenever the maintenance ConfigMap changes.
func (r *ServiceReconciler) mapRequestsFromMaintenanceConfigMap(ctx context.Context, object client.Object) []reconcile.Request {
	logger := log.FromContext(ctx).WithName("map service requests from maintenance config map")

	if object.GetName() != repository.MaintenanceConfigMapName {
		return nil
	}

	doguLabelReq, err := labels.NewRequirement(doguv2.DoguLabelName, selection.Exists, nil)
	if err != nil {
		logger.Error(err, "failed to create selector for dogu label")
		return nil
	}

	var doguServices corev1.ServiceList
	doguLabelSelector := labels.NewSelector().Add(*doguLabelReq)
	err = r.Client.List(ctx, &doguServices,
		&client.ListOptions{
			Namespace:     object.GetNamespace(),
			LabelSelector: doguLabelSelector,
		})
	if err != nil {
		logger.Error(err, "failed to list dogu services")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(doguServices.Items))
	for _, service := range doguServices.Items {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: service.Namespace, Name: service.Name}})
	}

	return requests
}
