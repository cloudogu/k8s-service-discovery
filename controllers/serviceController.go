package controllers

import (
	"context"
	"fmt"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-registry-lib/repository"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ServiceReconciler watches every Service object in the cluster and creates ingress objects accordingly.
type ServiceReconciler struct {
	IngressUpdater       IngressUpdater
	NetworkPolicyUpdater NetworkPolicyUpdater
	Client               client.Client
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The serviceReconciler is responsible to generate ingress objects for respective services containing the ces service
// discovery annotation.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	service, err := r.getService(ctx, req)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Info(fmt.Sprintf("failed to get service %s: %s", req.NamespacedName, err))
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Reconciling service [%s]", service.Name))
	return r.handleUpsert(ctx, service)
}

func (r *ServiceReconciler) handleUpsert(ctx context.Context, service *corev1.Service) (ctrl.Result, error) {
	err := r.IngressUpdater.UpsertForService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of service [%s]: %w", service.Name, err)
	}

	err = r.NetworkPolicyUpdater.UpsertNetworkPoliciesForService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update network policies for service [%s]: %w", service.Name, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		// Only reconcile if the annotation changes.
		WithEventFilter(predicate.AnnotationChangedPredicate{}).
		Watches(
			&appsv1.Deployment{},
			handler.EnqueueRequestsFromMapFunc(mapRequestsFromDeployment),
			// only reconcile on status changes
			builder.WithPredicates(predicate.And(
				predicate.ResourceVersionChangedPredicate{},
				predicate.Not(predicate.GenerationChangedPredicate{}),
			)),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.mapRequestsFromMaintenanceConfigMap),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Complete(r)
}

func (r *ServiceReconciler) getService(ctx context.Context, req ctrl.Request) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := r.Client.Get(ctx, req.NamespacedName, service)
	if err != nil {
		return &corev1.Service{}, fmt.Errorf("failed to get service: %w", err)
	}

	return service, nil
}

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
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: service.Namespace, Name: service.Labels[doguv2.DoguLabelName]}})
	}

	return requests
}
