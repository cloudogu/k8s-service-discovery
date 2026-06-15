package controllers

// This file defines the data-transfer types and mapping logic that translate
// a Dogu corev1.Service (its annotations and ports) into the domain
// types.Exposition. It lives in the controllers package so that reconcilers
// other than ServiceReconciler can reuse the mapping.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"strings"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-dogu-operator/v3/controllers/annotation"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	// cesServiceAnnotation can be appended to a service with information about ces services.
	cesServiceAnnotation = annotation.CesServicesAnnotation
	// cesExposedPortsAnnotation can be appended to a service with information about exposed ports from dogu descriptors.
	cesExposedPortsAnnotation = annotation.CesExposedPortAnnotation
)

// mapServiceToExposition builds a domain Exposition from a Dogu Service:
// HTTP routes from the ces-services annotation, TCP/UDP routes from the
// exposed-ports annotation, and a SetOwner that wires the Service as the
// controller of every generated resource.
func mapServiceToExposition(service *corev1.Service, c client.Client) (types.Exposition, error) {
	httpsRoutes, err := mapCesServicesToHttpRoutes(service)
	if err != nil {
		return types.Exposition{}, fmt.Errorf("failed to map ces service to http routes: %w", err)
	}

	exposedPorts, err := mapExposedPorts(service)
	if err != nil {
		return types.Exposition{}, fmt.Errorf("failed to map ces exposed ports: %w", err)
	}

	return types.Exposition{
		Name:       service.Name,
		DoguName:   service.GetLabels()[doguv2.DoguLabelName],
		Namespace:  service.Namespace,
		HttpRoutes: httpsRoutes,
		TcpRoutes:  exposedPorts.MapTCPPorts(),
		UdpRoutes:  exposedPorts.MapUDPPorts(),
		SetOwner: func(targetObject client.Object) error {
			return ctrl.SetControllerReference(service, targetObject, c.Scheme())
		},
		SetCondition: func(ctx context.Context, conditionType string, conditionStatus bool, reason string, msg string) error {
			return nil
		},
	}, nil
}

// cesServiceDTO describes one ces service entry decoded from the
// ces-services annotation on a Dogu Service.
type cesServiceDTO struct {
	// Name of the ces service serving as identifier.
	Name string `json:"name"`
	// Port of the ces service.
	Port int32 `json:"port"`
	// Location of the ces service defining the external path to the service.
	Location string `json:"location"`
	// Pass of the ces service defining the target path inside the service's pod.
	Pass string `json:"pass"`
	// Rewrite that should be applied to the ingress configuration.
	// Is a json-marshalled `serviceRewrite`. Useful if Dogus do not support sub-paths.
	Rewrite string `json:"rewrite,omitempty"`
}

// hasRewriteConfig reports whether the DTO carries a non-empty Rewrite payload.
func (cs cesServiceDTO) hasRewriteConfig() bool {
	return cs.Rewrite != ""
}

// getRewriteConfig decodes the embedded JSON Rewrite payload. Callers must
// check hasRewriteConfig first; an error is returned otherwise.
func (cs cesServiceDTO) getRewriteConfig() (*serviceRewriteDTO, error) {
	if !cs.hasRewriteConfig() {
		return nil, fmt.Errorf("cesService has no rewrite config")
	}

	serviceRewrite := &serviceRewriteDTO{}
	err := json.Unmarshal([]byte(cs.Rewrite), serviceRewrite)
	if err != nil {
		return nil, fmt.Errorf("failed to read service rewrite from ces service: %w", err)
	}

	return serviceRewrite, nil
}

// serviceRewriteDTO carries the regex-based path rewrite payload embedded in
// a cesServiceDTO.Rewrite field.
type serviceRewriteDTO struct {
	// Pattern is the regular expression matched against the incoming request path.
	Pattern string `json:"pattern"`
	// Rewrite is the replacement applied when Pattern matches.
	Rewrite string `json:"rewrite"`
}

// mapCesServicesToHttpRoutes decodes the ces-services annotation on the given
// Service and returns the resulting domain HttpRoutes. An empty slice (no
// error) is returned when the annotation is absent or the Service has no ports.
func mapCesServicesToHttpRoutes(service *corev1.Service) ([]types.HttpRoute, error) {
	if len(service.Spec.Ports) <= 0 {
		return []types.HttpRoute{}, nil
	}

	cesServicesAnnotation, ok := service.Annotations[cesServiceAnnotation]
	if !ok {
		return []types.HttpRoute{}, nil
	}

	var cesServices []cesServiceDTO
	err := json.Unmarshal([]byte(cesServicesAnnotation), &cesServices)
	if err != nil {
		return []types.HttpRoute{}, fmt.Errorf("failed to unmarshal ces services from dogu service annotation: %w", err)
	}

	httpRoutes, err := createHttpRoutes(service.Name, cesServices)
	if err != nil {
		return []types.HttpRoute{}, fmt.Errorf("failed to create http routes: %w", err)
	}

	return httpRoutes, nil
}

