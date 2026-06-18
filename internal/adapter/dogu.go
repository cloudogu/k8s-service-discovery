package adapter

import (
	"context"
	"fmt"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Dogu reads dogu CRs from the cluster and translates their status
// into the domain ApplicationState consumed by the Ingress processor.
type Dogu struct {
	Client client.Client
}

// GetStatus fetches the dogu CR identified by (namespace, doguName)
// and returns its derived ApplicationState. A Stopped dogu reports
// ApplicationStopped regardless of its healthy condition; otherwise
// the dogu reports ApplicationRunning when ConditionHealthy is true,
// and ApplicationIsStarting in every other case (condition false,
// unknown, or missing).
func (d Dogu) GetStatus(ctx context.Context, namespace string, doguName string) (types.ApplicationState, error) {
	dogu := &doguv2.Dogu{}
	doguKey := apitypes.NamespacedName{
		Namespace: namespace,
		Name:      doguName,
	}

	if lErr := d.Client.Get(ctx, doguKey, dogu); lErr != nil {
		return 0, fmt.Errorf("failed to get dogu: %w", lErr)
	}

	if dogu.Status.Stopped {
		return types.ApplicationStopped, nil
	}

	if !apimeta.IsStatusConditionTrue(dogu.Status.Conditions, doguv2.ConditionHealthy) {
		return types.ApplicationIsStarting, nil
	}

	return types.ApplicationRunning, nil
}
