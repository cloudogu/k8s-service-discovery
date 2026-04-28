package definition

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-dogu-operator/v3/controllers/annotation"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// CesServiceAnnotation can be appended to service with information of ces services.
	CesServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-services"
)

type ServiceConverter struct {
	maintenance  maintenanceAdapter
	readyChecker deploymentReadyChecker
}

func NewServiceConverter(maintenanceAdapter maintenanceAdapter, readyChecker deploymentReadyChecker) *ServiceConverter {
	return &ServiceConverter{
		maintenance:  maintenanceAdapter,
		readyChecker: readyChecker,
	}
}

func (c *ServiceConverter) Convert(ctx context.Context, service *corev1.Service) (ExpositionDefinition, error) {
	doguInformation, err := getDoguInformation(ctx, service.ObjectMeta, c.maintenance, c.readyChecker)
	if err != nil {
		return ExpositionDefinition{}, err
	}

	cesServices, err := getCesServices(service)
	if err != nil {
		return ExpositionDefinition{}, fmt.Errorf("failed to get ces services: %w", err)
	}

	additionalAnnotations, err := getAdditionalIngressAnnotations(service)
	if err != nil {
		return ExpositionDefinition{}, fmt.Errorf("failed to get additional ingress additionalAnnotations: %w", err)
	}

	httpRoutes, err := c.getHttpRoutesForService(service, cesServices, additionalAnnotations)
	if err != nil {
		return ExpositionDefinition{}, fmt.Errorf("failed to get http routes: %w", err)
	}

	return ExpositionDefinition{
		BaseName: service.Name,
		Dogu:     doguInformation,
		OwnerReference: metav1.OwnerReference{
			APIVersion: service.APIVersion,
			Kind:       service.Kind,
			Name:       service.Name,
			UID:        service.UID,
		},
		HttpRoutes: httpRoutes,
	}, nil
}

func (c *ServiceConverter) getHttpRoutesForService(service *corev1.Service, cesServices []cesService, additionalAnnotations doguv2.IngressAnnotations) ([]HttpRoute, error) {
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
			targetPath = fmt.Sprintf("%s(/|$)(.*)", strings.TrimRight(cesService.Location, "/"))
			rewrite = &HttpRewrite{Regex: &RegexReplacement{Replacement: path.Join(cesService.Pass, "$2"), Pattern: targetPath}}
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
