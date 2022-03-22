package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CesServiceAnnotation can be appended to services with information of ces services.
const CesServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-services"

// ServiceReconciler reconciles a Service object.
type ServiceReconciler struct {
	client.Client  `json:"client_._client"`
	Scheme         *runtime.Scheme  `json:"scheme"`
	IngressCreator IngressGenerator `json:"ingress_creator"`
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// The ServiceReconciler is responsible to generate ingress objects for respective services containing the ces service
// discovery annotation.
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	service, err := r.getService(ctx, req)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info(fmt.Sprintf("found service [%s]", service.Name))

	if !hasServicePorts(service) {
		logger.Info(fmt.Sprintf("service [%s] has no ports -> skipping ingress creation", service.Name))
		return ctrl.Result{}, nil
	}

	cesServicesAnnotation, ok := service.Annotations[CesServiceAnnotation]
	if !ok {
		logger.Info(fmt.Sprintf("found no [%s] annotation for [%s] -> creating no ingress resource", CesServiceAnnotation, service.Name))
		return ctrl.Result{}, nil
	}

	var cesServices []CesService
	err = json.Unmarshal([]byte(cesServicesAnnotation), &cesServices)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal ces services: %w", err)
	}

	for _, cesService := range cesServices {
		err := r.IngressCreator.CreateCesServiceIngress(ctx, cesService, service)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&corev1.Service{}).
		Complete(r)
}

func (r *ServiceReconciler) getService(ctx context.Context, req ctrl.Request) (*corev1.Service, error) {
	service := &corev1.Service{}
	err := r.Get(ctx, req.NamespacedName, service)
	if err != nil {
		return &corev1.Service{}, fmt.Errorf("failed to get service: %w", err)
	}

	return service, nil
}

func hasServicePorts(service *corev1.Service) bool {
	return len(service.Spec.Ports) >= 1
}
