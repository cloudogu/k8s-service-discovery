package adapter

import (
	"context"

	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	staticContentBackendName = "k8s-ces-assets-service"
	staticContentBackendPort = 80
)

type doguAdapter interface {
	GetStatus() (types.ApplicationState, error)
}

type maintenanceAdapter interface {
	GetStatus(ctx context.Context) (repository.MaintenanceModeDescription, bool, error)
}

type ingressController interface {
	GetOwnableTypes() []client.Object
	ProcessExposition(ctx context.Context, appState types.ApplicationState, exposition types.Exposition) error
}

type Ingress struct {
	ingressClass string
	client       client.Client
	dogu         doguAdapter
	maintenance  maintenanceAdapter
	controller   ingressController
}

func (i Ingress) ProcessExposition(ctx context.Context, exposition types.Exposition) error {
	return nil
}

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
