package adapter

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const traefikMiddlewareAnnotationKey = "traefik.ingress.kubernetes.io/router.middlewares"

const ownedByLabelKey = "k8s-service-discovery.cloudogu.com/owned-by"

const (
	staticContentMaintenanceRewrite    = "maintenance-mode@kubernetescrd"
	staticContentDoguIsStartingRewrite = "dogu-starting@kubernetescrd"
)

const (
	ConditionTypeIngressTCPRoutesCreated = "IngressTCPRoutesCreated"
	ConditionTypeIngressUDPRoutesCreated = "IngressUDPRoutesCreated"
)

// TraefikIngressController implements the IngressController interface using Traefik-specific
// Kubernetes resources (Ingress, Middleware, IngressRouteTCP, IngressRouteUDP).
type TraefikIngressController struct {
	IngressClass string
	Client       client.Client
}

// GetOwnableTypes returns the resource types this controller creates, so the exposition
// reconciler can register ownership watches and trigger cascade deletion when an
// resource is removed.
func (t *TraefikIngressController) GetOwnableTypes() []client.Object {
	return []client.Object{
		&networkingv1.Ingress{},
		&traefikapi.Middleware{},
		&traefikapi.IngressRouteTCP{},
		&traefikapi.IngressRouteUDP{},
	}
}

// ProcessExposition reconciles HTTP routing for a single Exposition by generating the
// corresponding Ingress and Middleware objects and applying them via create-or-update.
// State-specific rewrites (maintenance, starting) override any route-level rewrite config.
// All ingress and middleware errors within one exposition are collected and returned together.
func (t *TraefikIngressController) ProcessExposition(ctx context.Context, appState types.ApplicationState, exposition types.Exposition) error {
	ingresses, middlewares, err := t.generate(exposition, appState)
	if err != nil {
		return fmt.Errorf("failed to generate ingresses or middlewares: %w", err)
	}

	var errs []error
	err = t.upsertIngresses(ctx, exposition, ingresses)
	if err != nil {
		errs = append(errs, err)
	}

	err = t.upsertMiddlewares(ctx, exposition, middlewares)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to upsert ingresses or middlewares for %q: %w", exposition.Name, errors.Join(errs...))
	}

	return nil
}

// ExposePorts reconciles Traefik IngressRouteTCP and IngressRouteUDP objects for every
// Exposition. Errors from one exposition are collected rather than returned immediately,
// so a failure in one exposition never blocks the others from being processed.
func (t *TraefikIngressController) ExposePorts(ctx context.Context, expositions []types.Exposition) error {
	var errs []error
	for _, exposition := range expositions {
		if err := t.exposeTCPRoutes(ctx, exposition); err != nil {
			errs = append(errs, fmt.Errorf("failed to expose tcp routes for exposition %s: %w", exposition.Name, err))
		}

		if err := t.exposeUDPRoutes(ctx, exposition); err != nil {
			errs = append(errs, fmt.Errorf("failed to expose udp routes for exposition %s: %w", exposition.Name, err))
		}
	}

	return errors.Join(errs...)
}

