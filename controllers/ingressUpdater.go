package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/cloudogu/cesapp-lib/registry"
	doguv1 "github.com/cloudogu/k8s-dogu-operator/api/v1"
	"github.com/cloudogu/k8s-dogu-operator/controllers/annotation"
	"github.com/cloudogu/k8s-service-discovery/controllers/dogustart"
)

const (
	staticContentBackendName              = "nginx-static"
	staticContentBackendPort              = 80
	staticContentBackendRewrite           = "/errors/503.html"
	staticContentDoguIsStartingRewrite    = "/errors/starting.html"
	ingressRewriteTargetAnnotation        = "nginx.ingress.kubernetes.io/rewrite-target"
	ingressConfigurationSnippetAnnotation = "nginx.ingress.kubernetes.io/configuration-snippet"
)

const (
	ingressCreationEventReason = "IngressCreation"
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
	// Rewrite that should be applied to the ingress configuration.
	// Is a json-marshalled `serviceRewrite`. Useful if Dogus do not support sub-paths.
	Rewrite string `json:"rewrite,omitempty"`
}

func (cs CesService) generateRewriteConfig() (string, error) {
	if cs.Rewrite == "" {
		return "", nil
	}

	serviceRewrite := &serviceRewrite{}
	err := json.Unmarshal([]byte(cs.Rewrite), serviceRewrite)
	if err != nil {
		return "", fmt.Errorf("failed to read service rewrite from ces service: %w", err)
	}

	return serviceRewrite.generateConfig(), nil
}

type serviceRewrite struct {
	Pattern string `json:"pattern"`
	Rewrite string `json:"rewrite"`
}

func (sr *serviceRewrite) generateConfig() string {
	return fmt.Sprintf("rewrite ^/%s(/|$)(.*) %s/$2 break;", sr.Pattern, sr.Rewrite)
}

type ingressUpdater struct {
	// client used to communicate with k8s.
	client client.Client
	// globalConfig is used to read the global config from the etcd.
	globalConfig configurationContext
	// Namespace defines the target namespace for the ingress objects.
	namespace string
	// IngressClassName defines the ingress class for the ces services.
	ingressClassName string
	// deploymentReadyChecker checks whether dogu are ready (healthy).
	deploymentReadyChecker DeploymentReadyChecker
	eventRecorder          eventRecorder
}

// DeploymentReadyChecker checks the readiness from deployments.
type DeploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}

type configurationContext interface {
	registry.ConfigurationContext
}

// NewIngressUpdater creates a new instance responsible for updating ingress objects.
func NewIngressUpdater(client client.Client, globalConfig configurationContext, namespace string, ingressClassName string, recorder eventRecorder) (*ingressUpdater, error) {
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
		globalConfig:           globalConfig,
		namespace:              namespace,
		ingressClassName:       ingressClassName,
		deploymentReadyChecker: deploymentReadyChecker,
		eventRecorder:          recorder,
	}, nil
}

// UpsertIngressForService creates or updates the ingress object of the given service.
func (i *ingressUpdater) UpsertIngressForService(ctx context.Context, service *corev1.Service) error {
	isMaintenanceMode, err := isMaintenanceModeActive(i.globalConfig)
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
	namespacedName := types.NamespacedName{Name: service.Name, Namespace: service.Namespace}
	dogu := &doguv1.Dogu{}
	err := i.client.Get(ctx, namespacedName, dogu)
	if err != nil {
		return fmt.Errorf("failed to get dogu for service [%s]: %w", service.Name, err)
	}

	if isMaintenanceMode && dogu.Name != staticContentBackendName {
		return i.upsertMaintenanceModeIngressObject(ctx, cesService, service, dogu)
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

	err = i.upsertDoguIngressObject(ctx, cesService, service)
	if err != nil {
		return err
	}

	i.eventRecorder.Eventf(dogu, corev1.EventTypeNormal, ingressCreationEventReason, "Created regular ingress for service [%s].", cesService.Name)
	return err
}

func getAdditionalIngressAnnotations(doguService *corev1.Service) (doguv1.IngressAnnotations, error) {
	annotations := doguv1.IngressAnnotations(nil)
	annotationsJson, exists := doguService.Annotations[annotation.AdditionalIngressAnnotationsAnnotation]
	if exists {
		err := json.Unmarshal([]byte(annotationsJson), &annotations)
		if err != nil {
			return nil, fmt.Errorf("failed to get addtional ingress annotations from dogu service '%s': %w", doguService.Name, err)
		}
	}

	return annotations, nil
}

func (i *ingressUpdater) upsertMaintenanceModeIngressObject(ctx context.Context, cesService CesService, service *corev1.Service, dogu *doguv1.Dogu) error {
	log.FromContext(ctx).Info(fmt.Sprintf("system is in maintenance mode -> create maintenance ingress object for service [%s]", service.GetName()))
	annotations := map[string]string{ingressRewriteTargetAnnotation: staticContentBackendRewrite}

	err := i.upsertIngressObject(ctx, service, cesService, staticContentBackendName, staticContentBackendPort, annotations)
	if err != nil {
		return fmt.Errorf("failed to update ingress object: %w", err)
	}

	i.eventRecorder.Eventf(dogu, corev1.EventTypeNormal, ingressCreationEventReason, "Ingress for service [%s] has been updated to maintenance mode.", cesService.Name)
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
	serviceRewrite, err := cesService.generateRewriteConfig()
	if err != nil {
		return err
	}

	// This should overwrite the `Accept-Encoding: "gzip"` header that browsers send.
	// Gzipping by dogus is a problem because it prevents the warp menu from being injected.
	encodingOverwrite := "proxy_set_header Accept-Encoding \"identity\";"

	configurationSnippet := fmt.Sprintf("%s", encodingOverwrite)
	if serviceRewrite != "" {
		configurationSnippet = fmt.Sprintf("%s\n%s", encodingOverwrite, serviceRewrite)
	}
	annotations := map[string]string{
		ingressConfigurationSnippetAnnotation: configurationSnippet,
	}

	if cesService.Pass != cesService.Location {
		annotations[ingressRewriteTargetAnnotation] = cesService.Pass
	}

	additionalAnnotations, err := getAdditionalIngressAnnotations(service)
	if err != nil {
		return err
	}

	for key, value := range additionalAnnotations {
		annotations[key] = value
	}

	err = i.upsertIngressObject(ctx, service, cesService, service.GetName(), int32(cesService.Port), annotations)
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

func isMaintenanceModeActive(g configurationContext) (bool, error) {
	_, err := g.Get(maintenanceModeGlobalKey)
	if registry.IsKeyNotFoundError(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to read the maintenance mode from the registry: %w", err)
	}

	return true, nil
}
