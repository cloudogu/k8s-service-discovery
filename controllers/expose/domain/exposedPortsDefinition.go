package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// CesExposedPortsAnnotation can be appended to service with information of exposed ports from dogu descriptors.
	cesExposedPortsAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"
)

type ExposedPortsDefinition struct {
	// BaseName is the name that generated resource names should be based on.
	BaseName string
	// Type of the definition, currently either TypeService or TypeExposition.
	Type DefinitionType
	// OwnerReference to be set on the generated resources.
	OwnerReference metav1.OwnerReference
	// TcpRoutes to be exposed.
	TcpRoutes []ExposedPortRoute
	// UdpRoutes to be exposed.
	UdpRoutes []ExposedPortRoute
}

type ExposedPortRoute struct {
	Name                  string
	Service               string
	Port                  int32
	RequestedExternalPort *int32
	Protocol              *string
}

func CreateExposedPortsDefinitionFromService(service *corev1.Service) (ExposedPortsDefinition, error) {
	exposedPorts, err := parseExposedPortsFromService(service)
	if err != nil {
		return ExposedPortsDefinition{}, fmt.Errorf("failed to parse exposed ports from service %q: %w", service.Name, err)
	}

	var tcpRoutes []ExposedPortRoute
	var udpRoutes []ExposedPortRoute
	var errs []error
	for _, exposedPort := range exposedPorts {
		protocol := normalizedProtocol(exposedPort.Protocol)
		route := ExposedPortRoute{
			Name:                  fmt.Sprintf("port-%d-%d", exposedPort.Port, exposedPort.TargetPort),
			Service:               service.Name,
			Port:                  exposedPort.Port,
			RequestedExternalPort: &exposedPort.TargetPort,
		}
		if protocol == util.ProtocolTCP {
			tcpRoutes = append(tcpRoutes, route)
		} else if protocol == util.ProtocolUDP {
			udpRoutes = append(udpRoutes, route)
		} else {
			errs = append(errs, fmt.Errorf("invalid protocol %q for exposed port %d", protocol, exposedPort.Port))
		}
	}

	return ExposedPortsDefinition{
		BaseName:       service.Name,
		Type:           TypeService,
		OwnerReference: ownerReferenceFromObject(service),
		TcpRoutes:      tcpRoutes,
		UdpRoutes:      udpRoutes,
	}, errors.Join(errs...)
}

// ownerReferenceFromObject only supports services and expositions
func ownerReferenceFromObject(obj client.Object) metav1.OwnerReference {
	var apiVersion, kind string
	switch obj.(type) {
	case *corev1.Service:
		apiVersion = corev1.SchemeGroupVersion.String()
		kind = "Service"
	case *expositionv1.Exposition:
		apiVersion = expositionv1.SchemeGroupVersion.String()
		kind = "Exposition"
	}

	return metav1.OwnerReference{
		APIVersion:         apiVersion,
		Kind:               kind,
		Name:               obj.GetName(),
		UID:                obj.GetUID(),
		Controller:         new(true),
		BlockOwnerDeletion: new(true),
	}
}

func normalizedProtocol(protocol string) string {
	if protocol == "" {
		return util.ProtocolTCP
	}

	return strings.ToLower(protocol)
}

func parseExposedPortsFromService(service *corev1.Service) (util.ExposedPorts, error) {
	cesExposedPortsStr, ok := service.Annotations[cesExposedPortsAnnotation]
	if !ok {
		return util.ExposedPorts{}, nil
	}

	cesExposedPorts := &util.ExposedPorts{}

	err := json.Unmarshal([]byte(cesExposedPortsStr), cesExposedPorts)
	if err != nil {
		return util.ExposedPorts{}, fmt.Errorf("failed to unmarshal ces exposed ports annotation %q from service %q: %w", cesExposedPortsAnnotation, service.Name, err)
	}

	// Validate: Ports should be in Service Spec
	var errs []error
	for _, port := range *cesExposedPorts {
		found := false
		for _, servicePort := range service.Spec.Ports {
			if equalsServicePortExposedPort(servicePort, port) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, fmt.Errorf("invalid service annotation %q. port '%d' is not defined in service ports", cesExposedPortsAnnotation, port.Port))
		}
	}

	return *cesExposedPorts, errors.Join(errs...)
}

func equalsServicePortExposedPort(servicePort corev1.ServicePort, exposedPort util.ExposedPort) bool {
	if !strings.EqualFold(string(servicePort.Protocol), normalizedProtocol(exposedPort.Protocol)) {
		return false
	}

	if servicePort.Port != exposedPort.Port {
		return false
	}

	return true
}

func CreateExposedPortsDefinitionFromExposition(exposition *expositionv1.Exposition) ExposedPortsDefinition {
	var tcpRoutes []ExposedPortRoute
	for _, tcpRoute := range exposition.Spec.TCP {
		tcpRoutes = append(tcpRoutes, ExposedPortRoute{
			Name:                  tcpRoute.Name,
			Service:               tcpRoute.Service,
			Port:                  tcpRoute.Port,
			RequestedExternalPort: tcpRoute.RequestedExternalPort,
			Protocol:              tcpRoute.Protocol,
		})
	}

	var udpRoutes []ExposedPortRoute
	for _, udpRoute := range exposition.Spec.UDP {
		udpRoutes = append(udpRoutes, ExposedPortRoute{
			Name:                  udpRoute.Name,
			Service:               udpRoute.Service,
			Port:                  udpRoute.Port,
			RequestedExternalPort: udpRoute.RequestedExternalPort,
			Protocol:              udpRoute.Protocol,
		})
	}

	return ExposedPortsDefinition{
		BaseName:       exposition.Name,
		Type:           TypeExposition,
		OwnerReference: ownerReferenceFromObject(exposition),
		TcpRoutes:      tcpRoutes,
		UdpRoutes:      udpRoutes,
	}
}
