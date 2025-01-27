package nginx

const (
	ingressRewriteTargetAnnotation        = "nginx.ingress.kubernetes.io/rewrite-target"
	ingressUseRegexAnotation              = "nginx.ingress.kubernetes.io/use-regex"
	ingressGetProxyBodySizeKey            = "nginx.ingress.kubernetes.io/proxy-body-size"
	ingressConfigurationSnippetAnnotation = "nginx.ingress.kubernetes.io/configuration-snippet"
	nginxIngressControllerSpec            = "k8s.io/nginx-ingress"
	nginxIngressControllerName            = "nginx-ingress"
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

func (c *IngressController) Get() string {
	return ingressRewriteTargetAnnotation
}

func (c *IngressController) GetUseRegexKey() string {
	return ingressUseRegexAnotation
}

func (c *IngressController) GetProxyBodySizeKey() string {
	return ingressGetProxyBodySizeKey
}

func (c *IngressController) GetAdditionalConfigurationKey() string {
	return ingressConfigurationSnippetAnnotation
}