// exposeTCPRoutes creates or updates one IngressRouteTCP per entry in TcpRoutes, then
// deletes any previously owned IngressRouteTCP objects that are no longer desired.
// Per-route errors are collected so the cleanup pass always runs even when some upserts fail.
func (t *TraefikIngressController) exposeTCPRoutes(ctx context.Context, exposition types.Exposition) error {
	existing := &traefikapi.IngressRouteTCPList{}
	if err := t.Client.List(ctx, existing, &client.ListOptions{Namespace: exposition.Namespace, LabelSelector: selectorFromExpositionName(exposition.Name)}); err != nil {
		return fmt.Errorf("failed to list existing tcp routes: %w", err)
	}

	existingMap := make(map[string]traefikapi.IngressRouteTCP, len(existing.Items))
	for _, item := range existing.Items {
		existingMap[item.Name] = item
	}

	var errs []error
	for _, tcpPort := range exposition.TcpRoutes {
		tcpRoute := createIngressRouteTCP(exposition.Name, exposition.Namespace, tcpPort)
		// mark as desired immediately so it is never deleted even if the upsert fails
		delete(existingMap, tcpRoute.Name)

		if err := exposition.SetOwner(tcpRoute); err != nil {
			wErr := fmt.Errorf("failed to set owner reference for IngressTCPRoute from exposition %s: %w", exposition.Name, err)
			_ = exposition.SetCondition(ctx, ConditionTypeIngressTCPRoutesCreated, false, "OwnerReferenceFailed", wErr.Error())
			errs = append(errs, wErr)
			continue
		}

		updateRef := &traefikapi.IngressRouteTCP{ObjectMeta: metav1.ObjectMeta{Name: tcpRoute.Name, Namespace: tcpRoute.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, t.Client, updateRef, func() error {
			updateRef.Annotations = tcpRoute.Annotations
			updateRef.OwnerReferences = tcpRoute.OwnerReferences
			updateRef.Labels = tcpRoute.Labels
			updateRef.Spec = tcpRoute.Spec
			return nil
		}); err != nil {
			wErr := fmt.Errorf("failed to create or update ingress tcp route %q: %w", tcpRoute.Name, err)
			_ = exposition.SetCondition(ctx, ConditionTypeIngressTCPRoutesCreated, false, "TCPUpdateFailed", wErr.Error())
			errs = append(errs, wErr)
			continue
		}
	}

	for _, stale := range existingMap {
		if err := t.Client.Delete(ctx, &stale); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete outdated tcp route %q: %w", stale.Name, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}

	if len(exposition.TcpRoutes) == 0 {
		return nil
	}

	if err := exposition.SetCondition(
		ctx,
		ConditionTypeIngressTCPRoutesCreated,
		true,
		"IngressRouteTCPReady",
		"Routes for TCP were successfully created.",
	); err != nil {
		return fmt.Errorf("failed to set condition for %s: %w", ConditionTypeIngressTCPRoutesCreated, err)
	}

	return nil
}

// exposeUDPRoutes creates or updates one IngressRouteUDP per entry in UdpRoutes, then
// deletes any previously owned IngressRouteUDP objects that are no longer desired.
// Per-route errors are collected so the cleanup pass always runs even when some upserts fail.
func (t *TraefikIngressController) exposeUDPRoutes(ctx context.Context, exposition types.Exposition) error {
	existing := &traefikapi.IngressRouteUDPList{}
	if err := t.Client.List(ctx, existing, &client.ListOptions{Namespace: exposition.Namespace, LabelSelector: selectorFromExpositionName(exposition.Name)}); err != nil {
		return fmt.Errorf("failed to list existing udp routes: %w", err)
	}

	existingMap := make(map[string]traefikapi.IngressRouteUDP, len(existing.Items))
	for _, item := range existing.Items {
		existingMap[item.Name] = item
	}

	var errs []error
	for _, udpPort := range exposition.UdpRoutes {
		udpRoute := createIngressRouteUDP(exposition.Name, exposition.Namespace, udpPort)
		// mark as desired immediately so it is never deleted even if the upsert fails
		delete(existingMap, udpRoute.Name)

		if err := exposition.SetOwner(udpRoute); err != nil {
			wErr := fmt.Errorf("failed to set owner reference for IngressUDPRoute from exposition %s: %w", exposition.Name, err)
			_ = exposition.SetCondition(ctx, ConditionTypeIngressUDPRoutesCreated, false, "OwnerReferenceFailed", wErr.Error())
			errs = append(errs, wErr)
			continue
		}

		updateRef := &traefikapi.IngressRouteUDP{ObjectMeta: metav1.ObjectMeta{Name: udpRoute.Name, Namespace: udpRoute.Namespace}}
		if _, err := controllerutil.CreateOrUpdate(ctx, t.Client, updateRef, func() error {
			updateRef.Annotations = udpRoute.Annotations
			updateRef.OwnerReferences = udpRoute.OwnerReferences
			updateRef.Labels = udpRoute.Labels
			updateRef.Spec = udpRoute.Spec
			return nil
		}); err != nil {
			wErr := fmt.Errorf("failed to create or update ingress udp route %q: %w", udpRoute.Name, err)
			_ = exposition.SetCondition(ctx, ConditionTypeIngressUDPRoutesCreated, false, "UDPUpdateFailed", wErr.Error())
			errs = append(errs, wErr)
			continue
		}
	}

	for _, stale := range existingMap {
		if err := t.Client.Delete(ctx, &stale); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete outdated udp route %q: %w", stale.Name, err))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}

	if len(exposition.UdpRoutes) == 0 {
		return nil
	}

	if err := exposition.SetCondition(
		ctx,
		ConditionTypeIngressUDPRoutesCreated,
		true,
		"IngressRouteUDPReady",
		"Routes for UDP were successfully created.",
	); err != nil {
		return fmt.Errorf("failed to set condition for %s: %w", ConditionTypeIngressUDPRoutesCreated, err)
	}

	return nil
}

// generate builds the desired Ingress and Middleware objects for the given application state.
// Returns empty slices when the application is stopped (caller interprets this as "delete all").
func (t *TraefikIngressController) generate(exposition types.Exposition, appState types.ApplicationState) ([]*networkingv1.Ingress, []*traefikapi.Middleware, error) {
	if appState == types.ApplicationStopped {
		return nil, nil, nil
	}

	ingresses := make([]*networkingv1.Ingress, 0, len(exposition.HttpRoutes))
	var middlewares []*traefikapi.Middleware
	var errs []error
	for _, route := range exposition.HttpRoutes {
		var middlewareRef string
		if appState == types.ApplicationMaintenance {
			middlewareRef = staticContentMaintenanceRewrite
		} else if appState == types.ApplicationIsStarting {
			middlewareRef = staticContentDoguIsStartingRewrite
		} else if route.Rewrite != nil {
			middleware := t.generateMiddleware(exposition, route)
			err := exposition.SetOwner(middleware)
			if err != nil {
				errs = append(errs, err)
			}

			middlewareRef = fmt.Sprintf("%s-%s@kubernetescrd", exposition.Namespace, middleware.Name)
			middlewares = append(middlewares, middleware)
		}

		ingress := t.generateIngress(exposition, route, middlewareRef)
		err := exposition.SetOwner(ingress)
		if err != nil {
			errs = append(errs, err)
		}

		ingresses = append(ingresses, ingress)
	}

	return ingresses, middlewares, errors.Join(errs...)
}

func (t *TraefikIngressController) generateIngress(exposition types.Exposition, httpRoute types.HttpRoute, middlewareRef string) *networkingv1.Ingress {
	annotations := make(map[string]string, 1)
	if middlewareRef != "" {
		annotations[traefikMiddlewareAnnotationKey] = middlewareRef
	}

	selectionLabels := map[string]string{ownedByLabelKey: exposition.Name}
	maps.Insert(selectionLabels, maps.All(util.K8sCesServiceDiscoveryLabels))

	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        httpRoute.Name,
			Namespace:   exposition.Namespace,
			Annotations: annotations,
			Labels:      selectionLabels,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &t.IngressClass,
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     httpRoute.Path,
							PathType: new(networkingv1.PathTypePrefix),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: httpRoute.Service,
									Port: networkingv1.ServiceBackendPort{
										Number: httpRoute.Port,
									},
								},
							},
						}},
					},
				},
			}},
		},
	}
}

