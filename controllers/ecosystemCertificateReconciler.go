package controllers

import (
	"context"

	v1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	certificateSecretName = "ecosystem-certificate"
)

// ecosystemCertificateReconciler watches the ecosystem-certificate secret in the cluster and synchronizes it to the global config.
type ecosystemCertificateReconciler struct {
	certSync certificateSynchronizer
}

// NewEcosystemCertificateReconciler creates a new reconciler for the ecosystem-certificate secret.
func NewEcosystemCertificateReconciler(certSync certificateSynchronizer) *ecosystemCertificateReconciler {
	return &ecosystemCertificateReconciler{
		certSync: certSync,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The ecosystemCertificateReconciler is responsible to write changes of the ecosystem certificate to the global config.
func (r *ecosystemCertificateReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, r.certSync.Synchronize(ctx)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ecosystemCertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Secret{}).
		WithEventFilter(ecosystemCertificatePredicate()).
		Complete(r)
}

func ecosystemCertificatePredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return e.Object.GetName() == certificateSecretName
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return e.Object.GetName() == certificateSecretName
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			return e.ObjectOld.GetName() == certificateSecretName
		},
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			return e.Object.GetName() == certificateSecretName
		},
	}
}
