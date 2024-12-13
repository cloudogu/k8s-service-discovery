package controllers

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// serviceReconciler watches every Service object in the cluster and creates ingress objects accordingly.
type serviceReconciler struct {
	ingressUpdater         IngressUpdater
	exposedPortUpdater     ExposedPortUpdater
	networkPolicyUpdater   NetworkPolicyUpdater
	client                 client.Client
	networkPoliciesEnabled bool
}

// NewServiceReconciler creates a new service reconciler.
func NewServiceReconciler(client client.Client, ingressUpdater IngressUpdater, exposedPortUpdater ExposedPortUpdater, networkPolicyUpdater NetworkPolicyUpdater, networkPoliciesEnabled bool) *serviceReconciler {
	return &serviceReconciler{
		client:                 client,
		ingressUpdater:         ingressUpdater,
		exposedPortUpdater:     exposedPortUpdater,
		networkPolicyUpdater:   networkPolicyUpdater,
		networkPoliciesEnabled: networkPoliciesEnabled,
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

	if !r.networkPoliciesEnabled {
		// Try to delete the networkpolicy
		logger.Info("networkpolicy support is disabled")
		err = r.networkPolicyUpdater.RemoveNetworkPolicy(ctx)
		if err != nil {
			logger.Error(fmt.Errorf("failed to delete network policy: %w", err), "networkpolicy error")
		}
	}

	if apierrors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("service %s not found", req.NamespacedName))
		return r.handleDelete(ctx, req)
	}

	logger.Info(fmt.Sprintf("Found service [%s]", service.Name))
	return r.handleUpsert(ctx, service)
}

func (r *serviceReconciler) handleUpsert(ctx context.Context, service *corev1.Service) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	err := r.ingressUpdater.UpsertIngressForService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of service [%s]: %w", service.Name, err)
	}

	err = r.exposedPortUpdater.UpsertCesLoadbalancerService(ctx, service)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update exposed ports for service [%s]: %w", service.Name, err)
	}

	if r.networkPoliciesEnabled {
		logger.Info("networkpolicy support is enabled")
		err = r.networkPolicyUpdater.UpsertNetworkPoliciesForService(ctx, service)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create/update network policies for service [%s]: %w", service.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *serviceReconciler) handleDelete(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("remove exposed ports")
	exposeErr := r.exposedPortUpdater.RemoveExposedPorts(ctx, req.Name)
	var multiErr []error
	if exposeErr != nil {
		multiErr = append(multiErr, exposeErr)
		logger.Error(exposeErr, fmt.Sprintf("failed to remove exposed ports for service %s", req.NamespacedName))
	}

	// Do not remove ports if networkpolicies are not enabled because the policy should be deleted anyway.
	if r.networkPoliciesEnabled {
		logger.Info("remove network policy ports")
		netPolErr := r.networkPolicyUpdater.RemoveExposedPorts(ctx, req.Name)
		if netPolErr != nil {
			multiErr = append(multiErr, netPolErr)
			logger.Error(netPolErr, fmt.Sprintf("failed to remove exposed ports in network policy for service %s", req.NamespacedName))
		}
	}

	return ctrl.Result{}, errors.Join(multiErr...)
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
