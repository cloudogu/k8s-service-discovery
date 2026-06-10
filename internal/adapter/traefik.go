package adapter

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const traefikMiddlewareAnnotationKey = "traefik.ingress.kubernetes.io/router.middlewares"

const ownedByLabelKey = "k8s-service-discovery.cloudogu.com/owned-by"

const (
	staticContentMaintenanceRewrite    = "maintenance-mode@kubernetescrd"
	staticContentDoguIsStartingRewrite = "dogu-starting@kubernetescrd"
)

type TraefikIngressController struct {
	IngressClass string
	Client       client.Client
}

func (t *TraefikIngressController) GetOwnableTypes() []client.Object {
	return []client.Object{&networkingv1.Ingress{}, &traefikapi.Middleware{}}
}

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
			errs = append(errs, err)

			middlewareRef = fmt.Sprintf("%s-%s@kubernetescrd", exposition.Namespace, middleware.Name)
			middlewares = append(middlewares, middleware)
		}

		ingress := t.generateIngress(exposition, route, middlewareRef)
		err := exposition.SetOwner(ingress)
		errs = append(errs, err)

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
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to delete outdated ingress %q: %w", existingObject.Name, err))
		}
	}

	return errors.Join(errs...)
}

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
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to delete outdated middleware %q: %w", existingObject.Name, err))
		}
	}

	return errors.Join(errs...)
}

func selectorFromExpositionName(expositionName string) labels.Selector {
	return labels.Set{ownedByLabelKey: expositionName}.AsSelector()
}
