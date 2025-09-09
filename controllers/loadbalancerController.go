package controllers

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	exposedPortIndexKey = "k8s-service-discovery.cloudogu.com/exposedPort"
)

// LoadBalancerReconciler is responsible for reconciling the ces-loadbalancer configmap and to create / update the corresponding
// loadbalancer service. For this, it also watches Services to detect changes for exposed ports.
type LoadBalancerReconciler struct {
	Client            client.Client
	IngressController IngressController
	SvcClient         serviceClient
}

func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lbConfigMap := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, req.NamespacedName, lbConfigMap)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get config map for loadbalancer: %w", err)
	}

	lbConfig, err := types.ParseLoadbalancerConfig(lbConfigMap)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("faild to parse loadbalancer config: %w", err)
	}

	exposedDoguServices, err := r.getExposedServices(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get exposed dogu services: %w", err)
	}

	exposedPorts, err := createLoadBalancerExposedPorts(exposedDoguServices)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create loadbalancer exposed ports from dogu services: %w", err)
	}

	// TODO: Update Exposed ports in ingress controller

	if uErr := r.upsertLoadBalancer(ctx, req.Namespace, lbConfig, exposedPorts); uErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update loadbalancer: %w", uErr)
	}

	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) getExposedServices(ctx context.Context) ([]types.Service, error) {
	var k8sServiceList corev1.ServiceList
	if lErr := r.Client.List(ctx, &k8sServiceList, client.MatchingFields{exposedPortIndexKey: "true"}); lErr != nil {
		return nil, fmt.Errorf("failed to list exposed services: %w", lErr)
	}

	serviceList := make([]types.Service, 0, len(k8sServiceList.Items))
	for _, k8sService := range k8sServiceList.Items {
		serviceList = append(serviceList, types.Service(k8sService))
	}

	return serviceList, nil
}

func (r *LoadBalancerReconciler) upsertLoadBalancer(ctx context.Context, namespace string, cfg types.LoadbalancerConfig, exposedPorts types.ExposedPorts) error {
	lbObj, err := r.SvcClient.Get(ctx, types.LoadbalancerName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get service for loadbalancer: %w", err)
	}

	if apierrors.IsNotFound(err) {
		newLB := types.CreateLoadBalancer(namespace, cfg, exposedPorts, r.IngressController.GetSelector())
		if _, cErr := r.SvcClient.Create(ctx, newLB.ToK8sService(), metav1.CreateOptions{}); cErr != nil {
			return fmt.Errorf("failed to create new loadbalancer service: %w", cErr)
		}

		return nil
	}

	lb, ok := types.ParseLoadBalancer(lbObj)
	if !ok {
		return fmt.Errorf("could not parse exisiting service to LoadBalancer because of unkown type %T", lbObj)
	}

	lb.ApplyConfig(cfg)
	lb.UpdateExposedPorts(exposedPorts)

	if _, uErr := r.SvcClient.Update(ctx, lb.ToK8sService(), metav1.UpdateOptions{}); uErr != nil {
		return fmt.Errorf("failed to update exisiting loadbalancer: %w", uErr)
	}

	return nil
}

// SetupWithManager sets up the ces-loadbalancer configmap with the Manager.
// The controller watches for changes to the ces-loadbalancer configmap as well as dogu services.
// It also reconciles when the load-balancer changes.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if iErr := createExposedServiceIndex(mgr); iErr != nil {
		return fmt.Errorf("failed to create index for service with exposed ports: %w", iErr)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(
			&corev1.ConfigMap{},
			builder.WithPredicates(loadbalancerConfigPredicate()),
		).
		Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(enqueueLoadBalancerConfig),
			builder.WithPredicates(exposedPortServicePredicate()),
		).
		Owns(
			&corev1.Service{},
			builder.WithPredicates(loadbalancerServicePredicate()),
		).
		Complete(r)
}

func enqueueLoadBalancerConfig(_ context.Context, object client.Object) []reconcile.Request {
	return []reconcile.Request{{apitypes.NamespacedName{
		Namespace: object.GetNamespace(),
		Name:      types.LoadBalancerConfigName,
	}}}
}

func createExposedServiceIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Service{}, exposedPortIndexKey, func(object client.Object) []string {
		if !isExposedPortService(object) {
			return nil
		}

		return []string{"true"}
	})
}

func loadbalancerConfigPredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return object.GetName() == types.LoadBalancerConfigName
	})
}

func exposedPortServicePredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return isExposedPortService(e.Object)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return isExposedPortService(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			oldDoguService, oldIsDogu := types.ParseService(e.ObjectOld)
			newDoguService, newIsDogu := types.ParseService(e.ObjectNew)

			if oldIsDogu && newIsDogu {
				if oldDoguService.HasExposedPorts() != newDoguService.HasExposedPorts() {
					return true
				}

				oldExposedPorts, err := oldDoguService.GetExposedPorts()
				if err != nil {
					return false
				}

				newExposedPorts, err := newDoguService.GetExposedPorts()
				if err != nil {
					return false
				}

				return !oldExposedPorts.Equals(newExposedPorts)
			}

			if !oldIsDogu && !newIsDogu {
				return false
			}

			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isExposedPortService(e.Object)
		},
	}
}

func loadbalancerServicePredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return false
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			_, ok := types.ParseLoadBalancer(e.Object)
			return ok
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			oldLB, ok := types.ParseLoadBalancer(e.ObjectOld)
			if !ok {
				return false
			}

			newLB, ok := types.ParseLoadBalancer(e.ObjectNew)
			if !ok {
				return false
			}

			return !oldLB.Equals(newLB)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			_, ok := types.ParseLoadBalancer(e.Object)
			return ok
		},
	}
}

func isExposedPortService(obj metav1.Object) bool {
	doguService, ok := types.ParseService(obj)
	if !ok {
		return false
	}

	return doguService.HasExposedPorts()
}

func createLoadBalancerExposedPorts(services []types.Service) (types.ExposedPorts, error) {
	exposedPorts := types.CreateDefaultPorts()

	for _, service := range services {
		serviceExposedPorts, err := service.GetExposedPorts()
		if err != nil {
			return nil, fmt.Errorf("failred to get exposed ports from service %s: %w", service.Name, err)
		}

		exposedPorts = append(exposedPorts, serviceExposedPorts...)
	}

	exposedPorts.SortByName()

	return exposedPorts, nil
}
