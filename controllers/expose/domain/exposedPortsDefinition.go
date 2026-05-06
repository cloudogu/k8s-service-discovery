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
	"k8s.io/utils/ptr"
)

const (
	// CesExposedPortsAnnotation can be appended to service with information of exposed ports from dogu descriptors.
	cesExposedPortsAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"
)

type ExposedPortsDefinition struct {
	// BaseName is the name that generated resource names should be based on.
	BaseName string
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
			Protocol:              &protocol,
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
		BaseName: service.Name,
		OwnerReference: metav1.OwnerReference{
			APIVersion:         service.APIVersion,
			Kind:               service.Kind,
			Name:               service.Name,
			UID:                service.UID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
		TcpRoutes: tcpRoutes,
		UdpRoutes: udpRoutes,
	}, errors.Join(errs...)
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
	for _, port := range *cesExposedPorts {
		found := false
		for _, servicePort := range service.Spec.Ports {
			if equalsServicePortExposedPort(servicePort, port) {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid service annotation %q. port %q is not defined in service ports", cesExposedPortsAnnotation, port.Port)
		}
	}

	return *cesExposedPorts, nil
}

func equalsServicePortExposedPort(servicePort corev1.ServicePort, exposedPort util.ExposedPort) bool {
	if !strings.EqualFold(string(servicePort.Protocol), string(exposedPort.Protocol)) {
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
		BaseName: exposition.Name,
		OwnerReference: metav1.OwnerReference{
			APIVersion:         exposition.APIVersion,
			Kind:               exposition.Kind,
			Name:               exposition.Name,
			UID:                exposition.UID,
			Controller:         ptr.To(true),
			BlockOwnerDeletion: ptr.To(true),
		},
		TcpRoutes: nil,
		UdpRoutes: nil,
	}
}
