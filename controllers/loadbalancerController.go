package controllers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
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
	ExpositionConfig  types.ExpositionConfig
	Client            client.Client
	IngressController IngressController
	SvcClient         serviceClient
}

// Reconcile implements the controller-runtime reconcile loop for the
// LoadBalancerReconciler. It ensures that a single operator-managed
// LoadBalancer Service and related ingress configuration are kept in sync
// with the desired state derived from Dogu Services and the global
// loadbalancer configuration ConfigMap.
//
// Typical reconcile triggers:
// • Changes to the loadbalancer ConfigMap.
// • Changes to Dogu ClusterIP Services that declare exposed ports.
// • Changes to the LoadBalancer Service itself (for drift correction).
//
// Reconciliation is idempotent: calling Reconcile repeatedly with the same
// cluster state will not produce further changes.
func (r *LoadBalancerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	logger.Info("Reconciling loadbalancer")

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

	exposedPorts, err := r.getExposedPorts(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get exposed ports: %w", err)
	}

	exposedLoadBalancerPorts := createLoadBalancerExposedPorts(exposedPorts)

	uErr := r.upsertLoadBalancer(ctx, req.Namespace, lbConfig, exposedLoadBalancerPorts, setOwnerReference)
	if uErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update loadbalancer: %w", uErr)
	}

	logger.Info("Successfully applied new state to loadbalancer.")

	if eErr := r.IngressController.ExposePorts(ctx, req.Namespace, exposedPorts); eErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update exposed ports in ingress controller: %w", eErr)
	}

	logger.Info("Successfully exposed ports in IngressController.")

	return ctrl.Result{}, nil
}

func (r *LoadBalancerReconciler) getExposedServices(ctx context.Context) ([]corev1.Service, error) {
	var k8sServiceList corev1.ServiceList
	if lErr := r.Client.List(ctx, &k8sServiceList, client.MatchingFields{exposedPortIndexKey: "true"}); lErr != nil {
		return nil, fmt.Errorf("failed to list exposed services: %w", lErr)
	}

	return k8sServiceList.Items, nil
}

func (r *LoadBalancerReconciler) getExpositions(ctx context.Context) ([]expositionv1.Exposition, error) {
	var k8sExpositionList expositionv1.ExpositionList
	if lErr := r.Client.List(ctx, &k8sExpositionList, client.MatchingFields{exposedPortIndexKey: "true"}); lErr != nil {
		return nil, fmt.Errorf("failed to list expositions: %w", lErr)
	}

	return k8sExpositionList.Items, nil
}

func (r *LoadBalancerReconciler) upsertLoadBalancer(ctx context.Context, namespace string, cfg types.LoadbalancerConfig, exposedPorts types.ExposedPorts, setOwner func(object metav1.Object)) error {
	lbObj, err := r.SvcClient.Get(ctx, types.LoadbalancerName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get service for loadbalancer: %w", err)
	}

	if apierrors.IsNotFound(err) {
		newLB := types.CreateLoadBalancer(namespace, cfg, exposedPorts, r.IngressController.GetSelector())
		newLBService := newLB.ToK8sService()
		setOwner(newLBService)

		_, cErr := r.SvcClient.Create(ctx, newLBService, metav1.CreateOptions{})
		if cErr != nil {
			return fmt.Errorf("failed to create new loadbalancer service: %w", cErr)
		}

		return nil
	}

	lb, ok := types.ParseLoadBalancer(lbObj)
	if !ok {
		return fmt.Errorf("could not parse existing service to LoadBalancer because of unknown type %T", lbObj)
	}

	desired := types.CreateLoadBalancer(namespace, cfg, exposedPorts, r.IngressController.GetSelector())
	if lb.Equals(desired) {
		return nil
	}

	lb.ApplyConfig(cfg)
	lb.UpdateExposedPorts(exposedPorts)

	updatedLBService := lb.ToK8sService()
	setOwner(updatedLBService)

	_, uErr := r.SvcClient.Update(ctx, updatedLBService, metav1.UpdateOptions{})
	if uErr != nil {
		return fmt.Errorf("failed to update existing loadbalancer: %w", uErr)
	}

	return nil
}

