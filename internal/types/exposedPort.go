package types

import (
	"fmt"
	"slices"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	httpPort  = 80
	httpsPort = 443
)

type indexKey struct {
	name       string
	protocol   string
	port       string
	targetPort string
}

func indexKeyOfServicePort(port corev1.ServicePort) indexKey {
	return indexKey{
		name:       port.Name,
		protocol:   string(port.Protocol),
		port:       fmt.Sprintf("%d", port.Port),
		targetPort: port.TargetPort.String(),
	}
}

func indexKeyOfExposedPort(port ExposedPort) indexKey {
	return indexKey{
		name:       port.Name,
		protocol:   string(port.Protocol),
		port:       fmt.Sprintf("%d", port.Port),
		targetPort: fmt.Sprintf("%d", port.TargetPort),
	}
}

// ExposedPorts is a list of exposed ports
type ExposedPorts []ExposedPort

// SortByName sorts the slice in-place by the Name field.
func (eps ExposedPorts) SortByName() {
	sort.Slice(eps, func(i, j int) bool {
		return eps[i].Name < eps[j].Name
	})
}

// ToServicePorts maps the slice of ExposedPorts to a slice of Kubernetes ServicePort
func (eps ExposedPorts) ToServicePorts() []corev1.ServicePort {
	srvPorts := make([]corev1.ServicePort, 0, len(eps))

	eps.SortByName()

	for _, ePort := range eps {
		srvPorts = append(srvPorts, ePort.ToServicePort())
	}

	return srvPorts
}

// Equals check whether two ExposedPorts slices are equal. For stability, the slices are sorted by the name
// before comparing them.
func (eps ExposedPorts) Equals(o ExposedPorts) bool {
	eps.SortByName()
	o.SortByName()

	return slices.Equal(eps, o)
}

// SetNodePorts populates each ExposedPort.nodePort by looking up the matching
// corev1.ServicePort in the provided slice. T
//
// The comparing logic relies on an index built by name, protocol, port and targetPort.
//
// Notes
//   - NodePort of 0 (unassigned) will be copied as 0.
//   - Unmatching exports ports keep their initial nodePort value.
//   - Protocol is part of the key to avoid TCP/UDP collisions on the same port.
//
// After the call, any ExposedPort that corresponds to a ServicePort will have
// its nodePort field updated to the Service’s NodePort value.
func (eps ExposedPorts) SetNodePorts(servicePorts []corev1.ServicePort) {
	nodePortIndex := make(map[indexKey]int32, len(servicePorts))

	for _, sPort := range servicePorts {
		nodePortIndex[indexKeyOfServicePort(sPort)] = sPort.NodePort
	}

	for i := range eps {
		ep := &eps[i]
		if np, ok := nodePortIndex[indexKeyOfExposedPort(*ep)]; ok {
			ep.nodePort = np
		}
	}
}

// ExposedPort represent an exposed port by a dogu service.
// Fields:
// - Name: name of the port
// - ServiceName: name of the dogu service the port belongs to
// - Protocol: protocol used for the port, usually TCP or UDP
// - Port: Incoming port
// - TargetPort: port within the container/pod
type ExposedPort struct {
	Name        string
	ServiceName string
	Protocol    corev1.Protocol
	Port        int32
	TargetPort  int32
	nodePort    int32
}

// ToServicePort maps the ExposedPort to a Kubernetes ServicePort
func (ep ExposedPort) ToServicePort() corev1.ServicePort {
	return corev1.ServicePort{
		Name:       ep.Name,
		Protocol:   ep.Protocol,
		Port:       ep.Port,
		TargetPort: intstr.FromInt32(ep.TargetPort),
		NodePort:   ep.nodePort,
	}
}

// PortString returns ExposedPort.Port as string.
func (ep ExposedPort) PortString() string {
	return fmt.Sprintf("%d", ep.Port)
}

// CreateDefaultPorts create default exposed ports used for the loadbalancer. They include ports for http as well as
// https.
func CreateDefaultPorts() ExposedPorts {
	return []ExposedPort{
		{
			Name:       "http",
			Protocol:   corev1.ProtocolTCP,
			Port:       httpPort,
			TargetPort: httpPort,
		},
		{
			Name:       "https",
			Protocol:   corev1.ProtocolTCP,
			Port:       httpsPort,
			TargetPort: httpsPort,
		},
	}
}
