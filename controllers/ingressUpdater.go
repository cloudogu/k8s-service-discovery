package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudogu/k8s-service-discovery/controllers/dogustart"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	staticContentBackendName           = "nginx-static"
	staticContentBackendPort           = 80
	staticContentBackendRewrite        = "/errors/503.html"
	staticContentDoguIsStartingRewrite = "/errors/starting.html"
	ingressRewriteTargetAnnotation     = "nginx.ingress.kubernetes.io/rewrite-target"
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

type ingressUpdater struct {
	// client used to communicate with k8s.
	client client.Client
	// Namespace defines the target namespace for the ingress objects.
	namespace string
	// IngressClassName defines the ingress class for the ces services.
	ingressClassName       string
	deploymentReadyChecker DeploymentReadyChecker
	deploymentReadyReactor DeploymentReadyReactor
}

type DeploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}
type DeploymentReadyReactor interface {
	// WaitForReady allows the execution of code when the deployment switches from the not ready state into the ready state.
	WaitForReady(ctx context.Context, deploymentName string, onReady func(ctx context.Context)) error
}

// NewIngressUpdater creates a new instance responsible for updating ingress objects.
func NewIngressUpdater(client client.Client, namespace string, ingressClassName string) (*ingressUpdater, error) {
	restConfig, err := ctrl.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to find cluster config: %w", err)
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create client set: %w", err)
	}

	deploymentReadyChecker := dogustart.NewDeploymentReadyChecker(clientSet, namespace)
	return &ingressUpdater{
		client:                 client,
		namespace:              namespace,
		ingressClassName:       ingressClassName,
		deploymentReadyChecker: deploymentReadyChecker,
		deploymentReadyReactor: deploymentReadyChecker,
	}, nil
}

// UpdateIngressOfService creates or updates the ingress object of the given service.
func (i *ingressUpdater) UpdateIngressOfService(ctx context.Context, service *corev1.Service, isMaintenanceMode bool) error {
	logger := log.FromContext(ctx)

	if len(service.Spec.Ports) <= 0 {
		logger.Info(fmt.Sprintf("service [%s] has no ports -> skipping ingress creation", service.Name))
		return nil
	}

	cesServicesAnnotation, ok := service.Annotations[CesServiceAnnotation]
	if !ok {
		logger.Info(fmt.Sprintf("found no [%s] annotation for [%s] -> creating no ingress resource", CesServiceAnnotation, service.Name))
		return nil
	}

	var cesServices []CesService
	err := json.Unmarshal([]byte(cesServicesAnnotation), &cesServices)
	if err != nil {
		return fmt.Errorf("failed to unmarshal ces services: %w", err)
	}

	for _, cesService := range cesServices {
		err := i.updateServiceIngressObject(ctx, cesService, service, isMaintenanceMode)
		if err != nil {
			return err
		}
	}

	return nil
}

// updateServiceIngressObject upserts a new ingress resource based on the given ces service.
func (i *ingressUpdater) updateServiceIngressObject(ctx context.Context, cesService CesService, service *corev1.Service, isMaintenanceMode bool) error {
	isReady, err := i.deploymentReadyChecker.IsReady(ctx, service.Name)
	if err != nil {
		return err
	}

	err = i.updateIngressObject(ctx, cesService, service, isMaintenanceMode)
	if err != nil {
		return err
	}

	if !isReady {
		go func() {
			err := i.deploymentReadyReactor.WaitForReady(ctx, service.GetName(), func(ctx context.Context) {
				log.FromContext(ctx).Info(fmt.Sprintf("dogu [%s] is ready now -> update ingress object", service.GetName()))
				err = i.updateIngressObject(ctx, cesService, service, isMaintenanceMode)
				if err != nil {
					log.FromContext(ctx).Error(err, fmt.Sprintf("failed to update [%s] ingress object", service.GetName()))
				}
			})
			if err != nil {
				log.FromContext(ctx).Error(err, fmt.Sprintf("failed to execute ingress object watcher -> cannot update the state of the [%s] dogu ingress object", service.GetName()))
			}
		}()
	}

	return nil
}

func (i *ingressUpdater) updateIngressObject(ctx context.Context, cesService CesService, service *corev1.Service, isMaintenanceMode bool) error {
	logger := log.FromContext(ctx)

	isReady, err := i.deploymentReadyChecker.IsReady(ctx, service.Name)
	if err != nil {
		return err
	}

	pathType := networking.PathTypePrefix
	ingress := &networking.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        cesService.Name,
			Namespace:   i.namespace,
			Annotations: map[string]string{},
		},
	}

	_, err = ctrl.CreateOrUpdate(ctx, i.client, ingress, func() error {
		ingress.Annotations = map[string]string{}

		serviceName := service.GetName()
		servicePort := int32(cesService.Port)

		if cesService.Pass != cesService.Location {
			ingress.Annotations[ingressRewriteTargetAnnotation] = cesService.Pass
		}

		if isMaintenanceMode && serviceName != staticContentBackendName {
			logger.Info(fmt.Sprintf("system is in maintenance mode -> create maintenance ingress object for service [%s]", service.GetName()))
			serviceName = staticContentBackendName
			servicePort = staticContentBackendPort
			ingress.Annotations[ingressRewriteTargetAnnotation] = staticContentBackendRewrite
		} else if !isMaintenanceMode && !isReady && serviceName != staticContentBackendName {
			logger.Info(fmt.Sprintf("dogu is still starting -> create dogu is starting ingress object for service [%s]", service.GetName()))
			serviceName = staticContentBackendName
			servicePort = staticContentBackendPort
			ingress.Annotations[ingressRewriteTargetAnnotation] = staticContentDoguIsStartingRewrite
		} else {
			logger.Info(fmt.Sprintf("dogu is ready -> update ces service ingress object for service [%s]", service.GetName()))
		}

		ingress.Spec = networking.IngressSpec{
			IngressClassName: &i.ingressClassName,
			Rules: []networking.IngressRule{{
				IngressRuleValue: networking.IngressRuleValue{
					HTTP: &networking.HTTPIngressRuleValue{
						Paths: []networking.HTTPIngressPath{{Path: cesService.Location,
							PathType: &pathType,
							Backend: networking.IngressBackend{
								Service: &networking.IngressServiceBackend{
									Name: serviceName,
									Port: networking.ServiceBackendPort{
										Number: servicePort,
									},
								}}}}}}}}}

		err := ctrl.SetControllerReference(service, ingress, i.client.Scheme())
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
