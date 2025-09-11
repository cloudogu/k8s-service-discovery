package types

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	exposedPortServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"
)

type ServiceExposedPortDTO struct {
	Protocol   string `json:"protocol"`
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
}

type Service corev1.Service

func (s Service) HasExposedPorts() bool {
	if _, ok := s.GetAnnotations()[exposedPortServiceAnnotation]; !ok {
		return false
	}

	return true
}

func (s Service) GetExposedPorts() (ExposedPorts, error) {
	var svcExposedPorts []ServiceExposedPortDTO

	if !s.HasExposedPorts() {
		return ExposedPorts{}, nil
	}

	exposedPortAnnotation := s.GetAnnotations()[exposedPortServiceAnnotation]

	if uErr := json.Unmarshal([]byte(exposedPortAnnotation), &svcExposedPorts); uErr != nil {
		return nil, fmt.Errorf("failed to unmarshal exposed ports: %w", uErr)
	}

	exposedPorts := make(ExposedPorts, 0, len(svcExposedPorts))

	for _, port := range svcExposedPorts {
		exposedPort, mErr := mapServiceExposedPort(s.Name, port)
		if mErr != nil {
			return nil, fmt.Errorf("failed map port %d from service %s: %w", port.Port, s.Name, mErr)
		}

		exposedPorts = append(exposedPorts, exposedPort)
	}

	exposedPorts.SortByName()

	return exposedPorts, nil
}

func mapServiceExposedPort(svcName string, svcPort ServiceExposedPortDTO) (ExposedPort, error) {
	exPort, err := mapPortInt(svcPort.Port)
	if err != nil {
		return ExposedPort{}, fmt.Errorf("port is invalid: %w", err)
	}

	exTargetPort, err := mapPortInt(svcPort.TargetPort)
	if err != nil {
		return ExposedPort{}, fmt.Errorf("targetPort is invalid: %w", err)
	}

	var protocol corev1.Protocol
	switch corev1.Protocol(strings.ToUpper(svcPort.Protocol)) {
	case corev1.ProtocolTCP:
		protocol = corev1.ProtocolTCP
	case corev1.ProtocolUDP:
		protocol = corev1.ProtocolUDP
	case corev1.ProtocolSCTP:
		protocol = corev1.ProtocolSCTP
	default:
		return ExposedPort{}, fmt.Errorf("unsupported protocol for exposed port: %s", svcPort.Protocol)
	}

	return ExposedPort{
		Name:        fmt.Sprintf("%s-%d", svcName, svcPort.Port),
		ServiceName: svcName,
		Protocol:    protocol,
		Port:        exPort,
		TargetPort:  exTargetPort,
	}, nil
}

func mapPortInt(i int) (int32, error) {
	if i < 0 {
		return 0, fmt.Errorf("number is negative")
	}

	if i > math.MaxInt32 {
		return 0, fmt.Errorf("number is > %d", math.MaxInt32)
	}

	return int32(i), nil
}

func ParseService(obj metav1.Object) (Service, bool) {
	doguService, ok := obj.(*corev1.Service)
	if !ok {
		return Service{}, false
	}

	if doguService.Spec.Type != corev1.ServiceTypeClusterIP {
		return Service{}, false
	}

	labels := doguService.GetLabels()
	if len(labels) == 0 {
		return Service{}, false
	}

	_, ok = labels[k8sv2.DoguLabelName]
	if !ok {
		return Service{}, false
	}

	// Ensure Annotations are set
	ann := doguService.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
		doguService.SetAnnotations(ann)
	}

	return Service(*doguService), true
}