// createHttpRoutes builds one HttpRoute per ces service, deriving any path
// rewrite rules from the DTO (explicit Rewrite payload takes precedence over
// an implicit Pass/Location difference). Per-entry errors are accumulated and
// returned as a single joined error; successful entries are still included.
func createHttpRoutes(name string, services []cesServiceDTO) ([]types.HttpRoute, error) {
	httpRoutes := make([]types.HttpRoute, 0, len(services))
	errs := make([]error, 0)

	for _, cesService := range services {
		targetPath := cesService.Location
		var rewrite *types.HttpRewrite
		if cesService.hasRewriteConfig() {
			serviceRewrite, err := cesService.getRewriteConfig()
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to get serviceRewrite config for ces service %q: %w", cesService.Name, err))
				continue
			}

			targetPath = serviceRewrite.Pattern
			rewrite = &types.HttpRewrite{Regex: &types.RegexReplacement{Replacement: serviceRewrite.Rewrite, Pattern: targetPath}}
		} else if cesService.Pass != cesService.Location {
			pattern := fmt.Sprintf("%s(/|$)(.*)", strings.TrimRight(cesService.Location, "/"))
			rewrite = &types.HttpRewrite{Regex: &types.RegexReplacement{Replacement: path.Join(cesService.Pass, "$2"), Pattern: pattern}}
		}

		httpRoutes = append(httpRoutes, types.HttpRoute{
			Name:    fmt.Sprintf("%s-%d", name, cesService.Port),
			Service: cesService.Name,
			Port:    cesService.Port,
			Path:    targetPath,
			Rewrite: rewrite,
		})
	}

	return httpRoutes, errors.Join(errs...)
}

// ServiceExposedPortDTO describes one exposed port decoded from the
// exposed-ports annotation on a Dogu Service.
type ServiceExposedPortDTO struct {
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
}

// mapExposedPorts decodes the exposed-ports annotation on the Service,
// converts each entry to an ExposedPort, and validates the result against
// the Service's declared corev1.ServicePort list. An empty slice (no error)
// is returned when the annotation is absent.
func mapExposedPorts(service *corev1.Service) (types.ExposedPorts, error) {
	serviceAnnotations := service.GetAnnotations()
	if serviceAnnotations == nil {
		return types.ExposedPorts{}, nil
	}

	exposedPortAnnotation, ok := serviceAnnotations[cesExposedPortsAnnotation]
	if !ok {
		return types.ExposedPorts{}, nil
	}

	var svcExposedPorts []ServiceExposedPortDTO
	if uErr := json.Unmarshal([]byte(exposedPortAnnotation), &svcExposedPorts); uErr != nil {
		return nil, fmt.Errorf("failed to unmarshal exposed ports: %w", uErr)
	}

	exposedPorts, err := mapExposedPortList(service.Name, svcExposedPorts)
	if err != nil {
		return nil, fmt.Errorf("failed to map export ports list from service: %w", err)
	}

	if vErr := validateExposedPorts(exposedPorts, service.Spec.Ports); vErr != nil {
		return nil, fmt.Errorf("failed to validate exposed port annotation against service ports: %w", vErr)
	}

	return exposedPorts, nil
}

// mapExposedPortList converts each ServiceExposedPortDTO to an ExposedPort.
// Per-entry mapping failures are accumulated into a joined error; successful
// entries are still returned.
func mapExposedPortList(svcName string, svcExposedPorts []ServiceExposedPortDTO) (types.ExposedPorts, error) {
	exposedPorts := make(types.ExposedPorts, 0, len(svcExposedPorts))
	mapErrs := make([]error, 0, len(svcExposedPorts))

	for _, port := range svcExposedPorts {
		exposedPort, mErr := mapServiceExposedPort(svcName, port)
		if mErr != nil {
			mapErrs = append(mapErrs, fmt.Errorf("failed to map exposed port DTO - protocol: %s, port: %d, targetPort: %d: %w", port.Protocol, port.Port, port.TargetPort, mErr))
			continue
		}

		exposedPorts = append(exposedPorts, exposedPort)
	}

	return exposedPorts, errors.Join(mapErrs...)
}

