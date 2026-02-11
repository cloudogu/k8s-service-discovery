package expose

import (
	"context"
	"fmt"
	"path"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	middlewareGVR = schema.GroupVersionResource{
		Group:    "traefik.io",
		Version:  "v1alpha1",
		Resource: "middlewares",
	}
)

type middlewareManager struct {
	dynamicClient dynamic.Interface
	namespace     string
}

func newMiddlewareManager(dynamicClient dynamic.Interface, namespace string) *middlewareManager {
	return &middlewareManager{
		dynamicClient: dynamicClient,
		namespace:     namespace,
	}
}

// createOrUpdateReplacePathMiddleware creates or updates a Traefik Middleware CR for path replacement
func (m *middlewareManager) createOrUpdateReplacePathMiddleware(ctx context.Context, serviceName string, cesService CesService, ownerReferences []v1.OwnerReference) (string, error) {
	middlewareName := fmt.Sprintf("%s-%s-rewrite", serviceName, cesService.Name)
	targetPath := path.Join(cesService.Pass, "$2")

	// Create the middleware object
	middleware := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "traefik.io/v1alpha1",
			"kind":       "Middleware",
			"metadata": map[string]interface{}{
				"name":            middlewareName,
				"namespace":       m.namespace,
				"ownerReferences": convertOwnerReferencesToUnstructured(ownerReferences),
			},
			"spec": map[string]interface{}{
				"replacePathRegex": map[string]interface{}{
					"regex":       fmt.Sprintf("^%s(/|$)(.*)", strings.TrimRight(cesService.Location, "/")),
					"replacement": targetPath,
				},
			},
		},
	}

	// Try to get existing middleware
	existing, err := m.dynamicClient.Resource(middlewareGVR).Namespace(m.namespace).Get(ctx, middlewareName, v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new middleware
			ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Creating middleware [%s] for service [%s]", middlewareName, serviceName))
			_, createErr := m.dynamicClient.Resource(middlewareGVR).Namespace(m.namespace).Create(ctx, middleware, v1.CreateOptions{})
			if createErr != nil {
				return "", fmt.Errorf("failed to create middleware: %w", createErr)
			}
			return middlewareName, nil
		}
		return "", fmt.Errorf("failed to get middleware: %w", err)
	}

	// Update existing middleware
	middleware.SetResourceVersion(existing.GetResourceVersion())
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("Updating middleware [%s] for service [%s]", middlewareName, serviceName))
	_, updateErr := m.dynamicClient.Resource(middlewareGVR).Namespace(m.namespace).Update(ctx, middleware, v1.UpdateOptions{})
	if updateErr != nil {
		return "", fmt.Errorf("failed to update middleware: %w", updateErr)
	}

	return middlewareName, nil
}

func convertOwnerReferencesToUnstructured(ownerRefs []v1.OwnerReference) []interface{} {
	result := make([]interface{}, len(ownerRefs))
	for i, ref := range ownerRefs {
		result[i] = map[string]interface{}{
			"apiVersion":         ref.APIVersion,
			"kind":               ref.Kind,
			"name":               ref.Name,
			"uid":                string(ref.UID),
			"controller":         ref.Controller,
			"blockOwnerDeletion": ref.BlockOwnerDeletion,
		}
	}
	return result
}
