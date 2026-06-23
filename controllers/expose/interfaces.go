package expose

import (
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
)

type middlewareInterface interface {
	traefikv1alpha1.MiddlewareInterface
}

type traefikInterface interface {
	traefikv1alpha1.TraefikV1alpha1Interface
}
