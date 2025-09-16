package controllers

import (
	"context"
	"fmt"
	"slices"

	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	logger := ctrl.LoggerFrom(ctx)

	lbConfigMap := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, req.NamespacedName, lbConfigMap)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get config map for loadbalancer: %w", err)
	}

	lbConfig, err := types.ParseLoadbalancerConfig(lbConfigMap)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to parse loadbalancer config: %w", err)
	}

	setOwnerReference := func(targetObject metav1.Object) {
		if oErr := ctrl.SetControllerReference(lbConfigMap, targetObject, r.Client.Scheme(), controllerutil.WithBlockOwnerDeletion(false)); oErr != nil {
			logger.Info("Failed to set controller referencer", "object", targetObject.GetName(), "error", oErr)
		}
	}

	exposedDoguServices, err := r.getExposedServices(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get exposed dogu services: %w", err)
	}

	exposedDoguPorts, err := getExposedDoguPorts(exposedDoguServices)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get exposed ports from services: %w", err)
	}

	exposedLoadBalancerPorts := createLoadBalancerExposedPorts(exposedDoguPorts)

	lb, uErr := r.upsertLoadBalancer(ctx, req.Namespace, lbConfig, exposedLoadBalancerPorts, setOwnerReference)
	if uErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update loadbalancer: %w", uErr)
	}

	owner, err := lb.GetOwnerReference(r.Client.Scheme())
	if err != nil {
		logger.Info("Could not get OwnerReference from loadbalancer", "error", err)
	}

	if eErr := r.IngressController.ExposePorts(ctx, req.Namespace, exposedDoguPorts, owner); eErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update exposed ports in ingress controller: %w", eErr)
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

func (r *LoadBalancerReconciler) upsertLoadBalancer(ctx context.Context, namespace string, cfg types.LoadbalancerConfig, exposedPorts types.ExposedPorts, setOwner func(object metav1.Object)) (types.LoadBalancer, error) {
	lbObj, err := r.SvcClient.Get(ctx, types.LoadbalancerName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return types.LoadBalancer{}, fmt.Errorf("failed to get service for loadbalancer: %w", err)
	}

	if apierrors.IsNotFound(err) {
		newLB := types.CreateLoadBalancer(namespace, cfg, exposedPorts, r.IngressController.GetSelector())
		newLBService := newLB.ToK8sService()
		setOwner(newLBService)

		lbService, cErr := r.SvcClient.Create(ctx, newLBService, metav1.CreateOptions{})
		if cErr != nil {
			return types.LoadBalancer{}, fmt.Errorf("failed to create new loadbalancer service: %w", cErr)
		}

		return types.LoadBalancer(*lbService), nil
	}

	lb, ok := types.ParseLoadBalancer(lbObj)
	if !ok {
		return types.LoadBalancer{}, fmt.Errorf("could not parse exisiting service to LoadBalancer because of unkown type %T", lbObj)
	}

	lb.ApplyConfig(cfg)
	lb.UpdateExposedPorts(exposedPorts)

	updatedLBService := lb.ToK8sService()
	setOwner(updatedLBService)

	updatedLBService, uErr := r.SvcClient.Update(ctx, updatedLBService, metav1.UpdateOptions{})
	if uErr != nil {
		return types.LoadBalancer{}, fmt.Errorf("failed to update exisiting loadbalancer: %w", uErr)
	}

	return types.LoadBalancer(*updatedLBService), nil
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
		Named("loadbalancer-configmap").
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

func createLoadBalancerExposedPorts(doguPorts types.ExposedPorts) types.ExposedPorts {
	// Delete default ports 80 and 443 as they are handled by the loadbalancer
	slices.DeleteFunc(doguPorts, func(port types.ExposedPort) bool {
		return port.Port == 80 || port.Port == 443
	})

	exposedPorts := types.CreateDefaultPorts()
	exposedPorts = append(exposedPorts, doguPorts...)
	exposedPorts.SortByName()

	return exposedPorts
}

func getExposedDoguPorts(services []types.Service) (types.ExposedPorts, error) {
	exposedDoguPorts := make(types.ExposedPorts, 0, len(services))

	for _, service := range services {
		serviceExposedPorts, err := service.GetExposedPorts()
		if err != nil {
			return nil, fmt.Errorf("failed to get exposed ports from service %s: %w", service.Name, err)
		}

		exposedDoguPorts = append(exposedDoguPorts, serviceExposedPorts...)
	}

	exposedDoguPorts.SortByName()

	return exposedDoguPorts, nil
}
