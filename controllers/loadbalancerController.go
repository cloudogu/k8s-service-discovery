package controllers

import (
	"context"
	"fmt"
	"reflect"
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

// ExpositionConfig defines the configuration for discovering exposed ports
// for the load-balancer (excluding standard HTTP/HTTPS traffic).
type ExpositionConfig struct {
	// Enabled determines whether the exposition feature is active as a whole.
	// If set to false, no ports other than http / https will be exposed via this controller.
	Enabled bool

	// DiscoverServices enables port discovery via annotated corev1.Service objects.
	// This is a legacy feature and will be deprecated in future versions.
	DiscoverServices bool

	// DiscoverExpositions enables port discovery via the Exposition Custom Resource (CR).
	// This is the recommended way to configure port expositions moving forward.
	DiscoverExpositions bool
}

// LoadBalancerReconciler is responsible for reconciling the ces-loadbalancer configmap and to create / update the corresponding
// loadbalancer service. For this, it also watches Services to detect changes for exposed ports.
type LoadBalancerReconciler struct {
	ExpositionConfig  ExpositionConfig
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

func (r *LoadBalancerReconciler) getExpositions(ctx context.Context) ([]types.Exposition, error) {
	var k8sExpositionList expositionv1.ExpositionList
	if lErr := r.Client.List(ctx, &k8sExpositionList, client.MatchingFields{exposedPortIndexKey: "true"}); lErr != nil {
		return nil, fmt.Errorf("failed to list expositions: %w", lErr)
	}

	expositionList := make([]types.Exposition, 0, len(k8sExpositionList.Items))
	for _, k8sExposition := range k8sExpositionList.Items {
		expositionList = append(expositionList, types.Exposition(k8sExposition))
	}

	return expositionList, nil
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
		return fmt.Errorf("could not parse existing service to LoadBalancer because of unkown type %T", lbObj)
	}

	lb.ApplyConfig(cfg)
	lb.UpdateExposedPorts(exposedPorts)

	updatedLBService := lb.ToK8sService()
	setOwner(updatedLBService)

	updatedLBService, uErr := r.SvcClient.Update(ctx, updatedLBService, metav1.UpdateOptions{})
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
		if iErr := createExposedServiceIndex(mgr); iErr != nil {
			return fmt.Errorf("failed to create index for services with exposed ports: %w", iErr)
		}

		ctrlBuilder.Watches(
			&corev1.Service{},
			handler.EnqueueRequestsFromMapFunc(enqueueLoadBalancerConfig),
			builder.WithPredicates(exposedPortServicePredicate()),
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

	serviceList, err := r.getExposedServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch services with exposed ports: %w", err)
	}

	serviceExposedPorts, err := getExposedPorts(serviceList)
	if err != nil {
		return nil, fmt.Errorf("failed to get exposed ports from service list: %w", err)
	}

	return serviceExposedPorts, nil
}

func (r *LoadBalancerReconciler) getExposedPortsForExpositions(ctx context.Context) (types.ExposedPorts, error) {
	if !r.ExpositionConfig.DiscoverExpositions {
		return types.ExposedPorts{}, nil
	}

	expositionList, err := r.getExpositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch expositions with exposed ports: %w", err)
	}

	expositionExposedPorts, err := getExposedPorts(expositionList)
	if err != nil {
		return nil, fmt.Errorf("failed to get exposed ports from exposition list: %w", err)
	}

	return expositionExposedPorts, nil
}

func enqueueLoadBalancerConfig(_ context.Context, object client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: apitypes.NamespacedName{
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

			if len(expositionOld.Spec.TCP) != len(expositionNew.Spec.TCP) ||
				len(expositionOld.Spec.UDP) != len(expositionNew.Spec.UDP) {
				return true
			}

			tcpSortFunc := func(a, b expositionv1.TCPEntry) int {
				return strings.Compare(a.Name, b.Name)
			}
			slices.SortFunc(expositionOld.Spec.TCP, tcpSortFunc)
			slices.SortFunc(expositionNew.Spec.TCP, tcpSortFunc)

			udpSortFunc := func(a, b expositionv1.UDPEntry) int {
				return strings.Compare(a.Name, b.Name)
			}
			slices.SortFunc(expositionOld.Spec.UDP, udpSortFunc)
			slices.SortFunc(expositionNew.Spec.UDP, udpSortFunc)

			return !(reflect.DeepEqual(expositionOld.Spec.TCP, expositionNew.Spec.TCP) &&
				reflect.DeepEqual(expositionOld.Spec.UDP, expositionNew.Spec.UDP))
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
	doguPorts = slices.DeleteFunc(doguPorts, func(port types.ExposedPort) bool {
		return port.Port == 80 || port.Port == 443
	})

	exposedPorts := types.CreateDefaultPorts()
	exposedPorts = append(exposedPorts, doguPorts...)
	exposedPorts.SortByName()

	return exposedPorts
}

type exposedPortGetter interface {
	GetExposedPorts() (types.ExposedPorts, error)
}

func getExposedPorts[T exposedPortGetter](expositionObjects []T) (types.ExposedPorts, error) {
	exposedPorts := make(types.ExposedPorts, 0, len(expositionObjects))

	for _, obj := range expositionObjects {
		exposedPortList, err := obj.GetExposedPorts()
		if err != nil {
			return nil, fmt.Errorf("failed to get exposed ports from object with type %T: %w", obj, err)
		}

		exposedPorts = append(exposedPorts, exposedPortList...)
	}

	return exposedPorts, nil
}

func createExposedExpositionIndex(mgr ctrl.Manager) error {
	return mgr.GetFieldIndexer().IndexField(context.Background(), &expositionv1.Exposition{}, exposedPortIndexKey, func(object client.Object) []string {
		if !isExposedPortExposition(object) {
			return nil
		}

		return []string{"true"}
	})
}
