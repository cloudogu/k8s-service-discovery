package traefik

import (
	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
)

const (
	ingressRewriteTargetAnnotation = "traefik.ingress.kubernetes.io/router.middlewares"
	ingressUseRegexAnnotation      = "nginx.ingress.kubernetes.io/use-regex"

	IngressControllerName = "traefik"
	GatewayControllerName = "k8s-ces-gateway"

	componentLabelKey = "k8s.cloudogu.com/component.name"
)

type controllerType uint8

func (c controllerType) String() string {
	switch c {
	case gateway:
		return GatewayControllerName
	case ingress:
		return IngressControllerName
	}

	return "unknown"
}

const (
	gateway controllerType = iota
	ingress
)

var selectorMap = map[controllerType]map[string]string{
	gateway: {componentLabelKey: GatewayControllerName},
	ingress: {k8sv2.DoguLabelName: IngressControllerName},
}

type IngressController struct {
	controllerType
	*PortExposer
	*IngressRedirector
}

type IngressControllerDependencies struct {
	ConfigMapInterface configMapInterface
	IngressInterface   ingressInterface
	IngressClassName   string
	ControllerType     string
}

func NewTraefikController(deps IngressControllerDependencies) *IngressController {
	return &IngressController{
		PortExposer: &PortExposer{
			configMapInterface: deps.ConfigMapInterface,
		},
		IngressRedirector: &IngressRedirector{
			ingressClassName: deps.IngressClassName,
			ingressInterface: deps.IngressInterface,
		},
		controllerType: mapStringToControllerType(deps.ControllerType),
	}
}

func (c *IngressController) GetName() string {
	return c.String()
}

func (c *IngressController) GetRewriteAnnotationKey() string {
	return ingressRewriteTargetAnnotation
}

func (c *IngressController) GetUseRegexKey() string {
	return ingressUseRegexAnnotation
}

func (c *IngressController) GetSelector() map[string]string {
	return selectorMap[c.controllerType]
}

func mapStringToControllerType(s string) controllerType {
	switch s {
	case GatewayControllerName:
		return gateway
	case IngressControllerName:
		return ingress
	default:
		return 0
	}
}
