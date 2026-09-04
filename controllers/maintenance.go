package controllers

import (
	"context"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	maintenanceConfigMapName = repository.MaintenanceConfigMapName
)

// maintenanceConfigMapPredicate fires on every write to the maintenance ConfigMap.
// Combining the name match with ResourceVersionChanged keeps events for other ConfigMaps and informer resyncs out of the workqueue.
func maintenanceConfigMapPredicate() predicate.Predicate {
	return predicate.And(
		predicate.ResourceVersionChangedPredicate{},
		predicate.NewPredicateFuncs(func(obj client.Object) bool {
			return obj.GetName() == maintenanceConfigMapName
		}),
	)
}

// mapRequestsFromMaintenanceConfigMap builds the EnqueueRequestsFromMapFunc
// used by every controller that needs to re-reconcile its dogu-scoped
// resources when the maintenance ConfigMap changes.
//
// The caller supplies one factory that returns a fresh, empty list of the
// resource type to enqueue (e.g. &corev1.ServiceList{}). The helper lists
// objects in the ConfigMap's namespace filtered by the dogu label and
// returns one reconcile.Request per listed item, keyed by the item's own
// namespace and name. (Both Services and Exposition CRs use their
// metadata.name as the dogu name by convention.)
//
// The ConfigMap-name check lives in maintenanceConfigMapPredicate, so this
// function assumes any incoming object is the maintenance ConfigMap and
// does not re-check.
func mapRequestsFromMaintenanceConfigMap(
	cli client.Client,
	newList func() client.ObjectList,
	logName string,
) handler.MapFunc {
	// labels.NewRequirement with selection.Exists and nil values cannot fail
	// (no value validation runs), so the helper builds the selector once and
	// captures it in the returned closure.
	doguLabelReq, _ := labels.NewRequirement(doguv2.DoguLabelName, selection.Exists, nil)
	doguLabelSelector := labels.NewSelector().Add(*doguLabelReq)

	return func(ctx context.Context, object client.Object) []reconcile.Request {
		logger := log.FromContext(ctx).WithName(logName)

		list := newList()
		if err := cli.List(ctx, list, &client.ListOptions{
			Namespace:     object.GetNamespace(),
			LabelSelector: doguLabelSelector,
		}); err != nil {
			logger.Error(err, "failed to list dogu-scoped objects")
			return nil
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			logger.Error(err, "failed to extract list items")
			return nil
		}

		requests := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			mo, ok := item.(metav1.Object)
			if !ok {
				continue
			}
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{
				Namespace: mo.GetNamespace(),
				Name:      mo.GetName(),
			}})
		}
		return requests
	}
}
