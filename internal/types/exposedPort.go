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

type ExposedPorts []ExposedPort

// SortByName sorts the slice in-place by the Name field.
func (eps ExposedPorts) SortByName() {
	sort.Slice(eps, func(i, j int) bool {
		return eps[i].Name < eps[j].Name
	})
}

func (eps ExposedPorts) ToServicePorts() []corev1.ServicePort {
	srvPorts := make([]corev1.ServicePort, 0, len(eps))

	eps.SortByName()

	for _, ePort := range eps {
		srvPorts = append(srvPorts, ePort.ToServicePort())
	}

	return srvPorts
}

func (eps ExposedPorts) Equals(o ExposedPorts) bool {
	eps.SortByName()
	o.SortByName()

	return slices.Equal(eps, o)
}

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

type ExposedPort struct {
	Name        string
	ServiceName string
	Protocol    corev1.Protocol
	Port        int32
	TargetPort  int32
	nodePort    int32
}

func (ep ExposedPort) ToServicePort() corev1.ServicePort {
	return corev1.ServicePort{
		Name:       ep.Name,
		Protocol:   ep.Protocol,
		Port:       ep.Port,
		TargetPort: intstr.FromInt32(ep.TargetPort),
		NodePort:   ep.nodePort,
	}
}

func (ep ExposedPort) PortString() string {
	return fmt.Sprintf("%d", ep.Port)
}

func CreateDefaultPorts() ExposedPorts {
	return []ExposedPort{
		{
			Name:       "HTTP",
			Protocol:   corev1.ProtocolTCP,
			Port:       httpPort,
			TargetPort: httpPort,
		},
		{
			Name:       "HTTPS",
			Protocol:   corev1.ProtocolTCP,
			Port:       httpsPort,
			TargetPort: httpsPort,
		},
	}
}