func (t *TraefikIngressController) generateMiddleware(exposition types.Exposition, httpRoute types.HttpRoute) *traefikapi.Middleware {
	var replacePathRegex *dynamic.ReplacePathRegex
	var stripPrefix *dynamic.StripPrefix
	if httpRoute.Rewrite.Regex != nil {
		replacePathRegex = &dynamic.ReplacePathRegex{
			Regex:       httpRoute.Rewrite.Regex.Pattern,
			Replacement: httpRoute.Rewrite.Regex.Replacement,
		}
	}
	if httpRoute.Rewrite.StripPrefix != nil {
		stripPrefix = &dynamic.StripPrefix{
			Prefixes: []string{*httpRoute.Rewrite.StripPrefix},
		}
	}

	selectionLabels := map[string]string{ownedByLabelKey: exposition.Name}
	maps.Insert(selectionLabels, maps.All(util.K8sCesServiceDiscoveryLabels))

	return &traefikapi.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rewrite", httpRoute.Name),
			Namespace: exposition.Namespace,
			Labels:    selectionLabels,
		},
		Spec: traefikapi.MiddlewareSpec{
			ReplacePathRegex: replacePathRegex,
			StripPrefix:      stripPrefix,
		},
	}
}

// upsertIngresses applies desiredState by creating or updating each Ingress, then deletes
// any existing owned Ingresses that are absent from desiredState. All errors are collected.
func (t *TraefikIngressController) upsertIngresses(ctx context.Context, exposition types.Exposition, desiredState []*networkingv1.Ingress) error {
	var errs []error
	existing := &networkingv1.IngressList{}
	err := t.Client.List(ctx, existing, &client.ListOptions{Namespace: exposition.Namespace, LabelSelector: selectorFromExpositionName(exposition.Name)})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to list existing ingresses: %w", err))
	}

	existingMap := make(map[string]networkingv1.Ingress, len(existing.Items))
	for _, existingObject := range existing.Items {
		existingMap[existingObject.Name] = existingObject
	}

	for _, desiredObject := range desiredState {
		updateRef := &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: desiredObject.Name, Namespace: desiredObject.Namespace}}
		// only keep track of those that are not in the desired state to delete later
		delete(existingMap, desiredObject.Name)

		_, err := controllerutil.CreateOrUpdate(ctx, t.Client, updateRef, func() error {
			updateRef.Annotations = desiredObject.Annotations
			updateRef.OwnerReferences = desiredObject.OwnerReferences
			updateRef.Labels = desiredObject.Labels
			updateRef.Spec = desiredObject.Spec
			return nil
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create or update ingress %q: %w", desiredObject.Name, err))
		}
	}

	// delete objects not in desired state
	for _, existingObject := range existingMap {
		err := t.Client.Delete(ctx, &existingObject)
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete outdated ingress %q: %w", existingObject.Name, err))
		}
	}

	return errors.Join(errs...)
}

