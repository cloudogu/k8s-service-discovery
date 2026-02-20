package ingressController

import (
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/ingressController/traefik"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultIngressController = traefik.GatewayControllerName
)

type Dependencies struct {
	Controller         string
	ConfigMapInterface configMapInterface
	IngressInterface   ingressInterface
	IngressClassName   string
}

func ParseIngressController(deps Dependencies) IngressController {
	switch deps.Controller {
	case traefik.GatewayControllerName:
		return traefik.NewTraefikController(traefik.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
			ControllerType:     traefik.GatewayControllerName,
		})
	case traefik.IngressControllerName:
		return traefik.NewTraefikController(traefik.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
			ControllerType:     traefik.IngressControllerName,
		})
	default:
		ctrl.Log.WithName("k8s-service-discovery.ParseIngressController").Error(fmt.Errorf("could not parse ingress controller %q. using default: %q", deps.Controller, DefaultIngressController), "unknown ingress controller")
		return traefik.NewTraefikController(traefik.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
			ControllerType:     DefaultIngressController,
		})
	}
}
