package ingressController

import (
	"fmt"
	"github.com/cloudogu/k8s-service-discovery/controllers/expose/ingressController/nginx"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	DefaultIngressController = NginxIngressController
	NginxIngressController   = "nginx-ingress"
)

func ParseIngressController(controller string) IngressController {
	switch controller {
	case NginxIngressController:
		return nginx.NewController()
	default:
		ctrl.Log.WithName("k8s-service-discovery.ParseIngressController").Error(fmt.Errorf("could not parse ingress controller %q. using default: %q", controller, DefaultIngressController), "unknown ingress controller")
		return nginx.NewController()
	}
}
