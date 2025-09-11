package nginx

import (
	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
)

const (
	ingressRewriteTargetAnnotation = "nginx.ingress.kubernetes.io/rewrite-target"
	ingressUseRegexAnnotation      = "nginx.ingress.kubernetes.io/use-regex"
	nginxIngressControllerSpec     = "k8s.io/nginx-ingress"
	nginxIngressControllerName     = "nginx-ingress"
)

type IngressController struct {
	*PortExposer
	*IngressRedirector
}

type IngressControllerDependencies struct {
	ConfigMapInterface configMapInterface
	IngressInterface   ingressInterface
	IngressClassName   string
}

func NewNginxController(deps IngressControllerDependencies) *IngressController {
	return &IngressController{
		PortExposer: &PortExposer{
			configMapInterface: deps.ConfigMapInterface,
		},
		IngressRedirector: &IngressRedirector{
			ingressClassName: deps.IngressClassName,
			ingressInterface: deps.IngressInterface,
		},
	}
}

func (c *IngressController) GetName() string {
	return nginxIngressControllerName
}

func (c *IngressController) GetControllerSpec() string {
	return nginxIngressControllerSpec
}

func (c *IngressController) GetRewriteAnnotationKey() string {
	return ingressRewriteTargetAnnotation
}

func (c *IngressController) GetUseRegexKey() string {
	return ingressUseRegexAnnotation
}

func (c *IngressController) GetSelector() map[string]string {
	return map[string]string{
		k8sv2.DoguLabelName: nginxIngressControllerName,
	}
}
