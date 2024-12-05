package nginx

const (
	ingressRewriteTargetAnnotation        = "nginx.ingress.kubernetes.io/rewrite-target"
	ingressConfigurationSnippetAnnotation = "nginx.ingress.kubernetes.io/configuration-snippet"
	nginxIngressControllerSpec            = "k8s.io/nginx-ingress"
)

type controller struct{}

func NewController() controller {
	return controller{}
}

func (c controller) GetControllerSpec() string {
	return nginxIngressControllerSpec
}

func (c controller) GetRewriteAnnotationKey() string {
	return ingressRewriteTargetAnnotation
}

func (c controller) GetAdditionalConfigurationKey() string {
	return ingressConfigurationSnippetAnnotation
}