// mapServiceExposedPort validates and converts a single ServiceExposedPortDTO
// (decoded from JSON) into an ExposedPort.
//
//   - Ensures Port and TargetPort fit into int32 (via mapPortInt).
//   - Normalizes protocol to upper-case and requires TCP or UDP.
//   - Assigns a synthetic Name of the form "port-<port>-<targetPort>".
//
// Note: the DTO and the domain model name fields differently. The DTO's
// Port (the external port requested by the Dogu) is stored as
// RequestedExternalPort, and the DTO's TargetPort (the in-cluster Service
// port) is stored as ServicePort.
//
// Returns an error if any field is invalid or the protocol is unsupported.
func mapServiceExposedPort(svcName string, svcPort ServiceExposedPortDTO) (types.ExposedPort, error) {
	exPort, err := mapPortInt(svcPort.Port)
	if err != nil {
		return types.ExposedPort{}, fmt.Errorf("port is invalid: %w", err)
	}

	exTargetPort, err := mapPortInt(svcPort.TargetPort)
	if err != nil {
		return types.ExposedPort{}, fmt.Errorf("targetPort is invalid: %w", err)
	}

	var protocol corev1.Protocol
	switch corev1.Protocol(strings.ToUpper(svcPort.Protocol)) {
	case corev1.ProtocolTCP:
		protocol = corev1.ProtocolTCP
	case corev1.ProtocolUDP:
		protocol = corev1.ProtocolUDP
	default:
		return types.ExposedPort{}, fmt.Errorf("unsupported protocol for exposed port: %s", svcPort.Protocol)
	}

	return types.ExposedPort{
		Name:                  fmt.Sprintf("%s-expose-%d-%d", svcName, svcPort.Port, svcPort.TargetPort),
		ServiceName:           svcName,
		Protocol:              protocol,
		RequestedExternalPort: exPort,
		ServicePort:           exTargetPort,
	}, nil
}

// mapPortInt safely converts an int (decoded from JSON) into an int32.
//
//   - Rejects negative numbers.
//   - Rejects numbers greater than math.MaxInt32.
//
// Returns an error on invalid input.
func mapPortInt(i int) (int32, error) {
	if i < 0 {
		return 0, fmt.Errorf("number is negative")
	}

	if i > math.MaxInt32 {
		return 0, fmt.Errorf("number is > %d", math.MaxInt32)
	}

	return int32(i), nil
}

// validateExposedPorts returns a joined error listing every ExposedPort that
// has no matching corev1.ServicePort entry on the underlying Service.
func validateExposedPorts(exposedPorts types.ExposedPorts, servicePorts []corev1.ServicePort) error {
	vErrs := make([]error, 0, len(exposedPorts))

	for _, port := range exposedPorts {
		found := false
		for _, servicePort := range servicePorts {
			if equalsServicePortExposedPort(servicePort, port) {
				found = true
				break
			}
		}
		if !found {
			vErrs = append(vErrs, fmt.Errorf("port '%d' is not defined in service ports", port.ServicePort))
		}
	}

	return errors.Join(vErrs...)
}

// equalsServicePortExposedPort reports whether a corev1.ServicePort matches an
// ExposedPort by normalized protocol and port number.
func equalsServicePortExposedPort(servicePort corev1.ServicePort, exposedPort types.ExposedPort) bool {
	if normalizedProtocol(servicePort.Protocol) != normalizedProtocol(exposedPort.Protocol) {
		return false
	}

	if servicePort.Port != exposedPort.ServicePort {
		return false
	}

	return true
}

// normalizedProtocol defaults an unset protocol to TCP, matching Kubernetes'
// own behavior for corev1.ServicePort.
func normalizedProtocol(protocol corev1.Protocol) corev1.Protocol {
	if protocol == "" {
		return corev1.ProtocolTCP
	}

	return protocol
}

// doguServicePredicate is a controller-runtime predicate that only admits
// Services that pass isDoguService.
func doguServicePredicate() predicate.Funcs {
	return predicate.NewPredicateFuncs(func(object client.Object) bool {
		return isDoguService(object)
	})
}

// isDoguService reports whether the object is a Dogu Service: a ClusterIP
// corev1.Service carrying the doguv2.DoguLabelName label.
func isDoguService(object client.Object) bool {
	doguService, ok := object.(*corev1.Service)
	if !ok {
		return false
	}

	if doguService.Spec.Type != corev1.ServiceTypeClusterIP {
		return false
	}

	serviceLabels := doguService.GetLabels()
	if len(serviceLabels) == 0 {
		return false
	}

	_, ok = serviceLabels[doguv2.DoguLabelName]
	if !ok {
		return false
	}

	return true
}