// upsertMiddlewares applies desiredState by creating or updating each Middleware, then
// deletes any existing owned Middlewares absent from desiredState. All errors are collected.
func (t *TraefikIngressController) upsertMiddlewares(ctx context.Context, exposition types.Exposition, desiredState []*traefikapi.Middleware) error {
	var errs []error
	existing := &traefikapi.MiddlewareList{}
	err := t.Client.List(ctx, existing, &client.ListOptions{Namespace: exposition.Namespace, LabelSelector: selectorFromExpositionName(exposition.Name)})
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to list existing middlewares: %w", err))
	}

	existingMap := make(map[string]traefikapi.Middleware, len(existing.Items))
	for _, existingObject := range existing.Items {
		existingMap[existingObject.Name] = existingObject
	}

	for _, desiredObject := range desiredState {
		updateRef := &traefikapi.Middleware{ObjectMeta: metav1.ObjectMeta{Name: desiredObject.Name, Namespace: desiredObject.Namespace}}
		// only keep track of those that are not in the desired state to delete later
		delete(existingMap, desiredObject.Name)

		_, err := controllerutil.CreateOrUpdate(ctx, t.Client, updateRef, func() error {
			updateRef.Annotations = desiredObject.Annotations
			updateRef.OwnerReferences = desiredObject.OwnerReferences
			updateRef.Labels = desiredObject.Labels
			updateRef.Spec = desiredObject.Spec
			return nil
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create or update middleware %q: %w", desiredObject.Name, err))
		}
	}

	// delete objects not in desired state
	for _, existingObject := range existingMap {
		err := t.Client.Delete(ctx, &existingObject)
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("failed to delete outdated middleware %q: %w", existingObject.Name, err))
		}
	}

	return errors.Join(errs...)
}

func selectorFromExpositionName(expositionName string) labels.Selector {
	return labels.Set{ownedByLabelKey: expositionName}.AsSelector()
}

// createIngressRouteTCP builds an IngressRouteTCP that listens on the Traefik entrypoint
// "tcp-<port>", matches all SNI with HostSNI(`*`), and forwards traffic to the backend service.
func createIngressRouteTCP(name string, namespace string, port types.ExposedPort) *traefikapi.IngressRouteTCP {
	externalPortStr := strconv.Itoa(int(port.RequestedExternalPort))

	selectionLabels := map[string]string{ownedByLabelKey: name}
	maps.Insert(selectionLabels, maps.All(util.K8sCesServiceDiscoveryLabels))

	route := &traefikapi.IngressRouteTCP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-tcp", port.ServiceName, externalPortStr),
			Namespace: namespace,
			Labels:    selectionLabels,
		},
		Spec: traefikapi.IngressRouteTCPSpec{
			EntryPoints: []string{fmt.Sprintf("tcp-%s", externalPortStr)},
			Routes: []traefikapi.RouteTCP{
				{
					Match: "HostSNI(`*`)",
					Services: []traefikapi.ServiceTCP{
						{
							Name:      port.ServiceName,
							Namespace: namespace,
							Port:      intstr.FromInt32(port.ServicePort),
						},
					},
				},
			},
		},
	}

	return route
}

// createIngressRouteUDP builds an IngressRouteUDP that listens on the Traefik entrypoint
// "udp-<port>" and forwards traffic to the backend service.
func createIngressRouteUDP(name string, namespace string, port types.ExposedPort) *traefikapi.IngressRouteUDP {
	externalPortStr := strconv.Itoa(int(port.RequestedExternalPort))

	selectionLabels := map[string]string{ownedByLabelKey: name}
	maps.Insert(selectionLabels, maps.All(util.K8sCesServiceDiscoveryLabels))

	route := &traefikapi.IngressRouteUDP{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s-udp", port.ServiceName, externalPortStr),
			Namespace: namespace,
			Labels:    selectionLabels,
		},
		Spec: traefikapi.IngressRouteUDPSpec{
			EntryPoints: []string{fmt.Sprintf("udp-%s", externalPortStr)},
			Routes: []traefikapi.RouteUDP{
				{
					Services: []traefikapi.ServiceUDP{
						{
							Name:      port.ServiceName,
							Namespace: namespace,
							Port:      intstr.FromInt32(port.ServicePort),
						},
					},
				},
			},
		},
	}

	return route
}
