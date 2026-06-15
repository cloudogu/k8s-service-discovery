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

	conditionTypeLBPortAllocation = "LoadBalancerPortsAllocated"
	conditionReasonPortCollision  = "PortCollision"
	conditionReasonPortAllocated  = "PortsAllocated"
)

// LoadBalancerReconciler is responsible for reconciling the ces-loadbalancer configmap and to create / update the corresponding
// loadbalancer service. For this, it also watches Services to detect changes for exposed ports.
type LoadBalancerReconciler struct {
	IngressSelector  map[string]string
	ExpositionConfig types.ExpositionConfig
	Client           client.Client
	PortExposer      PortExposer
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

	expositions, err := r.getExpositions(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get expositions: %w", err)
	}

	validExpositions, collisionMap := checkPortCollisions(expositions)
	setPortsAllocatedConditionError(ctx, collisionMap)

	uErr := r.upsertLoadBalancer(ctx, req.Namespace, lbConfig, validExpositions, setOwnerReference)
	if uErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update loadbalancer: %w", uErr)
	}

	setPortsAllocatedCondition(ctx, validExpositions)
	logger.Info("Successfully applied new state to loadbalancer.")

	if eErr := r.PortExposer.ExposePorts(ctx, validExpositions); eErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to expose ports in ingress controller: %w", eErr)
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

func (r *LoadBalancerReconciler) getExpositionCRs(ctx context.Context) ([]expositionv1.Exposition, error) {
	var k8sExpositionList expositionv1.ExpositionList
	if lErr := r.Client.List(ctx, &k8sExpositionList, client.MatchingFields{exposedPortIndexKey: "true"}); lErr != nil {
		return nil, fmt.Errorf("failed to list expositions: %w", lErr)
	}

	return k8sExpositionList.Items, nil
}

// upsertLoadBalancer creates the LoadBalancer Service if it does not exist,
// or updates it when the desired state differs from the current state.
// setOwner wires the owner reference on any created or updated object.
func (r *LoadBalancerReconciler) upsertLoadBalancer(ctx context.Context, namespace string, cfg types.LoadbalancerConfig, expositions []types.Exposition, setOwner func(object metav1.Object)) error {
	lbObj := &corev1.Service{}
	gErr := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: types.LoadbalancerName}, lbObj)
	if gErr != nil && !apierrors.IsNotFound(gErr) {
		return fmt.Errorf("failed to get service for loadbalancer: %w", gErr)
	}

	desired := types.CreateLoadBalancer(namespace, cfg, expositions, r.IngressSelector)

	if apierrors.IsNotFound(gErr) {
		newLBService := desired.ToK8sService()
		setOwner(newLBService)

		if cErr := r.Client.Create(ctx, newLBService); cErr != nil {
			return fmt.Errorf("failed to create new loadbalancer service: %w", cErr)
		}

		return nil
	}

	lb, ok := types.ParseLoadBalancer(lbObj)
	if !ok {
		return fmt.Errorf("could not parse existing service to LoadBalancer because of unknown type %T", lbObj)
	}

	if lb.Equals(desired) {
		return nil
	}

	lb.ApplyConfig(cfg)
	lb.UpdateExposedPorts(types.CreateLoadBalancerExposedPorts(expositions))

	updatedLBService := lb.ToK8sService()
	setOwner(updatedLBService)

	if uErr := r.Client.Update(ctx, updatedLBService); uErr != nil {
		return fmt.Errorf("failed to update existing loadbalancer: %w", uErr)
	}

	return nil
}

// SetupWithManager sets up the ces-loadbalancer configmap with the Manager.
// The controller watches for changes to the ces-loadbalancer configmap as well as dogu services.
// It also reconciles when the load-balancer changes.
func (r *LoadBalancerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.IngressSelector == nil {
		return fmt.Errorf("IngressSelector for LoadBalancer is not set")
	}

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

func (r *LoadBalancerReconciler) getExpositions(ctx context.Context) ([]types.Exposition, error) {
	if !r.ExpositionConfig.Enabled {
		return []types.Exposition{}, nil
	}

	serviceExpositions, err := r.getExpositionsForServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get expositions for services: %w", err)
	}

	crExpositions, err := r.getExpositionsForExpositionCRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get expositions for exposition CRs: %w", err)
	}

	return slices.Concat(serviceExpositions, crExpositions), nil
}

func (r *LoadBalancerReconciler) getExpositionsForServices(ctx context.Context) ([]types.Exposition, error) {
	if !r.ExpositionConfig.DiscoverServices {
		return []types.Exposition{}, nil
	}

	logger := ctrl.LoggerFrom(ctx)

	serviceList, err := r.getExposedServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch services with exposed ports: %w", err)
	}

	expositions := make([]types.Exposition, 0, len(serviceList))

	for _, service := range serviceList {
		serviceExposition, mErr := mapServiceToExposition(&service, r.Client)
		if mErr != nil {
			// don't let a single corrupted service block exposing ports from other services
			logger.Error(mErr, "failed to map service to exposition while exposing ports", "service", service.Name)
			continue
		}

		expositions = append(expositions, serviceExposition)
	}

	return expositions, nil
}

