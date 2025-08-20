package ingressController

import (
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/ingressController/nginx"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultIngressController = NginxIngressController
	NginxIngressController   = "nginx-ingress"
)

type Dependencies struct {
	Controller         string
	ConfigMapInterface configMapInterface
	IngressInterface   ingressInterface
	IngressClassName   string
}

func ParseIngressController(deps Dependencies) IngressController {
	switch deps.Controller {
	case NginxIngressController:
		return nginx.NewNginxController(nginx.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
		})
	default:
		ctrl.Log.WithName("k8s-service-discovery.ParseIngressController").Error(fmt.Errorf("could not parse ingress controller %q. using default: %q", deps.Controller, DefaultIngressController), "unknown ingress controller")
		return nginx.NewNginxController(nginx.IngressControllerDependencies{
			ConfigMapInterface: deps.ConfigMapInterface,
			IngressInterface:   deps.IngressInterface,
			IngressClassName:   deps.IngressClassName,
		})
	}
}
