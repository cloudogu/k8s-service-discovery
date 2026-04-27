package definition

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type ExpositionDefinition struct {
	// BaseName is the name that generated resource names should be based on.
	BaseName string
	// Dogu contains information only relevant to dogus.
	Dogu *DoguInformation
	// OwnerReference to be set on the generated resources.
	OwnerReference metav1.OwnerReference
	// HttpRoutes to be exposed.
	HttpRoutes []HttpRoute
}

type DoguInformation struct {
	IsMaintenanceMode bool
	IsStarting        bool
}

type HttpRoute struct {
	// Name of the route.
	Name string
	// Service name of the route.
	Service string
	// Port of the service.
	Port int32
	// Path under which the application should be exposed.
	Path string
	// Rewrite that should be applied to the ingress configuration.
	Rewrite *HttpRewrite
	// AdditionalAnnotations that should be added to the ingress.
	AdditionalAnnotations map[string]string
}

type HttpRewrite struct {
	StripPrefix *string
	Regex       *RegexReplacement
}

type RegexReplacement struct {
	Pattern     string
	Replacement string
}
