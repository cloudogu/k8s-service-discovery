package expose

import (
	"fmt"
	"maps"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const traefikMiddlewareAnnotationKey = "traefik.ingress.kubernetes.io/router.middlewares"

type ingressGenerator struct {
	// namespace defines the target namespace for the ingress objects.
	namespace string
	// ingressClassName defines the ingress class for the ces services.
	ingressClassName  string
	eventRecorder     eventRecorder
	controller        ingressController
	middlewareManager middlewareManager
}

func (i *ingressGenerator) GenerateWithMiddlewares(definition *IngressesDefinition) ([]networkingv1.Ingress, []traefikapi.Middleware) {
	if definition.Dogu != nil && definition.Dogu.IsStarting {
		ingresses := i.generateStarting(definition)
		return ingresses, nil
	} else if definition.Dogu != nil && definition.Dogu.IsMaintenanceMode {
		ingresses := i.generateMaintenanceMode(definition)
		return ingresses, nil
	}

	return i.generateNormalWithMiddlewares(definition)
}

func (i *ingressGenerator) generateStarting(definition *IngressesDefinition) []networkingv1.Ingress {
	// TODO
	return []networkingv1.Ingress{}
}

func (i *ingressGenerator) generateMaintenanceMode(definition *IngressesDefinition) []networkingv1.Ingress {
	// TODO
	return []networkingv1.Ingress{}
}

func (i *ingressGenerator) generateNormalWithMiddlewares(definition *IngressesDefinition) ([]networkingv1.Ingress, []traefikapi.Middleware) {
	var ingresses []networkingv1.Ingress
	var middlewares []traefikapi.Middleware
	for _, route := range definition.HttpRoutes {
		var middlewareName string
		if route.Rewrite != nil {
			middleware := i.generateMiddleware(definition.BaseName, definition.OwnerReference, route)
			middlewareName = middleware.Name
			middlewares = append(middlewares, middleware)
		}

		ingress := i.generateIngress(definition.BaseName, middlewareName, definition.OwnerReference, route)
		ingresses = append(ingresses, ingress)
	}

	return ingresses, middlewares
}

func (i *ingressGenerator) generateIngress(baseName, middlewareName string, ownerReference metav1.OwnerReference, httpRoute HttpRoute) networkingv1.Ingress {
	annotations := map[string]string{
		traefikMiddlewareAnnotationKey: fmt.Sprintf("%s-%s@kubernetescrd", i.namespace, middlewareName),
	}
	maps.Insert(annotations, maps.All(httpRoute.AdditionalAnnotations))

	pathType := networkingv1.PathTypePrefix
	return networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s", baseName, httpRoute.Name),
			Namespace:       i.namespace,
			Annotations:     annotations,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels:          util.K8sCesServiceDiscoveryLabels,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &i.ingressClassName,
			Rules: []networkingv1.IngressRule{{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     httpRoute.Path,
							PathType: &pathType,
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

func (i *ingressGenerator) generateMiddleware(baseName string, ownerReference metav1.OwnerReference, httpRoute HttpRoute) traefikapi.Middleware {
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

	return traefikapi.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:            fmt.Sprintf("%s-%s-rewrite", baseName, httpRoute.Name),
			Namespace:       i.namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
		},
		Spec: traefikapi.MiddlewareSpec{
			ReplacePathRegex: replacePathRegex,
			StripPrefix:      stripPrefix,
		},
	}
}
