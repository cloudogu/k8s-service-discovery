package domain

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-dogu-operator/v3/controllers/annotation"
	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-registry-lib/repository"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CesServiceAnnotation can be appended to service with information of ces services.
	CesServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-services"
)

type maintenanceAdapter interface {
	GetStatus(ctx context.Context) (repository.MaintenanceModeDescription, bool, error)
}

// deploymentReadyChecker checks the readiness from deployments.
type deploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}

type IngressDefinitionCreator struct {
	Maintenance  maintenanceAdapter
	ReadyChecker deploymentReadyChecker
}

func getDoguInformation(ctx context.Context, meta metav1.ObjectMeta, maintenanceAdapter maintenanceAdapter, readyChecker deploymentReadyChecker) (*DoguInformation, error) {
	doguName, isDogu := meta.Labels[doguv2.DoguLabelName]
	if isDogu {
		_, maintenanceActive, err := maintenanceAdapter.GetStatus(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get maintenance status: %w", err)
		}

		isReady, err := readyChecker.IsReady(ctx, doguName)
		if err != nil {
			return nil, fmt.Errorf("failed to check deployment readiness for %q: %w", doguName, err)
		}

		return &DoguInformation{
			IsMaintenanceMode: maintenanceActive,
			IsStarting:        !isReady,
		}, nil
	}

	return nil, nil
}

func (c *IngressDefinitionCreator) CreateFromExposition(ctx context.Context, exposition *expositionv1.Exposition) (IngressDefinition, error) {
	doguInformation, err := getDoguInformation(ctx, exposition.ObjectMeta, c.Maintenance, c.ReadyChecker)
	if err != nil {
		return IngressDefinition{}, err
	}

	return IngressDefinition{
		BaseName:       exposition.Name,
		Type:           TypeExposition,
		Dogu:           doguInformation,
		OwnerReference: ownerReferenceFromObject(exposition),
		HttpRoutes:     c.getHttpRoutesForExposition(exposition),
	}, nil
}

func (c *IngressDefinitionCreator) getHttpRoutesForExposition(exposition *expositionv1.Exposition) []HttpRoute {
	var httpRoutes []HttpRoute
	for _, httpEntry := range exposition.Spec.HTTP {
		var rewrite *HttpRewrite
		if httpEntry.Rewrite != nil {
			var regex *RegexReplacement
			if httpEntry.Rewrite.Regex != nil {
				regex = &RegexReplacement{
					Pattern:     httpEntry.Rewrite.Regex.Pattern,
					Replacement: httpEntry.Rewrite.Regex.Replacement,
				}
			}

			rewrite = &HttpRewrite{
				StripPrefix: httpEntry.Rewrite.StripPrefix,
				Regex:       regex,
			}
		}

		httpRoutes = append(httpRoutes, HttpRoute{
			Name:    httpEntry.Name,
			Service: httpEntry.Service,
			Port:    httpEntry.Port,
			Path:    httpEntry.Path,
			Rewrite: rewrite,
		})
	}
	return httpRoutes
}

func (c *IngressDefinitionCreator) CreateFromService(ctx context.Context, service *corev1.Service) (IngressDefinition, error) {
	doguInformation, err := getDoguInformation(ctx, service.ObjectMeta, c.Maintenance, c.ReadyChecker)
	if err != nil {
		return IngressDefinition{}, err
	}

	cesServices, err := getCesServices(service)
	if err != nil {
		return IngressDefinition{}, fmt.Errorf("failed to get ces services: %w", err)
	}

	additionalAnnotations, err := getAdditionalIngressAnnotations(service)
	if err != nil {
		return IngressDefinition{}, fmt.Errorf("failed to get additional ingress additionalAnnotations: %w", err)
	}

	httpRoutes, err := c.getHttpRoutesForService(service, cesServices, additionalAnnotations)
	if err != nil {
		return IngressDefinition{}, fmt.Errorf("failed to get http routes: %w", err)
	}

	return IngressDefinition{
		BaseName:       service.Name,
		Type:           TypeService,
		Dogu:           doguInformation,
		OwnerReference: ownerReferenceFromObject(service),
		HttpRoutes:     httpRoutes,
	}, nil
}

func (c *IngressDefinitionCreator) getHttpRoutesForService(service *corev1.Service, cesServices []cesService, additionalAnnotations doguv2.IngressAnnotations) ([]HttpRoute, error) {
	var httpRoutes []HttpRoute
	var errs []error
	for _, cesService := range cesServices {
		targetPath := cesService.Location
		var rewrite *HttpRewrite
		if cesService.hasRewriteConfig() {
			serviceRewrite, err := cesService.getRewriteConfig()
			if err != nil {
				errs = append(errs, fmt.Errorf("failed to get serviceRewrite config for ces service %q: %w", cesService.Name, err))
				continue
			}

			targetPath = serviceRewrite.Pattern
			rewrite = &HttpRewrite{Regex: &RegexReplacement{Replacement: serviceRewrite.Rewrite, Pattern: targetPath}}
		} else if cesService.Pass != cesService.Location {
			pattern := fmt.Sprintf("%s(/|$)(.*)", strings.TrimRight(cesService.Location, "/"))
			rewrite = &HttpRewrite{Regex: &RegexReplacement{Replacement: path.Join(cesService.Pass, "$2"), Pattern: pattern}}
		}

		httpRoutes = append(httpRoutes, HttpRoute{
			Name:                  cesService.Name,
			Service:               service.Name,
			Port:                  cesService.Port,
			Path:                  targetPath,
			Rewrite:               rewrite,
			AdditionalAnnotations: additionalAnnotations,
		})
	}

	return httpRoutes, errors.Join(errs...)
}

// cesService contains information about one exposed ces service.
type cesService struct {
	// Name of the ces service serving as identifier.
	Name string `json:"name"`
	// Port of the ces service.
	Port int32 `json:"port"`
	// Location of the ces service defining the external path to the service.
	Location string `json:"location"`
	// Pass of the ces service defining the target path inside the service's pod.
	Pass string `json:"pass"`
	// Rewrite that should be applied to the ingress configuration.
	// Is a json-marshalled `serviceRewrite`. Useful if Dogus do not support sub-paths.
	Rewrite string `json:"rewrite,omitempty"`
}

func (cs cesService) hasRewriteConfig() bool {
	return cs.Rewrite != ""
}

func (cs cesService) getRewriteConfig() (*serviceRewrite, error) {
	if !cs.hasRewriteConfig() {
		return nil, fmt.Errorf("cesService has no rewrite config")
	}

	serviceRewrite := &serviceRewrite{}
	err := json.Unmarshal([]byte(cs.Rewrite), serviceRewrite)
	if err != nil {
		return nil, fmt.Errorf("failed to read service rewrite from ces service: %w", err)
	}

	return serviceRewrite, nil
}

type serviceRewrite struct {
	Pattern string `json:"pattern"`
	Rewrite string `json:"rewrite"`
}

func getCesServices(service *corev1.Service) ([]cesService, error) {
	if len(service.Spec.Ports) <= 0 {
		return []cesService{}, nil
	}

	cesServicesAnnotation, ok := service.Annotations[CesServiceAnnotation]
	if !ok {
		return []cesService{}, nil
	}

	var cesServices []cesService
	err := json.Unmarshal([]byte(cesServicesAnnotation), &cesServices)
	if err != nil {
		return []cesService{}, fmt.Errorf("failed to unmarshal ces services: %w", err)
	}

	return cesServices, nil
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
