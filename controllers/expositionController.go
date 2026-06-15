package controllers

import (
	"context"
	"errors"
	"fmt"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/adapter"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	validConditionType                = "Valid"
	initializingConditionReason       = "Initializing"
	mappingFailedConditionReason      = "MappingFailed"
	mappingSuccessfulConditionReason  = "MappingSuccessful"
	mappingSuccessfulConditionMessage = "The exposition has been successfully mapped to the domain."
)

// ExpositionReconciler watches Exposition CRs, maps each one to a domain
// Exposition and delegates processing to an ExpositionService. It also
// re-enqueues Expositions on Dogu health-condition changes and on
// maintenance ConfigMap updates.
type ExpositionReconciler struct {
	Client            client.Client
	ExpositionService ExpositionService
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For an Exposition CR it builds a domain Exposition and forwards it to the
// ExpositionService for processing.
func (r *ExpositionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := ctrl.LoggerFrom(ctx)

	expositionCR := &expositionv1.Exposition{}
	if err := r.Client.Get(ctx, req.NamespacedName, expositionCR); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get exposition: %w", err)
	}

	initializeErr := r.initializeUnknownConditions(ctx, expositionCR)
	if initializeErr != nil {
		initializeErr = fmt.Errorf("failed to initialize conditions with unknown: %w", initializeErr)
	}

	exposition, err := mapExpositionCRToExposition(expositionCR, r.Client)
	err = r.handleValidationErr(ctx, err, expositionCR)
	if err != nil {
		return ctrl.Result{}, err
	}

	if lErr := r.ExpositionService.ProcessExposition(ctx, exposition); lErr != nil {
		return ctrl.Result{}, fmt.Errorf("failed to process exposition from exposition CR: %w", lErr)
	}

	logger.Info("Successfully processed exposition from exposition CR.", "expositionCR", expositionCR.Name)

	return ctrl.Result{}, initializeErr
}

func (r *ExpositionReconciler) handleValidationErr(ctx context.Context, err error, expositionCR *expositionv1.Exposition) error {
	reason := mappingSuccessfulConditionReason
	message := mappingSuccessfulConditionMessage
	if err != nil {
		reason = mappingFailedConditionReason
		message = err.Error()
	}

	meta.SetStatusCondition(&expositionCR.Status.Conditions, metav1.Condition{
		Type:               validConditionType,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: expositionCR.Generation,
		Reason:             reason,
		Message:            message,
	})

	updateErr := r.Client.Status().Update(ctx, expositionCR)
	return errors.Join(err, updateErr)
}

func (r *ExpositionReconciler) initializeUnknownConditions(ctx context.Context, cr *expositionv1.Exposition) error {
	conditionTypes := []string{validConditionType, adapter.IngressesConditionType, adapter.NetworkPolicyConditionType,
		adapter.ConditionTypeIngressTCPRoutesCreated, adapter.ConditionTypeIngressUDPRoutesCreated}
	for _, conditionType := range conditionTypes {
		if meta.FindStatusCondition(cr.Status.Conditions, conditionType) == nil {
			meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
				Type:               conditionType,
				Reason:             initializingConditionReason,
				Status:             metav1.ConditionUnknown,
				ObservedGeneration: cr.Generation,
			})
		}
	}

	return r.Client.Status().Update(ctx, cr)
}

