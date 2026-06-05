package controllers

import (
	"context"
	"fmt"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ExpositionReconciler watches every Exposition object in the cluster and creates ingress objects accordingly.
type ExpositionReconciler struct {
	IngressUpdater       IngressUpdater
	NetworkPolicyUpdater NetworkPolicyUpdater
	Client               client.Client
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The expositionReconciler is responsible for generating ingress objects for respective exposition objects.
func (r *ExpositionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	exposition, err := r.getExposition(ctx, req)
	if apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Info(fmt.Sprintf("failed to get exposition %s: %s", req.NamespacedName, err))
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("Reconciling exposition [%s]", exposition.Name))
	return r.handleUpsert(ctx, exposition)
}

func (r *ExpositionReconciler) handleUpsert(ctx context.Context, exposition *expositionv1.Exposition) (ctrl.Result, error) {
	err := r.IngressUpdater.UpsertForExposition(ctx, exposition)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of exposition [%s]: %w", exposition.Name, err)
	}

	err = r.NetworkPolicyUpdater.UpsertNetworkPoliciesForExposition(ctx, exposition)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update network policies for exposition [%s]: %w", exposition.Name, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ExpositionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&expositionv1.Exposition{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&networkingv1.Ingress{}).
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
			handler.EnqueueRequestsFromMapFunc(mapRequestsFromMaintenanceConfigMap(
				r.Client,
				func() client.ObjectList { return &expositionv1.ExpositionList{} },
				"exposition-maintenance",
			)),
			builder.WithPredicates(maintenanceConfigMapPredicate()),
		).
		Complete(r)
}

func mapRequestsFromDeployment(_ context.Context, object client.Object) []reconcile.Request {
	deployment, ok := object.(*appsv1.Deployment)
	if !ok || !util.HasDoguLabel(deployment) {
		return nil
	}

	return []reconcile.Request{{
		NamespacedName: client.ObjectKey{
			Namespace: deployment.Namespace,
			Name:      deployment.Labels[doguv2.DoguLabelName],
		},
	}}
}

func (r *ExpositionReconciler) getExposition(ctx context.Context, req ctrl.Request) (*expositionv1.Exposition, error) {
	exposition := &expositionv1.Exposition{}
	err := r.Client.Get(ctx, req.NamespacedName, exposition)
	if err != nil {
		return &expositionv1.Exposition{}, fmt.Errorf("failed to get exposition: %w", err)
	}

	return exposition, nil
}
