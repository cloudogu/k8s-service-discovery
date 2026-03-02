package traefik

import (
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/generated/clientset/versioned/typed/traefikio/v1alpha1"
	netv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
)

type ingressInterface interface {
	netv1.IngressInterface
}

type traefikInterface interface {
	traefikv1alpha1.TraefikV1alpha1Interface
}

type ingressrouteTcpInterface interface {
	traefikv1alpha1.IngressRouteTCPInterface
}

type ingressrouteUdpInterface interface {
	traefikv1alpha1.IngressRouteUDPInterface
}

//nolint:unused
//goland:noinspection GoUnusedType
type middlewareInterface interface {
	traefikv1alpha1.MiddlewareInterface
}
