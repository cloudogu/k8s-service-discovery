package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/dogustart"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
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

const (
	ingressCreationEventReason = "Ingress creation"
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
	// registry is used to read from the etcd.
	registry registry.Registry
	// Namespace defines the target namespace for the ingress objects.
	namespace string
	// IngressClassName defines the ingress class for the ces services.
	ingressClassName string
	// deploymentReadyChecker checks whether dogu are ready (healthy).
	deploymentReadyChecker DeploymentReadyChecker
	eventRecorder          record.EventRecorder
}

// DeploymentReadyChecker checks the readiness from deployments.
type DeploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}

// NewIngressUpdater creates a new instance responsible for updating ingress objects.
func NewIngressUpdater(client client.Client, registry registry.Registry, namespace string, ingressClassName string, recorder record.EventRecorder) (*ingressUpdater, error) {
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
		registry:               registry,
		namespace:              namespace,
		ingressClassName:       ingressClassName,
		deploymentReadyChecker: deploymentReadyChecker,
		eventRecorder:          recorder,
	}, nil
}

// UpsertIngressForService creates or updates the ingress object of the given service.
func (i *ingressUpdater) UpsertIngressForService(ctx context.Context, service *corev1.Service) error {
	isMaintenanceMode, err := isMaintenanceModeActive(i.registry)
	if err != nil {
		return err
	}

	cesServices, ok, err := i.getCesServices(service)
	if err != nil {
		return fmt.Errorf("failed to get ces services: %w", err)
	}

	if !ok {
		log.FromContext(ctx).Info(fmt.Sprintf("service [%s] has no ports or ces services -> skipping ingress creation", service.Name))
		return nil
	}

	for _, cesService := range cesServices {
		err := i.upsertIngressForCesService(ctx, cesService, service, isMaintenanceMode)
		if err != nil {
			return fmt.Errorf("failed to create ingress object for ces service [%+v]: %w", cesService, err)
		}
	}

	return nil
}

func (i *ingressUpdater) getCesServices(service *corev1.Service) ([]CesService, bool, error) {
	if len(service.Spec.Ports) <= 0 {
		return []CesService{}, false, nil
	}

	cesServicesAnnotation, ok := service.Annotations[CesServiceAnnotation]
	if !ok {
		return []CesService{}, false, nil
	}

	var cesServices []CesService
	err := json.Unmarshal([]byte(cesServicesAnnotation), &cesServices)
	if err != nil {
		return []CesService{}, false, fmt.Errorf("failed to unmarshal ces services: %w", err)
	}

	return cesServices, true, nil
}

func (i *ingressUpdater) upsertIngressForCesService(ctx context.Context, cesService CesService, service *corev1.Service, isMaintenanceMode bool) error {
	if isMaintenanceMode {
		return i.upsertMaintenanceModeIngressObject(ctx, cesService, service)
	}

	if hasDoguLabel(service) {
		isReady, err := i.deploymentReadyChecker.IsReady(ctx, service.Name)
		if err != nil {
			return err
		}

		if !isReady {
			return i.upsertDoguIsStartingIngressObject(ctx, cesService, service)
		}
	}

	err := i.upsertDoguIngressObject(ctx, cesService, service)
	if err != nil {
		return err
	}
	// TODO Event New regular Ingress-Object
	i.eventRecorder.Eventf(nil, corev1.EventTypeNormal, ingressCreationEventReason, "Ingress for service [%s] created.", cesService.Name)

	return i.upsertDoguIngressObject(ctx, cesService, service)
}

func (i *ingressUpdater) upsertMaintenanceModeIngressObject(ctx context.Context, cesService CesService, service *corev1.Service) error {
	log.FromContext(ctx).Info(fmt.Sprintf("system is in maintenance mode -> create maintenance ingress object for service [%s]", service.GetName()))
	annotations := map[string]string{ingressRewriteTargetAnnotation: staticContentBackendRewrite}

	err := i.upsertIngressObject(ctx, service, cesService, staticContentBackendName, staticContentBackendPort, annotations)
	if err != nil {
		return fmt.Errorf("failed to update ingress object: %w", err)
	}

	return nil
}

func (i *ingressUpdater) upsertDoguIsStartingIngressObject(ctx context.Context, cesService CesService, service *corev1.Service) error {
	log.FromContext(ctx).Info(fmt.Sprintf("dogu is still starting -> create dogu is starting ingress object for service [%s]", service.GetName()))
	annotations := map[string]string{ingressRewriteTargetAnnotation: staticContentDoguIsStartingRewrite}

	err := i.upsertIngressObject(ctx, service, cesService, staticContentBackendName, staticContentBackendPort, annotations)
	if err != nil {
		return fmt.Errorf("failed to update ingress object: %w", err)
	}

	return nil
}

func (i *ingressUpdater) upsertDoguIngressObject(ctx context.Context, cesService CesService, service *corev1.Service) error {
	log.FromContext(ctx).Info(fmt.Sprintf("dogu is ready -> update ces service ingress object for service [%s]", service.GetName()))
	annotations := map[string]string{}

	if cesService.Pass != cesService.Location {
		annotations[ingressRewriteTargetAnnotation] = cesService.Pass
	}

	err := i.upsertIngressObject(ctx, service, cesService, service.GetName(), int32(cesService.Port), annotations)
	if err != nil {
		return fmt.Errorf("failed to update ingress object: %w", err)
	}

	return nil
}

func (i *ingressUpdater) upsertIngressObject(
	ctx context.Context,
	service *corev1.Service,
	cesService CesService,
	endpointName string,
	endpointPort int32,
	annotations map[string]string,
) error {
	pathType := networking.PathTypePrefix
	ingress := &networking.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        cesService.Name,
			Namespace:   i.namespace,
			Annotations: map[string]string{},
		},
	}

	_, err := ctrl.CreateOrUpdate(ctx, i.client, ingress, func() error {
		ingress.Annotations = annotations

		ingress.Spec = networking.IngressSpec{
			IngressClassName: &i.ingressClassName,
			Rules: []networking.IngressRule{{
				IngressRuleValue: networking.IngressRuleValue{
					HTTP: &networking.HTTPIngressRuleValue{
						Paths: []networking.HTTPIngressPath{{Path: cesService.Location,
							PathType: &pathType,
							Backend: networking.IngressBackend{
								Service: &networking.IngressServiceBackend{
									Name: endpointName,
									Port: networking.ServiceBackendPort{
										Number: endpointPort,
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

func isMaintenanceModeActive(r registry.Registry) (bool, error) {
	_, err := r.GlobalConfig().Get(maintenanceModeGlobalKey)
	if registry.IsKeyNotFoundError(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to read the maintenance mode from the registry: %w", err)
	}

	return true, nil
}
