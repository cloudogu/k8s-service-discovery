package controllers

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	IngressRewriteTargetAnnotation = "nginx.ingress.kubernetes.io/rewrite-target"
)

// CesService contains information about one exposed ces service.
type CesService struct {
	// Name of the ces service serving as identifier.
	Name string `json:"name"`
	// Port of the ces service.
	Port int `json:"port"`
	// Location of the ces service defining the external path to the service.
	Location string `json:"location"`
	// Pass of the ces service defining the target path inside the service's pod.
	Pass string `json:"pass"`
}

// IngressGenerator generates ingress objects based on ces service information.
type IngressGenerator struct {
	// Client used to communicate with k8s.
	Client client.Client `json:"client"`
	// Namespace defines the target namespace for the ingress objects.
	Namespace string `json:"namespace"`
	// IngressClassName defines the ingress class for the ces services.
	IngressClassName string `json:"ingress_class_name"`
}

// NewIngressGenerator create a new ingress generator.
func NewIngressGenerator(client client.Client, namespace string, ingressClassName string) IngressGenerator {
	return IngressGenerator{
		Client:           client,
		Namespace:        namespace,
		IngressClassName: ingressClassName,
	}
}

// CreateCesServiceIngress creates a new ingress resource based on the given ces service
func (g IngressGenerator) CreateCesServiceIngress(ctx context.Context, cesService CesService, service *corev1.Service) error {
	logger := log.FromContext(ctx)
	logger.Info(fmt.Sprintf("create ces service ingress object for service [%s]", service.GetName()))

	pathType := networking.PathTypePrefix
	ingress := &networking.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        cesService.Name,
			Namespace:   g.Namespace,
			Annotations: map[string]string{},
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, g.Client, ingress, func() error {
		ingress.Spec = networking.IngressSpec{
			IngressClassName: &g.IngressClassName,
			Rules: []networking.IngressRule{{
				IngressRuleValue: networking.IngressRuleValue{
					HTTP: &networking.HTTPIngressRuleValue{
						Paths: []networking.HTTPIngressPath{{Path: cesService.Location,
							PathType: &pathType,
							Backend: networking.IngressBackend{
								Service: &networking.IngressServiceBackend{
									Name: service.GetName(),
									Port: networking.ServiceBackendPort{
										Number: int32(cesService.Port),
									},
								}}}}}}}}}

		if cesService.Pass != cesService.Location {
			ingress.Annotations[IngressRewriteTargetAnnotation] = cesService.Pass
		}

		err := ctrl.SetControllerReference(service, ingress, g.Client.Scheme())
		if err != nil {
			return fmt.Errorf("failed to set controller reference for ingress: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create or update ingress object: %w", err)
	}

	return nil
}
