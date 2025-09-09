package types

import (
	"encoding/json"
	"fmt"

	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	exposedPortServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"
)

type Service corev1.Service

func (s Service) HasExposedPorts() bool {
	if _, ok := s.GetAnnotations()[exposedPortServiceAnnotation]; !ok {
		return false
	}

	return true
}

func (s Service) GetExposedPorts() (ExposedPorts, error) {
	var exposedPorts ExposedPorts

	if !s.HasExposedPorts() {
		return ExposedPorts{}, nil
	}

	exposedPortAnnotation := s.GetAnnotations()[exposedPortServiceAnnotation]

	if uErr := json.Unmarshal([]byte(exposedPortAnnotation), &exposedPorts); uErr != nil {
		return nil, fmt.Errorf("failed to unmarshal exposed ports: %w", uErr)
	}

	for i, port := range exposedPorts {
		port.Name = fmt.Sprintf("%s-%d", s.Name, port.Port)
		exposedPorts[i] = port
	}

	exposedPorts.SortByName()

	return exposedPorts, nil
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