func (r *LoadBalancerReconciler) getExpositionsForExpositionCRs(ctx context.Context) ([]types.Exposition, error) {
	if !r.ExpositionConfig.DiscoverExpositions {
		return []types.Exposition{}, nil
	}

	logger := ctrl.LoggerFrom(ctx)

	expositionList, err := r.getExpositionCRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch expositions with exposed ports: %w", err)
	}

	expositions := make([]types.Exposition, 0, len(expositionList))

	for _, expositionCR := range expositionList {
		exposition, mErr := mapExpositionCRToExposition(&expositionCR, r.Client)
		if mErr != nil {
			// don't let a single corrupted exposition block exposing ports from other expositions
			logger.Error(mErr, "failed to map expositionCR to exposition", "exposition", expositionCR.Name)
			continue
		}

		expositions = append(expositions, exposition)
	}

	return expositions, nil
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

	exposition, err := mapServiceToExposition(service, r.Client)
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

// portProtocolKey is the collision key: two ports only collide when both the
// external port number and the protocol match, since K8s allows the same port
// number for TCP and UDP as distinct Service port entries.
type portProtocolKey struct {
	port     int32
	protocol corev1.Protocol
}

// checkPortCollisions detects expositions that claim the same external port
// and protocol. It returns the subset of expositions that have no collisions,
// and a map from each colliding exposition pointer (into the input slice) to
// the colliding (port, protocol) pairs. Callers must not modify the input
// slice after this call as long as the returned map is in use.
func checkPortCollisions(expositions []types.Exposition) ([]types.Exposition, map[*types.Exposition][]portProtocolKey) {
	portMap := make(map[portProtocolKey][]*types.Exposition, len(expositions))

	for i := range expositions {
		e := &expositions[i]
		expositionPorts := slices.Concat(e.TcpRoutes, e.UdpRoutes)
		for _, port := range expositionPorts {
			key := portProtocolKey{port: port.RequestedExternalPort, protocol: port.Protocol}
			portMap[key] = append(portMap[key], e)
		}
	}

	collisionMap := make(map[*types.Exposition][]portProtocolKey)

	for key, eList := range portMap {
		if len(eList) > 1 {
			for _, e := range eList {
				collisionMap[e] = append(collisionMap[e], key)
			}
		}
	}

	var filteredList []types.Exposition
	for i := range expositions {
		e := &expositions[i]
		if _, hasCollision := collisionMap[e]; !hasCollision {
			filteredList = append(filteredList, *e)
		}
	}

	return filteredList, collisionMap
}

// setPortsAllocatedConditionError writes a PortCollision condition on every
// exposition that lost the collision check. Service-based expositions have no
// SetCondition wired up, so they are silently skipped.
func setPortsAllocatedConditionError(ctx context.Context, collisionMap map[*types.Exposition][]portProtocolKey) {
	logger := ctrl.LoggerFrom(ctx)

	for e, keys := range collisionMap {
		if e.SetCondition == nil {
			continue
		}

		msg := collisionMessage(keys)
		if cErr := e.SetCondition(
			ctx,
			conditionTypeLBPortAllocation,
			false,
			conditionReasonPortCollision,
			msg,
		); cErr != nil {
			logger.Error(cErr, "failed to set condition", "type", conditionTypeLBPortAllocation, "exposition", e.Name)
		}
	}
}

// collisionMessage builds a human-readable description of the colliding
// (port, protocol) pairs, e.g. "port collision for: TCP/22, UDP/53".
func collisionMessage(keys []portProtocolKey) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s/%d", k.protocol, k.port))
	}
	return fmt.Sprintf("port collision for: %s", strings.Join(parts, ", "))
}

// setPortsAllocatedCondition writes a PortsAllocated success condition on
// every exposition that passed the collision check. Failures are logged and
// not returned — condition writes are best-effort and must not block the
// primary reconcile work. Service-based expositions have no SetCondition
// wired up, so they are silently skipped.
func setPortsAllocatedCondition(ctx context.Context, expositions []types.Exposition) {
	logger := ctrl.LoggerFrom(ctx)

	for _, e := range expositions {
		if e.SetCondition == nil {
			continue
		}

		if cErr := e.SetCondition(
			ctx,
			conditionTypeLBPortAllocation,
			true,
			conditionReasonPortAllocated,
			fmt.Sprint("All requested ports were successfully allocated."),
		); cErr != nil {
			logger.Error(cErr, "failed to set condition", "type", conditionTypeLBPortAllocation, "exposition", e.Name)
		}
	}
}
