package controllers

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/errors"
	"github.com/cloudogu/retry-lib/retry"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	certificateSecretName  = "ecosystem-certificate"
	serverCertificateID    = "certificate/server.crt"
	serverCertificateKeyID = "certificate/server.key"
)

// ecosystemCertificateReconciler watches the ecosystem-certificate secret in the cluster and synchronizes it to the global config.
type ecosystemCertificateReconciler struct {
	secretInterface  SecretClient
	globalConfigRepo GlobalConfigRepository
}

// NewEcosystemCertificateReconciler creates a new reconciler for the ecosystem-certificate secret.
func NewEcosystemCertificateReconciler(secretInterface SecretClient, globalConfigRepo GlobalConfigRepository) *ecosystemCertificateReconciler {
	return &ecosystemCertificateReconciler{
		secretInterface:  secretInterface,
		globalConfigRepo: globalConfigRepo,
	}
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The ecosystemCertificateReconciler is responsible to write changes of the ecosystem certificate to the global config.
func (r *ecosystemCertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	secret, err := r.secretInterface.Get(ctx, req.Name, metav1.GetOptions{})
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(fmt.Errorf("failed to get ecosystem certificate secret: %w", err))
	}

	certificateBytes, exists := secret.Data[v1.TLSCertKey]
	if !exists {
		return ctrl.Result{}, fmt.Errorf("could not find certificate in ecosystem certificate secret")
	}

	logger.Info("Updating ecosystem certificate in global config...")
	err = r.updateCertificate(ctx, certificateBytes)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update ecosystem certificate in global config: %w", err)
	}

	logger.Info("Updated ecosystem certificate in global config")
	return ctrl.Result{}, nil
}

func (r *ecosystemCertificateReconciler) updateCertificate(ctx context.Context, certificateBytes []byte) error {
	return retry.OnError(1000, errors.IsConflictError, func() error {
		globalConfig, err := r.globalConfigRepo.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to get global config object: %w", err)
		}

		globalConfig.Config, err = globalConfig.Set(serverCertificateID, config.Value(certificateBytes))
		if err != nil {
			return fmt.Errorf("failed to set ecosystem certificate in global config object: %w", err)
		}

		// delete private key since it is a security risk
		globalConfig.Config = globalConfig.Delete(serverCertificateKeyID)

		_, err = r.globalConfigRepo.Update(ctx, globalConfig)
		if err != nil {
			return fmt.Errorf("failed write global config object: %w", err)
		}

		return nil
	})
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
