package services

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/internal"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Processor interface {
	internal.ExpositionProcessor
}

// ExpositionService acts as a composite processor that orchestrates
// a collection of specialized adapters that process Exposition.
type ExpositionService struct {
	// processors contains all adapters that need to be executed for an exposition.
	processors []Processor
}

// NewExpositionService creates a new composite domain service.
func NewExpositionService(processors ...Processor) *ExpositionService {
	return &ExpositionService{
		processors: processors,
	}
}

// GetOwnableTypes aggregates all ownable Kubernetes types from all registered adapters.
// This allows the controller to dynamically learn about every resource type that
// might be generated, without knowing the specific adapters.
func (s *ExpositionService) GetOwnableTypes() []client.Object {
	var allTypes []client.Object
	for _, p := range s.processors {
		allTypes = append(allTypes, p.GetOwnableTypes()...)
	}
	return allTypes
}

// ProcessExposition forwards the exposition to all registered adapters
// sequentially. If any adapter fails, the processing chain is interrupted.
func (s *ExpositionService) ProcessExposition(ctx context.Context, exposition types.Exposition) error {
	for _, p := range s.processors {
		if err := p.ProcessExposition(ctx, exposition); err != nil {
			return fmt.Errorf("failed to process exposition with %T: %w", p, err)
		}
	}

	return nil
}
