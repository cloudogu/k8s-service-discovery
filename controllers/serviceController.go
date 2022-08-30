package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/cesapp-lib/core"
	"github.com/cloudogu/cesapp-lib/registry"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CesServiceAnnotation can be appended to service with information of ces services.
const CesServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-services"

// serviceReconciler watches every Service object in the cluster and creates ingress objects accordingly.
type serviceReconciler struct {
	updater  IngressUpdater
	client   client.Client
	registry registry.Registry
}

// NewServiceReconciler creates a new service reconciler.
func NewServiceReconciler(client client.Client, namespace string, updater IngressUpdater) (*serviceReconciler, error) {
	endpoint := fmt.Sprintf("http://etcd.%s.svc.cluster.local:4001", namespace)
	reg, err := registry.New(core.Registry{
		Type:      "etcd",
		Endpoints: []string{endpoint},
	})
	if err != nil {
		return nil, err
	}

	return &serviceReconciler{
		client:   client,
		updater:  updater,
		registry: reg,
	}, nil
}

// IngressUpdater is responsible to create and update the actual ingress objects in the cluster.
type IngressUpdater interface {
	// UpdateIngressOfService creates or updates the ingress object of the given service.
	UpdateIngressOfService(ctx context.Context, service *corev1.Service, isMaintenanceMode bool) error
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The serviceReconciler is responsible to generate ingress objects for respective services containing the ces service
// discovery annotation.
func (r *serviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	service, err := r.getService(ctx, req)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info(fmt.Sprintf("Found service [%s]", service.Name))

	maintanaceMode, err := isMaintenanceModeActive(r.registry)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.updater.UpdateIngressOfService(ctx, service, maintanaceMode)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create/update ingress object of service [%s]: %s", service.Name, err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *serviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&corev1.Service{}).
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

func isMaintenanceModeActive(r registry.Registry) (bool, error) {
	_, err := r.GlobalConfig().Get(maintenanceModeGlobalKey)
	if registry.IsKeyNotFoundError(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to read the maintenance mode from the registry: %w", err)
	}

	return true, nil
}
