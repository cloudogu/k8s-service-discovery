package controllers

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// serviceReconciler watches every Service object in the cluster and creates ingress objects accordingly.
type serviceReconciler struct {
	ingressUpdater     IngressUpdater
	exposedPortUpdater ExposedPortUpdater
	client             client.Client
}

// NewServiceReconciler creates a new service reconciler.
func NewServiceReconciler(client client.Client, ingressUpdater IngressUpdater, exposedPortUpdater ExposedPortUpdater) *serviceReconciler {
	return &serviceReconciler{
		client:             client,
		ingressUpdater:     ingressUpdater,
		exposedPortUpdater: exposedPortUpdater,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The serviceReconciler is responsible to generate ingress objects for respective services containing the ces service
// discovery annotation.
func (r *serviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	service, err := r.getService(ctx, req)
	if err != nil && !errors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("failed to get service %s: %s", req.NamespacedName, err))
		return ctrl.Result{}, err
	}

	if errors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("service %s not found", req.NamespacedName))
		logger.Info("remove exposed ports")
		removeErr := r.exposedPortUpdater.RemoveExposedPorts(ctx, req.Name)
		if removeErr != nil {
			logger.Error(err, fmt.Sprintf("failed to remove exposed ports for service %s", req.NamespacedName))
		}
		return ctrl.Result{}, nil
	}

	logger.Info(fmt.Sprintf("Found service [%s]", service.Name))

	err = r.ingressUpdater.UpsertIngressForService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of service [%s]: %w", service.Name, err)
	}

	err = r.exposedPortUpdater.UpsertCesLoadbalancerService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update exposed ports for service [%s]: %w", service.Name, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *serviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		// Only reconcile if the annotation changes.
		WithEventFilter(predicate.AnnotationChangedPredicate{}).
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
