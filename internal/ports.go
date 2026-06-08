package internal

import (
	"context"

	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ExpositionProcessor interface {
	GetOwnableTypes() []client.Object
	ProcessExposition(ctx context.Context, exposition types.Exposition) error
}
