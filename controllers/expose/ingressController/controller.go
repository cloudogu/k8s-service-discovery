package ingressController

import (
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/ingressController/nginx"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultIngressController = nginx.GatewayControllerName
)

type Dependencies struct {
	Controller         string
	ConfigMapInterface configMapInterface
	IngressInterface   ingressInterface
	IngressClassName   string
}

func ParseIngressController(deps Dependencies) IngressController {
	switch deps.Controller {
	case nginx.GatewayControllerName:
		return nginx.NewNginxController(nginx.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
			ControllerType:     nginx.GatewayControllerName,
		})
	case nginx.IngressControllerName:
		return nginx.NewNginxController(nginx.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
			ControllerType:     nginx.IngressControllerName,
		})
	default:
		ctrl.Log.WithName("k8s-service-discovery.ParseIngressController").Error(fmt.Errorf("could not parse ingress controller %q. using default: %q", deps.Controller, DefaultIngressController), "unknown ingress controller")
		return nginx.NewNginxController(nginx.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
			ControllerType:     DefaultIngressController,
		})
	}
}
