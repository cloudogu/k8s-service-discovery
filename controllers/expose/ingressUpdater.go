package expose

import (
	"context"
	"encoding/json"
	"fmt"
	doguv2 "github.com/cloudogu/k8s-dogu-operator/v2/api/v2"
	"github.com/cloudogu/k8s-dogu-operator/v3/controllers/annotation"
	"github.com/cloudogu/k8s-service-discovery/controllers/dogustart"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	"github.com/cloudogu/retry-lib/retry"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	staticContentBackendName           = "nginx-static"
	staticContentBackendPort           = 80
	staticContentBackendRewrite        = "/errors/503.html"
	staticContentDoguIsStartingRewrite = "/errors/starting.html"
)

const (
	// CesServiceAnnotation can be appended to service with information of ces services.
	CesServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-services"
)

const (
	ingressCreationEventReason = "IngressCreation"
)
const failedIngressUpdateErrMsg = "failed to update ingress object: %w"

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
	// globalConfig is used to read the global configuration.
	globalConfigRepo GlobalConfigRepository
	// Namespace defines the target namespace for the ingress objects.
	namespace string
	// IngressClassName defines the ingress class for the ces services.
	ingressClassName string
	// deploymentReadyChecker checks whether dogu are ready (healthy).
	deploymentReadyChecker DeploymentReadyChecker
	eventRecorder          eventRecorder
	controller             ingressController
	ingressInterface       ingressInterface
	doguInterface          doguInterface
}

// NewIngressUpdater creates a new instance responsible for updating ingress objects.
func NewIngressUpdater(clientSet clientSetInterface, doguInterface doguInterface, globalConfigRepo GlobalConfigRepository, namespace string, ingressClassName string, recorder eventRecorder, controller ingressController) *ingressUpdater {
	ingressClient := clientSet.NetworkingV1().Ingresses(namespace)

	deploymentReadyChecker := dogustart.NewDeploymentReadyChecker(clientSet, namespace)
	return &ingressUpdater{
		globalConfigRepo:       globalConfigRepo,
		namespace:              namespace,
		ingressClassName:       ingressClassName,
		deploymentReadyChecker: deploymentReadyChecker,
		eventRecorder:          recorder,
		controller:             controller,
		ingressInterface:       ingressClient,
		doguInterface:          doguInterface,
	}
}

