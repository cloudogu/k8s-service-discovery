package controllers

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	primaryFQDNKey     = "fqdn"
	alternativeFQDNKey = "alternativeFQDNs"
	redirectObjectName = "redirect-alt-fqdn"
)

const (
	certEcosystemSecretName = "ecosystem-certificate"
)

// RedirectReconciler is responsible for reconciling the global configmap and to create a corresponding ingress object
// for redirecting alternative FQDNs to the primary FQDN.
type RedirectReconciler struct {
	Client             client.Client
	GlobalConfigGetter GlobalConfigRepository
	Redirector         AlternativeFQDNRedirector
}

// Reconcile reconciles the global configmap and triggers the AlternativeFQDNRedirector to redirect the alternative FQDNs.
func (r *RedirectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Reconciling global config for redirect")

	cm := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, req.NamespacedName, cm)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get global config map: %w", err)
	}

	globalCfg, err := r.GlobalConfigGetter.Get(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get global config: %w", err)
	}

	fqdn, ok := globalCfg.Get(primaryFQDNKey)
	if !ok {
		return ctrl.Result{}, fmt.Errorf("fqdn not found in global config")
	}

	altFQDNStr, _ := globalCfg.Get(alternativeFQDNKey)
	altFQDNList := assignDefaultCertSecrets(types.ParseAlternativeFQDNsFromConfigString(altFQDNStr.String()))

	setOwnerReference := func(targetObject metav1.Object) error {
		return ctrl.SetControllerReference(cm, targetObject, r.Client.Scheme(), controllerutil.WithBlockOwnerDeletion(false))
	}

	if rErr := r.Redirector.RedirectAlternativeFQDN(ctx, req.Namespace, redirectObjectName, fqdn.String(), altFQDNList, setOwnerReference); rErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to redirect alternative fqdns: %w", rErr)
	}

	return ctrl.Result{}, nil
}

func assignDefaultCertSecrets(altFQDNList []types.AlternativeFQDN) []types.AlternativeFQDN {
	result := make([]types.AlternativeFQDN, 0, len(altFQDNList))

	for _, altFQDN := range altFQDNList {
		if altFQDN.HasCertificate() {
			result = append(result, altFQDN)
		} else {
			result = append(result, types.AlternativeFQDN{
				FQDN:                  altFQDN.FQDN,
				CertificateSecretName: certEcosystemSecretName,
			})
		}
	}

	return result
}

// SetupWithManager sets up the global configmap controller with the Manager.
// The controller watches for changes to the global configmap and also reconciles when the redirect ingress object changes.
func (r *RedirectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}, builder.WithPredicates(globalConfigPredicate())).
		Owns(&networking.Ingress{}, builder.WithPredicates(redirectIngressPredicate())).
		Complete(r)
}

func globalConfigPredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == "global-config"
	})
}

func redirectIngressPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return false
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return e.Object.GetName() == redirectObjectName
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			if e.ObjectOld.GetName() != redirectObjectName {
				return false
			}

			oldIngress, ok := e.ObjectOld.(*networking.Ingress)
			if !ok {
				return false
			}
			newIngress, ok := e.ObjectNew.(*networking.Ingress)
			if !ok {
				return false
			}

			if !reflect.DeepEqual(oldIngress.Spec, newIngress.Spec) {
				return true
			}

			if !reflect.DeepEqual(oldIngress.ObjectMeta, newIngress.ObjectMeta) {
				return true
			}

			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
