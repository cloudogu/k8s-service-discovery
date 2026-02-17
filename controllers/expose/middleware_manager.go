package expose

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type MiddlewareManager struct {
	client    traefikv1alpha1.MiddlewareInterface
	namespace string
}

func NewMiddlewareManager(traefikClient traefikv1alpha1.TraefikV1alpha1Interface, namespace string) *MiddlewareManager {
	return &MiddlewareManager{
		client:    traefikClient.Middlewares(namespace),
		namespace: namespace,
	}
}

// createOrUpdateReplacePathMiddleware creates or updates a Traefik Middleware CR for path replacement
func (m *MiddlewareManager) createOrUpdateReplacePathMiddleware(ctx context.Context, serviceName string, cesService CesService, ownerReferences []v1.OwnerReference) (string, error) {
	middlewareName := fmt.Sprintf("%s-%s-rewrite", serviceName, cesService.Name)
	targetPath := path.Join(cesService.Pass, "$2")

	// Create the middleware object using typed API
	middleware := &traefikapi.Middleware{
		ObjectMeta: v1.ObjectMeta{
			Name:            middlewareName,
			Namespace:       m.namespace,
			OwnerReferences: ownerReferences,
		},
		Spec: traefikapi.MiddlewareSpec{
			ReplacePathRegex: &dynamic.ReplacePathRegex{
				Regex:       fmt.Sprintf("^%s(/|$)(.*)", strings.TrimRight(cesService.Location, "/")),
				Replacement: targetPath,
			},
		},
	}

	// Try to get existing middleware
	existing, err := m.client.Get(ctx, middlewareName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new middleware
			ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Creating middleware [%s] for service [%s]", middlewareName, serviceName))
			_, createErr := m.client.Create(ctx, middleware, v1.CreateOptions{})
			if createErr != nil {
				return "", fmt.Errorf("failed to create middleware: %w", createErr)
			}
			return middlewareName, nil
		}
		return "", fmt.Errorf("failed to get middleware: %w", err)
	}

	// Update existing middleware
	middleware.ResourceVersion = existing.ResourceVersion
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating middleware [%s] for service [%s]", middlewareName, serviceName))
	_, updateErr := m.client.Update(ctx, middleware, v1.UpdateOptions{})
	if updateErr != nil {
		return "", fmt.Errorf("failed to update middleware: %w", updateErr)
	}

	return middlewareName, nil
}

// CreateOrUpdateAlternativeFQDNRedirectMiddleware creates or updates a Traefik Middleware CR for redirecting
// alternative FQDNs to the primary FQDN. A single middleware handles all alternative domain names.
func (m *MiddlewareManager) CreateOrUpdateAlternativeFQDNRedirectMiddleware(ctx context.Context, alternativeFQDNs []string, primaryFQDN string, ownerReferences []v1.OwnerReference) (string, error) {
	middlewareName := "alternative-fqdn"

	// Build regex pattern that matches all alternative FQDNs
	// Pattern: ^https?://(alt1\.example\.com|alt2\.example\.com|alt3\.example\.com)(.*)
	escapedFQDNs := make([]string, len(alternativeFQDNs))
	for i, fqdn := range alternativeFQDNs {
		// Escape dots in domain names for regex
		escapedFQDNs[i] = strings.ReplaceAll(fqdn, ".", "\\.")
	}

	regexPattern := fmt.Sprintf("^https?://(%s)(.*)", strings.Join(escapedFQDNs, "|"))
	replacement := fmt.Sprintf("https://%s${2}", primaryFQDN)

	middleware := &traefikapi.Middleware{
		ObjectMeta: v1.ObjectMeta{
			Name:            middlewareName,
			Namespace:       m.namespace,
			OwnerReferences: ownerReferences,
		},
		Spec: traefikapi.MiddlewareSpec{
			RedirectRegex: &dynamic.RedirectRegex{
				Regex:       regexPattern,
				Replacement: replacement,
			},
		},
	}

	// Try to get existing middleware
	existing, err := m.client.Get(ctx, middlewareName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new middleware
			ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Creating alternative FQDN redirect middleware [%s] for FQDNs %v -> %s", middlewareName, alternativeFQDNs, primaryFQDN))
			_, createErr := m.client.Create(ctx, middleware, v1.CreateOptions{})
			if createErr != nil {
				return "", fmt.Errorf("failed to create alternative FQDN redirect middleware: %w", createErr)
			}
			return middlewareName, nil
		}
		return "", fmt.Errorf("failed to get middleware: %w", err)
	}

	// Update existing middleware
	middleware.ResourceVersion = existing.ResourceVersion
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating alternative FQDN redirect middleware [%s] for FQDNs %v -> %s", middlewareName, alternativeFQDNs, primaryFQDN))
	_, updateErr := m.client.Update(ctx, middleware, v1.UpdateOptions{})
	if updateErr != nil {
		return "", fmt.Errorf("failed to update alternative FQDN redirect middleware: %w", updateErr)
	}

	return middlewareName, nil
}