// UpsertIngressForService creates or updates the ingress object of the given service.
func (i *ingressUpdater) UpsertIngressForService(ctx context.Context, service *corev1.Service) error {
	isMaintenanceMode, err := util.IsMaintenanceModeActive(ctx, i.globalConfigRepo)
	if err != nil {
		return err
	}

	cesServices, ok, err := i.getCesServices(service)
	if err != nil {
		return fmt.Errorf("failed to get ces services: %w", err)
	}

	if !ok {
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("service [%s] has no ports or ces services -> skipping ingress creation", service.Name))
		return nil
	}

	for _, cesService := range cesServices {
		upsertErr := i.upsertIngressForCesService(ctx, cesService, service, isMaintenanceMode)
		if upsertErr != nil {
			return fmt.Errorf("failed to create ingress object for ces service [%+v]: %w", cesService, upsertErr)
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
	dogu, err := i.doguInterface.Get(ctx, service.Name, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get dogu for service [%s]: %w", service.Name, err)
	}

	if isMaintenanceMode && dogu.Name != staticContentBackendName {
		return i.upsertMaintenanceModeIngressObject(ctx, cesService, service, dogu)
	}

	if util.HasDoguLabel(service) {
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

func getAdditionalIngressAnnotations(doguService *corev1.Service) (doguv2.IngressAnnotations, error) {
	annotations := doguv2.IngressAnnotations(nil)
	annotationsJson, exists := doguService.Annotations[annotation.AdditionalIngressAnnotationsAnnotation]
	if exists {
		err := json.Unmarshal([]byte(annotationsJson), &annotations)
		if err != nil {
			return nil, fmt.Errorf("failed to get addtional ingress annotations from dogu service '%s': %w", doguService.Name, err)
		}
	}

	return annotations, nil
}

func (i *ingressUpdater) upsertMaintenanceModeIngressObject(ctx context.Context, cesService CesService, service *corev1.Service, dogu *doguv2.Dogu) error {
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("system is in maintenance mode -> create maintenance ingress object for service [%s]", service.GetName()))
	annotations := map[string]string{i.controller.GetRewriteAnnotationKey(): staticContentBackendRewrite}

	err := i.upsertIngressObject(ctx, service, cesService, staticContentBackendName, staticContentBackendPort, annotations)
	if err != nil {
		return fmt.Errorf(failedIngressUpdateErrMsg, err)
	}

	i.eventRecorder.Eventf(dogu, corev1.EventTypeNormal, ingressCreationEventReason, "Ingress for service [%s] has been updated to maintenance mode.", cesService.Name)
	return nil
}

func (i *ingressUpdater) upsertDoguIsStartingIngressObject(ctx context.Context, cesService CesService, service *corev1.Service) error {
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("dogu is still starting -> create dogu is starting ingress object for service [%s]", service.GetName()))
	annotations := map[string]string{i.controller.GetRewriteAnnotationKey(): staticContentDoguIsStartingRewrite}

	err := i.upsertIngressObject(ctx, service, cesService, staticContentBackendName, staticContentBackendPort, annotations)
	if err != nil {
		return fmt.Errorf(failedIngressUpdateErrMsg, err)
	}

	return nil
}

func (i *ingressUpdater) upsertDoguIngressObject(ctx context.Context, cesService CesService, service *corev1.Service) error {
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("dogu is ready -> update ces service ingress object for service [%s]", service.GetName()))
	rewriteStr, err := cesService.generateRewriteConfig()
	if err != nil {
		return err
	}

	// This should overwrite the `Accept-Encoding: "gzip"` header that browsers send.
	// Gzipping by dogus is a problem because it prevents the warp menu from being injected.
	encodingOverwrite := "proxy_set_header Accept-Encoding \"identity\";"

	configurationSnippet := encodingOverwrite
	if rewriteStr != "" {
		configurationSnippet = fmt.Sprintf("%s\n%s", encodingOverwrite, rewriteStr)
	}
	annotations := map[string]string{
		i.controller.GetAdditionalConfigurationKey(): configurationSnippet,
	}

	if cesService.Pass != cesService.Location {
		annotations[i.controller.GetRewriteAnnotationKey()] = cesService.Pass
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
		return fmt.Errorf(failedIngressUpdateErrMsg, err)
	}

	return nil
}

func (i *ingressUpdater) upsertIngressObject(ctx context.Context, service *corev1.Service, cesService CesService, endpointName string, endpointPort int32, annotations map[string]string) error {
	ingress := i.getIngress(service.ObjectMeta, service.TypeMeta, cesService, endpointName, endpointPort, annotations)

	err := retry.OnConflict(func() error {
		_, err := i.ingressInterface.Get(ctx, ingress.Name, v1.GetOptions{})

		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		if errors.IsNotFound(err) {
			_, createErr := i.ingressInterface.Create(ctx, ingress, v1.CreateOptions{})
			return createErr
		}

		_, err = i.ingressInterface.Update(ctx, ingress, v1.UpdateOptions{})
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to upsert ingress %s: %w", ingress.Name, err)
	}

	return nil
}

func (i *ingressUpdater) getIngress(ownerObject v1.ObjectMeta, ownerType v1.TypeMeta, cesService CesService, endpointName string, endpointPort int32, annotations map[string]string) *networking.Ingress {
	pathType := networking.PathTypePrefix
	return &networking.Ingress{
		ObjectMeta: v1.ObjectMeta{
			Name:        cesService.Name,
			Namespace:   i.namespace,
			Annotations: annotations,
			Labels:      util.K8sCesServiceDiscoveryLabels,
			OwnerReferences: []v1.OwnerReference{{
				APIVersion: ownerType.APIVersion,
				Kind:       ownerType.Kind,
				Name:       ownerObject.Name,
				UID:        ownerObject.UID,
			}},
		},
		Spec: networking.IngressSpec{
			IngressClassName: &i.ingressClassName,
			Rules: []networking.IngressRule{
				{
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{
							Paths: []networking.HTTPIngressPath{
								{
									Path:     cesService.Location,
									PathType: &pathType,
									Backend: networking.IngressBackend{
										Service: &networking.IngressServiceBackend{
											Name: endpointName,
											Port: networking.ServiceBackendPort{
												Number: endpointPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
