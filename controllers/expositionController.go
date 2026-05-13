package controllers

import (
	"context"
	"fmt"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// expositionReconciler watches every Exposition object in the cluster and creates ingress objects accordingly.
type expositionReconciler struct {
	ingressUpdater       IngressUpdater
	networkPolicyUpdater NetworkPolicyUpdater
	client               client.Client
}

// NewExpositionReconciler creates a new exposition reconciler.
func NewExpositionReconciler(client client.Client, ingressUpdater IngressUpdater, networkPolicyUpdater NetworkPolicyUpdater) *expositionReconciler {
	return &expositionReconciler{
		client:               client,
		ingressUpdater:       ingressUpdater,
		networkPolicyUpdater: networkPolicyUpdater,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The expositionReconciler is responsible for generating ingress objects for respective exposition objects.
func (r *expositionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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

func (r *expositionReconciler) handleUpsert(ctx context.Context, exposition *expositionv1.Exposition) (ctrl.Result, error) {
	err := r.ingressUpdater.UpsertForExposition(ctx, exposition)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of exposition [%s]: %w", exposition.Name, err)
	}

	err = r.networkPolicyUpdater.UpsertNetworkPoliciesForExposition(ctx, exposition)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update network policies for exposition [%s]: %w", exposition.Name, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *expositionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&expositionv1.Exposition{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}

func (r *expositionReconciler) getExposition(ctx context.Context, req ctrl.Request) (*expositionv1.Exposition, error) {
	exposition := &expositionv1.Exposition{}
	err := r.client.Get(ctx, req.NamespacedName, exposition)
	if err != nil {
		return &expositionv1.Exposition{}, fmt.Errorf("failed to get exposition: %w", err)
	}

	return exposition, nil
}
