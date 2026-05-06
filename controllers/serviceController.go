package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// serviceReconciler watches every Service object in the cluster and creates ingress objects accordingly.
type serviceReconciler struct {
	ingressUpdater       IngressUpdater
	networkPolicyUpdater NetworkPolicyUpdater
	client               client.Client
}

// NewServiceReconciler creates a new service reconciler.
func NewServiceReconciler(client client.Client, ingressUpdater IngressUpdater, networkPolicyUpdater NetworkPolicyUpdater) *serviceReconciler {
	return &serviceReconciler{
		client:               client,
		ingressUpdater:       ingressUpdater,
		networkPolicyUpdater: networkPolicyUpdater,
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
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("failed to get service %s: %s", req.NamespacedName, err))
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Reconciling service [%s]", service.Name))
	return r.handleUpsert(ctx, service)
}

func (r *serviceReconciler) handleUpsert(ctx context.Context, service *corev1.Service) (ctrl.Result, error) {
	err := r.ingressUpdater.UpsertForService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of service [%s]: %w", service.Name, err)
	}

	err = r.networkPolicyUpdater.UpsertNetworkPoliciesForService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update network policies for service [%s]: %w", service.Name, err)
	}

	// TODO finalizer?

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
