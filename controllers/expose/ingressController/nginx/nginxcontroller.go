package nginx

const (
	ingressRewriteTargetAnnotation = "nginx.ingress.kubernetes.io/rewrite-target"
	ingressUseRegexAnnotation      = "nginx.ingress.kubernetes.io/use-regex"
	nginxIngressControllerSpec     = "k8s.io/nginx-ingress"
	nginxIngressControllerName     = "nginx-ingress"
)

type IngressController struct {
	*ingressNginxTcpUpdExposer
}

func NewNginxController(configMapInterface configMapInterface) *IngressController {
	return &IngressController{
		ingressNginxTcpUpdExposer: NewIngressNginxTCPUDPExposer(configMapInterface),
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