// SetupWithManager sets up the ces-loadbalancer configmap with the Manager.
// The controller watches for changes to the ces-loadbalancer configmap as well as dogu services.
// It also reconciles when the load-balancer changes.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctrlBuilder := ctrl.NewControllerManagedBy(mgr).
		For(
			&corev1.ConfigMap{},
			builder.WithPredicates(loadbalancerConfigPredicate()),
		).
		Owns(
			&corev1.Service{},
			builder.WithPredicates(loadbalancerServicePredicate()),
		).
		Named("loadbalancer-configmap")

	if !r.ExpositionConfig.Enabled {
		return ctrlBuilder.Complete(r)
	}

	if r.ExpositionConfig.DiscoverServices {
		if iErr := r.createExposedServiceIndex(mgr); iErr != nil {
			return fmt.Errorf("failed to create index for services with exposed ports: %w", iErr)
		}

		ctrlBuilder.Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(enqueueLoadBalancerConfig),
			builder.WithPredicates(r.exposedPortServicePredicate()),
		)
	}

	if r.ExpositionConfig.DiscoverExpositions {
		if iErr := createExposedExpositionIndex(mgr); iErr != nil {
			return fmt.Errorf("failed to create index for exposition: %w", iErr)
		}

		ctrlBuilder.Watches(
			&expositionv1.Exposition{},
			handler.EnqueueRequestsFromMapFunc(enqueueLoadBalancerConfig),
			builder.WithPredicates(exposedPortExpositionPredicate()),
		)
	}

	return ctrlBuilder.Complete(r)
}

func (r *LoadBalancerReconciler) getExposedPorts(ctx context.Context) (types.ExposedPorts, error) {
	if !r.ExpositionConfig.Enabled {
		return types.ExposedPorts{}, nil
	}

	exposedPorts := make(types.ExposedPorts, 0)

	serviceExposedPorts, err := r.getExposedPortsForServices(ctx)
	if err != nil {
		return types.ExposedPorts{}, fmt.Errorf("failed to get exposed ports from services: %w", err)
	}

	expositionExposedPorts, err := r.getExposedPortsForExpositions(ctx)
	if err != nil {
		return types.ExposedPorts{}, fmt.Errorf("failed to get exposed ports from expositions: %w", err)
	}

	exposedPorts = append(exposedPorts, serviceExposedPorts...)
	exposedPorts = append(exposedPorts, expositionExposedPorts...)
	exposedPorts.SortByName()

	return exposedPorts, nil
}

func (r *LoadBalancerReconciler) getExposedPortsForServices(ctx context.Context) (types.ExposedPorts, error) {
	if !r.ExpositionConfig.DiscoverServices {
		return types.ExposedPorts{}, nil
	}

	logger := ctrl.LoggerFrom(ctx)

	serviceList, err := r.getExposedServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch services with exposed ports: %w", err)
	}

	exposedPorts := make(types.ExposedPorts, 0, len(serviceList))

	for _, service := range serviceList {
		serviceExposition, mErr := mapServiceToExposition(&service, r.Client.Scheme())
		if mErr != nil {
			// don't let a single corrupted service block exposing ports from other services
			logger.Error(mErr, "failed to map service to exposition while exposing ports", "service", service.Name)
		}

		exposedPorts = append(exposedPorts, serviceExposition.TcpRoutes...)
		exposedPorts = append(exposedPorts, serviceExposition.UdpRoutes...)
	}

	return exposedPorts, nil
}

func (r *LoadBalancerReconciler) getExposedPortsForExpositions(ctx context.Context) (types.ExposedPorts, error) {
	if !r.ExpositionConfig.DiscoverExpositions {
		return types.ExposedPorts{}, nil
	}

	logger := ctrl.LoggerFrom(ctx)

	expositionList, err := r.getExpositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch expositions with exposed ports: %w", err)
	}

	exposedPorts := make(types.ExposedPorts, 0, len(expositionList))

	for _, expositionCR := range expositionList {
		exposition, mErr := mapExpositionCRToExposition(&expositionCR, r.Client.Scheme())
		if mErr != nil {
			// don't let a single corrupted exposition block exposing ports from other expositions
			logger.Error(mErr, "failed to map expositionCR to exposition while exposing ports", "exposition", expositionCR.Name)
		}

		exposedPorts = append(exposedPorts, exposition.TcpRoutes...)
		exposedPorts = append(exposedPorts, exposition.UdpRoutes...)
	}

	return exposedPorts, nil
}

func enqueueLoadBalancerConfig(_ context.Context, object client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: apitypes.NamespacedName{
		Namespace: object.GetNamespace(),
		Name:      types.LoadBalancerConfigName,
	}}}
}

