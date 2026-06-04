package types

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ExpositionConfig defines the configuration for discovering exposed ports
// for the load-balancer.
type ExpositionConfig struct {
	// Enabled determines whether the exposition feature is active as a whole.
	// If set to false, no ports other than http / https will be exposed via this controller.
	Enabled bool

	// DiscoverServices enables port discovery via annotated corev1.Service objects.
	// This is a legacy feature and will be deprecated in future versions.
	DiscoverServices bool

	// DiscoverExpositions enables port discovery via the Exposition Custom Resource (CR).
	// This is the recommended way to configure port expositions moving forward.
	DiscoverExpositions bool
}

// HttpRoute represents the routing configuration for HTTP-based traffic
// targeting a specific backend service.
type HttpRoute struct {
	// Name specifies the unique identifier for this route.
	Name string

	// Service is the name of the target Kubernetes Service handling the traffic.
	Service string

	// Port defines the network port of the target Service.
	Port int32

	// Path defines the URL prefix or pattern under which the application is exposed.
	Path string

	// Rewrite specifies optional path modification rules applied to the ingress configuration.
	// If nil, no rewriting is performed.
	Rewrite *HttpRewrite
}

// HttpRewrite defines how HTTP request paths should be modified before
// forwarding them to the backend service.
type HttpRewrite struct {
	// StripPrefix specifies a path prefix that should be removed from the request URL.
	// If nil, no prefix stripping is applied.
	StripPrefix *string

	// Regex defines regular expression replacement rules for advanced URL rewriting.
	// If nil, no regex replacement is performed.
	Regex *RegexReplacement
}

// RegexReplacement configures regular expression-based URL manipulation.
type RegexReplacement struct {
	// Pattern is the regular expression matching the incoming request path.
	Pattern string

	// Replacement is the target string that replaces the matched pattern.
	Replacement string
}

// SetOwnerFunc sets the owner reference on the given Kubernetes object,
// enabling automatic garbage collection via OwnerReferences.
type SetOwnerFunc func(targetObject client.Object) error

// Exposition represents the complete specification for exposing an application's
// HTTP, TCP, and UDP endpoints, including resource ownership.
type Exposition struct {
	// Name is the unique identifier for this exposition configuration.
	Name string

	// Namespace is the Kubernetes namespace in which the exposition is defined.
	Namespace string

	// HttpRoutes lists all HTTP-based routing rules.
	HttpRoutes []HttpRoute

	// TcpRoutes lists all raw TCP port forwarding configurations.
	TcpRoutes ExposedPorts

	// UdpRoutes lists all raw UDP port forwarding configurations.
	UdpRoutes ExposedPorts

	// SetOwner specifies the Kubernetes object that owns the generated resources,
	// enabling automatic garbage collection via OwnerReferences.
	SetOwner SetOwnerFunc
}
