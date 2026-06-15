package adapter

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	IngressesConditionType = "IngressesReady"

	ingressesCreatedConditionReason  = "Created"
	ingressesCreatedConditionMessage = "Normal ingresses have been created."

	ingressesMaintenanceModeActiveConditionReason  = "MaintenanceMode"
	ingressesMaintenanceModeActiveConditionMessage = "Ingresses for maintenance mode have been created."

	ingressesDoguStoppedConditionReason  = "DoguStopped"
	ingressesDoguStoppedConditionMessage = "Ingresses for stopped dogu have been handled."

	ingressesDoguStartingConditionReason  = "DoguStarting"
	ingressesDoguStartingConditionMessage = "Ingresses for starting dogu have been created."

	ingressesGetStatusFailedConditionReason       = "GetStatusFailed"
	ingressesCreateOrUpdateFailedConditionReason  = "CreateOrUpdateFailed"
	ingressesMaintenanceModeFailedConditionReason = "MaintenanceModeFailed"
	ingressesDoguStoppedFailedConditionReason     = "DoguStoppedFailed"
	ingressesDoguStartingFailedConditionReason    = "DoguStartingFailed"
)

// Service name and port of the static-content backend that serves
// the maintenance page and the "dogu is starting" splash page.
const (
	staticContentBackendName = "k8s-ces-assets-service"
	staticContentBackendPort = 80
)

// doguAdapter reports the operational state of the dogu whose
// ingress is being reconciled.
type doguAdapter interface {
	GetStatus(ctx context.Context, namespace string, doguName string) (types.ApplicationState, error)
}

// maintenanceAdapter reports the global ecosystem-wide maintenance
// flag, sourced from the maintenance ConfigMap.
type maintenanceAdapter interface {
	GetStatus(ctx context.Context) (repository.MaintenanceModeDescription, bool, error)
}

// ingressController is the technology-specific backend (Traefik,
// nginx, …) that materializes ingress objects from the domain
// Exposition for a given application state.
type ingressController interface {
	GetOwnableTypes() []client.Object
	ProcessExposition(ctx context.Context, appState types.ApplicationState, exposition types.Exposition) error
}

// Ingress is the ExpositionService processor that decides whether
// traffic should hit the application's services or the static-content
// backend, then delegates the actual ingress materialization to an
// ingressController.
type Ingress struct {
	Dogu        doguAdapter
	Maintenance maintenanceAdapter
	Controller  ingressController
}

// GetOwnableTypes delegates to the underlying ingressController so
// that the ExpositionService can register every ingress-related
// Kubernetes type the controller may produce.
func (i Ingress) GetOwnableTypes() []client.Object {
	return i.Controller.GetOwnableTypes()
}

// ProcessExposition reads the dogu's application state and the global
// maintenance flag, rewrites the HTTP routes to point at the
// static-content backend when the dogu is starting or maintenance is
// active, and forwards the (possibly modified) Exposition to the
// ingressController.
func (i Ingress) ProcessExposition(ctx context.Context, exposition types.Exposition) error {
	applicationState, err := i.getApplicationState(ctx, exposition)
	if err != nil {
		return err
	}

	if applicationState == types.ApplicationIsStarting || applicationState == types.ApplicationMaintenance {
		exposition.HttpRoutes = i.redirectHttpRoutesToStaticBackend(exposition.HttpRoutes)
	}

	if lErr := i.Controller.ProcessExposition(ctx, applicationState, exposition); lErr != nil {
		return handleErrorCondition(ctx, exposition,
			IngressesConditionType, appStateToIngressConditionErrorReason(applicationState),
			fmt.Errorf("failed to process exposition from %T while dogu is in state %s: %w", i.Controller, applicationState, lErr))
	}

	conditionReason, conditionMessage := appStateToIngressConditionReasonMessage(applicationState)
	return exposition.SetCondition(ctx, IngressesConditionType, true, conditionReason, conditionMessage)
}

func (i Ingress) getApplicationState(ctx context.Context, exposition types.Exposition) (types.ApplicationState, error) {
	if exposition.DoguName == "" {
		return types.ApplicationRunning, nil
	}

	applicationState, err := i.Dogu.GetStatus(ctx, exposition.Namespace, exposition.Name)
	if err != nil {
		return 0, handleErrorCondition(ctx, exposition,
			IngressesConditionType, ingressesGetStatusFailedConditionReason,
			fmt.Errorf("failed to get status of dogu: %w", err))
	}

	_, maintenanceMode, err := i.Maintenance.GetStatus(ctx)
	if err != nil {
		return 0, handleErrorCondition(ctx, exposition,
			IngressesConditionType, ingressesGetStatusFailedConditionReason,
			fmt.Errorf("failed to get status of global maintenance mode: %w", err))
	}

	if maintenanceMode {
		applicationState = types.ApplicationMaintenance
	}

	return applicationState, nil
}

func handleErrorCondition(ctx context.Context, exposition types.Exposition, conditionType, reason string, err error) error {
	conditionErr := exposition.SetCondition(ctx, conditionType, false, reason, err.Error())
	return errors.Join(err, conditionErr)
}

func appStateToIngressConditionErrorReason(state types.ApplicationState) string {
	switch state {
	case types.ApplicationStopped:
		return ingressesDoguStoppedFailedConditionReason
	case types.ApplicationIsStarting:
		return ingressesDoguStartingFailedConditionReason
	case types.ApplicationMaintenance:
		return ingressesMaintenanceModeFailedConditionReason
	default:
		return ingressesCreateOrUpdateFailedConditionReason
	}
}

func appStateToIngressConditionReasonMessage(state types.ApplicationState) (string, string) {
	switch state {
	case types.ApplicationStopped:
		return ingressesDoguStoppedConditionReason, ingressesDoguStoppedConditionMessage
	case types.ApplicationIsStarting:
		return ingressesDoguStartingConditionReason, ingressesDoguStartingConditionMessage
	case types.ApplicationMaintenance:
		return ingressesMaintenanceModeActiveConditionReason, ingressesMaintenanceModeActiveConditionMessage
	default:
		return ingressesCreatedConditionReason, ingressesCreatedConditionMessage
	}
}

// redirectHttpRoutesToStaticBackend returns a copy of httpRoutes in
// which every entry's Service and Port are replaced by the
// static-content backend and any path Rewrite is cleared. Name and
// Path are preserved so the route still matches the same external URL.
func (i Ingress) redirectHttpRoutesToStaticBackend(httpRoutes []types.HttpRoute) []types.HttpRoute {
	redirectedRoutes := make([]types.HttpRoute, 0, len(httpRoutes))

	for _, route := range httpRoutes {
		route.Service = staticContentBackendName
		route.Port = staticContentBackendPort
		route.Rewrite = nil

		redirectedRoutes = append(redirectedRoutes, types.HttpRoute{
			Name:    route.Name,
			Service: staticContentBackendName,
			Port:    staticContentBackendPort,
			Path:    route.Path,
			Rewrite: nil,
		})
	}

	return redirectedRoutes
}
