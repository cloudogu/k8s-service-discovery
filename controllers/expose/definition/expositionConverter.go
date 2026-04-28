package definition

import (
	"context"
	"fmt"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-registry-lib/repository"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type maintenanceAdapter interface {
	GetStatus(ctx context.Context) (repository.MaintenanceModeDescription, bool, error)
}

// deploymentReadyChecker checks the readiness from deployments.
type deploymentReadyChecker interface {
	// IsReady checks whether the application of the deployment is ready, i.e., contains at least one ready pod.
	IsReady(ctx context.Context, deploymentName string) (bool, error)
}

type ExpositionConverter struct {
	maintenance  maintenanceAdapter
	readyChecker deploymentReadyChecker
}

func NewExpositionConverter(maintenanceAdapter maintenanceAdapter, readyChecker deploymentReadyChecker) *ExpositionConverter {
	return &ExpositionConverter{
		maintenance:  maintenanceAdapter,
		readyChecker: readyChecker,
	}
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

func (c *ExpositionConverter) Convert(ctx context.Context, exposition *expositionv1.Exposition) (ExpositionDefinition, error) {
	doguInformation, err := getDoguInformation(ctx, exposition.ObjectMeta, c.maintenance, c.readyChecker)
	if err != nil {
		return ExpositionDefinition{}, err
	}

	return ExpositionDefinition{
		BaseName: exposition.Name,
		Dogu:     doguInformation,
		OwnerReference: metav1.OwnerReference{
			APIVersion: exposition.APIVersion,
			Kind:       exposition.Kind,
			Name:       exposition.Name,
			UID:        exposition.UID,
		},
		HttpRoutes: c.getHttpRoutesForExposition(exposition),
	}, nil
}

func (c *ExpositionConverter) getHttpRoutesForExposition(exposition *expositionv1.Exposition) []HttpRoute {
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