func (r *LoadBalancerReconciler) createExposedServiceIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Service{}, exposedPortIndexKey, func(object client.Object) []string {
		if !r.isExposedPortService(object) {
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

func exposedPortExpositionPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return isExposedPortExposition(e.Object)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return isExposedPortExposition(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			expositionOld, ok := e.ObjectOld.(*expositionv1.Exposition)
			if !ok {
				return false
			}

			expositionNew, ok := e.ObjectNew.(*expositionv1.Exposition)
			if !ok {
				return false
			}

			tcpSortFunc := func(a, b expositionv1.TCPEntry) int {
				return strings.Compare(a.Name, b.Name)
			}
			tcpOld := slices.SortedFunc(slices.Values(expositionOld.Spec.TCP), tcpSortFunc)
			tcpNew := slices.SortedFunc(slices.Values(expositionNew.Spec.TCP), tcpSortFunc)

			udpSortFunc := func(a, b expositionv1.UDPEntry) int {
				return strings.Compare(a.Name, b.Name)
			}
			udpOld := slices.SortedFunc(slices.Values(expositionOld.Spec.UDP), udpSortFunc)
			udpNew := slices.SortedFunc(slices.Values(expositionNew.Spec.UDP), udpSortFunc)

			return !(slices.Equal(tcpOld, tcpNew) && slices.Equal(udpOld, udpNew))
		},
		GenericFunc: func(e event.TypedGenericEvent[client.Object]) bool {
			return isExposedPortExposition(e.Object)
		},
	}
}

func isExposedPortExposition(object metav1.Object) bool {
	exposition, ok := object.(*expositionv1.Exposition)
	if !ok {
		return false
	}

	return len(exposition.Spec.TCP)+len(exposition.Spec.UDP) > 0
}

func (r *LoadBalancerReconciler) exposedPortServicePredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.TypedCreateEvent[client.Object]) bool {
			return r.isExposedPortService(e.Object)
		},
		DeleteFunc: func(e event.TypedDeleteEvent[client.Object]) bool {
			return r.isExposedPortService(e.Object)
		},
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			oldDoguService, oldIsDogu := r.mapToExposedService(e.ObjectOld)
			newDoguService, newIsDogu := r.mapToExposedService(e.ObjectNew)

			if oldIsDogu && newIsDogu {
				if oldDoguService.HasExposedPorts() != newDoguService.HasExposedPorts() {
					return true
				}

				oldExposedPorts := oldDoguService.GetExposedPorts()
				newExposedPorts := newDoguService.GetExposedPorts()

				return !oldExposedPorts.Equals(newExposedPorts)
			}

			if !oldIsDogu && !newIsDogu {
				return false
			}

			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return r.isExposedPortService(e.Object)
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

func (r *LoadBalancerReconciler) mapToExposedService(obj client.Object) (exposedService, bool) {
	if !isDoguService(obj) {
		return exposedService{}, false
	}

	service := obj.(*corev1.Service)

	exposition, err := mapServiceToExposition(service, r.Client.Scheme())
	if err != nil {
		return exposedService{}, false
	}

	return exposedService{Exposition: exposition}, true
}

func (r *LoadBalancerReconciler) isExposedPortService(obj client.Object) bool {
	eService, ok := r.mapToExposedService(obj)
	if !ok {
		return false
	}

	return eService.HasExposedPorts()
}

func createLoadBalancerExposedPorts(doguPorts types.ExposedPorts) types.ExposedPorts {
	// Strip any caller-provided 80/443 entries so the canonical "http"/"https" default ports
	// added below are always present with consistent names.
	doguPorts = slices.DeleteFunc(doguPorts, func(port types.ExposedPort) bool {
		return port.ServicePort == 80 || port.ServicePort == 443
	})

	exposedPorts := types.CreateDefaultPorts()
	exposedPorts = append(exposedPorts, doguPorts...)
	exposedPorts.SortByName()

	return exposedPorts
}

func createExposedExpositionIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(context.Background(), &expositionv1.Exposition{}, exposedPortIndexKey, func(object client.Object) []string {
		if !isExposedPortExposition(object) {
			return nil
		}

		return []string{"true"}
	})
}

type exposedService struct {
	types.Exposition
}

func (e exposedService) GetExposedPorts() types.ExposedPorts {
	exposedPorts := make(types.ExposedPorts, 0, len(e.TcpRoutes)+len(e.UdpRoutes))
	exposedPorts = append(exposedPorts, e.TcpRoutes...)
	exposedPorts = append(exposedPorts, e.UdpRoutes...)

	return exposedPorts
}

func (e exposedService) HasExposedPorts() bool {
	return len(e.TcpRoutes)+len(e.UdpRoutes) > 0
}
