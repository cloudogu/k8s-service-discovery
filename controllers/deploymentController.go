package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// deploymentReconciler watches every Deployment object in the cluster and creates ingress objects when the ready state
// of a dogu changes between ready <-> not ready.
type deploymentReconciler struct {
	updater IngressUpdater
	client  client.Client
}

// NewDeploymentReconciler creates a new deployment reconciler.
func NewDeploymentReconciler(client client.Client, updater IngressUpdater) *deploymentReconciler {
	return &deploymentReconciler{
		client:  client,
		updater: updater,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The deploymentReconciler is responsible to regenerate ingress objects for respective dogus containing the ces service
// discovery annotation when their state switches between ready <-> not ready.
func (r *deploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	deployment, err := r.getDeployment(ctx, req)
	if err != nil {
		logger.Info(fmt.Sprintf("failed to get deployment %s: %s", req.NamespacedName, err))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !util.HasDoguLabel(deployment) {
		// ignore non dogu deployments
		return ctrl.Result{}, nil
	}
	logger.Info(fmt.Sprintf("Found dogu deployment: [%s]", deployment.Name))

	doguService, err := r.getService(ctx, req)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to find service for deployment [%s]: %w", deployment.Name, err)
	}

	err = r.updater.UpsertIngressForService(ctx, doguService)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of service [%s]: %w", doguService.Name, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *deploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Deployment{}).
		Complete(r)
}

func (r *deploymentReconciler) getDeployment(ctx context.Context, req ctrl.Request) (*v1.Deployment, error) {
	deployment := &v1.Deployment{}
	err := r.client.Get(ctx, req.NamespacedName, deployment)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	return deployment, nil
}

func (r *deploymentReconciler) getService(ctx context.Context, req ctrl.Request) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := r.client.Get(ctx, req.NamespacedName, service)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	return service, nil
}