// SetupWithManager registers the reconciler with the Manager. It watches
// Exposition CRs (filtered to generation changes), Dogu CRs (filtered to
// healthy-condition changes), and the maintenance ConfigMap, plus any
// resource types reported by ExpositionService.GetOwnableTypes as Owns
// sources.
func (r *ExpositionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctrlBuilder := ctrl.NewControllerManagedBy(mgr).
		For(
			&expositionv1.Exposition{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&doguv2.Dogu{},
			handler.EnqueueRequestsFromMapFunc(mapRequestsFromDoguCR),
			builder.WithPredicates(doguHealthyConditionChangedPredicate()),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(mapRequestsFromMaintenanceConfigMap(
				r.Client,
				func() client.ObjectList { return &expositionv1.ExpositionList{} },
				"exposition-maintenance",
			)),
			builder.WithPredicates(maintenanceConfigMapPredicate()),
		)

	for _, res := range r.ExpositionService.GetOwnableTypes() {
		ctrlBuilder.Owns(res)
	}

	return ctrlBuilder.Complete(r)
}

// mapExpositionCRToExposition builds a domain Exposition from an Exposition
// CR: HTTP routes from spec.HTTP, TCP/UDP routes from spec.TCP and spec.UDP,
// and a SetOwner closure that wires the CR as the controller of every
// generated resource. The current implementation cannot fail; the error
// return is preserved for symmetry with mapServiceToExposition.
func mapExpositionCRToExposition(cr *expositionv1.Exposition, c client.Client) (types.Exposition, error) {
	return types.Exposition{
		Name:       cr.Name,
		Namespace:  cr.Namespace,
		HttpRoutes: mapExpositionCRToHttpRoutes(cr),
		TcpRoutes:  mapExpositionCRToTCPExposedPorts(cr),
		UdpRoutes:  mapExpositionCRToUDPExposedPorts(cr),
		SetOwner: func(targetObject client.Object) error {
			return ctrl.SetControllerReference(cr, targetObject, c.Scheme())
		},
		SetCondition: func(ctx context.Context, conditionType string, conditionStatus bool, reason string, msg string) error {
			meta.SetStatusCondition(&cr.Status.Conditions, metav1.Condition{
				Type:               conditionType,
				Status:             mapBoolToConditionStatus(conditionStatus),
				ObservedGeneration: cr.Generation,
				Reason:             reason,
				Message:            msg,
			})

			return c.Status().Update(ctx, cr)
		},
	}, nil
}

// mapExpositionCRToHttpRoutes translates spec.HTTP entries to domain
// HttpRoutes, preserving optional regex and strip-prefix rewrite rules.
// Returns a nil slice when spec.HTTP is empty (no allocation).
func mapExpositionCRToHttpRoutes(cr *expositionv1.Exposition) []types.HttpRoute {
	var httpRoutes []types.HttpRoute
	for _, httpEntry := range cr.Spec.HTTP {
		var rewrite *types.HttpRewrite
		if httpEntry.Rewrite != nil {
			var regex *types.RegexReplacement
			if httpEntry.Rewrite.Regex != nil {
				regex = &types.RegexReplacement{
					Pattern:     httpEntry.Rewrite.Regex.Pattern,
					Replacement: httpEntry.Rewrite.Regex.Replacement,
				}
			}

			rewrite = &types.HttpRewrite{
				StripPrefix: httpEntry.Rewrite.StripPrefix,
				Regex:       regex,
			}
		}

		httpRoutes = append(httpRoutes, types.HttpRoute{
			Name:    fmt.Sprintf("%s-%s-%d", cr.Name, httpEntry.Name, httpEntry.Port),
			Service: httpEntry.Service,
			Port:    httpEntry.Port,
			Path:    httpEntry.Path,
			Rewrite: rewrite,
		})
	}
	return httpRoutes
}

// mapExpositionCRToTCPExposedPorts translates spec.TCP entries to domain
// ExposedPorts (Protocol: TCP). RequestedExternalPort falls back to the
// entry's Port when nil — required while the Multi-CES port-allocator is not
// yet authoritative.
func mapExpositionCRToTCPExposedPorts(cr *expositionv1.Exposition) types.ExposedPorts {
	tcpRoutes := make(types.ExposedPorts, 0, len(cr.Spec.TCP))
	for _, tcpRoute := range cr.Spec.TCP {
		var reqExternalPortValue = tcpRoute.Port
		if tcpRoute.RequestedExternalPort != nil {
			reqExternalPortValue = *tcpRoute.RequestedExternalPort
		}

		tcpRoutes = append(tcpRoutes, types.ExposedPort{
			Name:                  fmt.Sprintf("%s-%s", cr.Name, tcpRoute.Name),
			ServiceName:           tcpRoute.Service,
			Protocol:              corev1.ProtocolTCP,
			ServicePort:           tcpRoute.Port,
			RequestedExternalPort: reqExternalPortValue,
		})
	}

	return tcpRoutes
}

// mapExpositionCRToUDPExposedPorts translates spec.UDP entries to domain
// ExposedPorts (Protocol: UDP). RequestedExternalPort falls back to the
// entry's Port when nil — required while the Multi-CES port-allocator is not
// yet authoritative.
func mapExpositionCRToUDPExposedPorts(cr *expositionv1.Exposition) types.ExposedPorts {
	udpRoutes := make(types.ExposedPorts, 0, len(cr.Spec.UDP))
	for _, udpRoute := range cr.Spec.UDP {
		var reqExternalPortValue = udpRoute.Port
		if udpRoute.RequestedExternalPort != nil {
			reqExternalPortValue = *udpRoute.RequestedExternalPort
		}

		udpRoutes = append(udpRoutes, types.ExposedPort{
			Name:                  fmt.Sprintf("%s-%s", cr.Name, udpRoute.Name),
			ServiceName:           udpRoute.Service,
			Protocol:              corev1.ProtocolUDP,
			ServicePort:           udpRoute.Port,
			RequestedExternalPort: reqExternalPortValue,
		})
	}

	return udpRoutes
}

func mapBoolToConditionStatus(boolStatus bool) metav1.ConditionStatus {
	if boolStatus {
		return metav1.ConditionTrue
	}

	return metav1.ConditionFalse
}
