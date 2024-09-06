package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// CesServiceAnnotation can be appended to service with information of ces services.
const CesServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-services"

// serviceReconciler watches every Service object in the cluster and creates ingress objects accordingly.
type serviceReconciler struct {
	updater IngressUpdater
	client  client.Client
}

// NewServiceReconciler creates a new service reconciler.
func NewServiceReconciler(client client.Client, updater IngressUpdater) *serviceReconciler {
	return &serviceReconciler{
		client:  client,
		updater: updater,
	}
}

// IngressUpdater is responsible to create and update the actual ingress objects in the cluster.
type IngressUpdater interface {
	// UpsertIngressForService creates or updates the ingress object of the given service.
	UpsertIngressForService(ctx context.Context, service *corev1.Service) error
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The serviceReconciler is responsible to generate ingress objects for respective services containing the ces service
// discovery annotation.
func (r *serviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	service, err := r.getService(ctx, req)
	if err != nil {
		logger.Info(fmt.Sprintf("failed to get service %s: %s", req.NamespacedName, err))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info(fmt.Sprintf("Found service [%s]", service.Name))

	err = r.updater.UpsertIngressForService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of service [%s]: %s", service.Name, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *serviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		WithEventFilter(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				// We don't need to listen to delete events
				return false
			},
		}).
		Complete(r)
}

func (r *serviceReconciler) getService(ctx context.Context, req ctrl.Request) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := r.client.Get(ctx, req.NamespacedName, service)
	if err != nil {
		return &corev1.Service{}, fmt.Errorf("failed to get service: %w", err)
	}

	return service, nil
}
