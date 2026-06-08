package types

import (
	"fmt"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	corev1 "k8s.io/api/core/v1"
)

type ExpositionOld expositionv1.Exposition

func (e ExpositionOld) HasExposedPorts() bool {
	return len(e.Spec.TCP)+len(e.Spec.UDP) > 0
}

func (e ExpositionOld) GetExposedPorts() (ExposedPorts, error) {
	if !e.HasExposedPorts() {
		return ExposedPorts{}, nil
	}

	exposedPorts := make(ExposedPorts, 0, len(e.Spec.TCP)+len(e.Spec.UDP))
	for _, tcpEntry := range e.Spec.TCP {
		exposedPort := mapTcpEntry(e.Name, tcpEntry)
		exposedPorts = append(exposedPorts, exposedPort)
	}
	for _, udpEntry := range e.Spec.UDP {
		exposedPort := mapUdpEntry(e.Name, udpEntry)
		exposedPorts = append(exposedPorts, exposedPort)
	}

	exposedPorts.SortByName()

	return exposedPorts, nil
}

func mapUdpEntry(expositionName string, entry expositionv1.UDPEntry) ExposedPort {
	externalPort := entry.Port
	if entry.RequestedExternalPort != nil {
		externalPort = *entry.RequestedExternalPort
	}
	return ExposedPort{
		Name:                  fmt.Sprintf("%s-%s", expositionName, entry.Name),
		ServiceName:           entry.Service,
		Protocol:              corev1.ProtocolUDP,
		ServicePort:           entry.Port,
		RequestedExternalPort: externalPort,
	}
}

func mapTcpEntry(expositionName string, entry expositionv1.TCPEntry) ExposedPort {
	externalPort := entry.Port
	if entry.RequestedExternalPort != nil {
		externalPort = *entry.RequestedExternalPort
	}
	return ExposedPort{
		Name:                  fmt.Sprintf("%s-%s", expositionName, entry.Name),
		ServiceName:           entry.Service,
		Protocol:              corev1.ProtocolTCP,
		ServicePort:           entry.Port,
		RequestedExternalPort: externalPort,
	}
}
