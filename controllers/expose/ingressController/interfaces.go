package ingressController

type IngressController interface {
	GetControllerSpec() string
	GetRewriteAnnotationKey() string
	GetAdditionalConfigurationKey() string
}
