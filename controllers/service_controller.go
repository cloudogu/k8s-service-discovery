/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

type CesService struct {
	Name     string `json:"name"`
	Port     int    `json:"port"`
	Location string `json:"location"`
	Pass     string `json:"pass"`
}

//+kubebuilder:rbac:groups=cloudogu.com,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cloudogu.com,resources=services/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cloudogu.com,resources=services/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Service object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *ServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	service := &corev1.Service{}
	err := r.Get(ctx, req.NamespacedName, service)
	if err != nil {
		logger.Info(fmt.Sprintf("failed to get service: %s", err))
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	logger.Info("found", "service", service)

	if len(service.Spec.Ports) == 0 {
		logger.Info("found no ports for", "service", service.Name)
		return ctrl.Result{}, nil
	}

	cesServicesAnnotation, ok := service.Annotations["service-discovery.cloudogu.com/ces-services"]
	if !ok {
		logger.Info("found no services annotation for", "service", service.Name)
		return ctrl.Result{}, nil
	}

	var cesServices []CesService
	err = json.Unmarshal([]byte(cesServicesAnnotation), &cesServices)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to unmarshal ces services: %w", err)
	}

	for _, cesService := range cesServices {
		ingressClassName := "nginx-ecosystem"
		pathType := networking.PathTypePrefix
		ingress := &networking.Ingress{
			ObjectMeta: v1.ObjectMeta{
				Name:      cesService.Name,
				Namespace: service.Namespace,
				Labels:    nil,
			},
		}

		result, err := ctrl.CreateOrUpdate(ctx, r.Client, ingress, func() error {
			ingress.Spec = networking.IngressSpec{
				IngressClassName: &ingressClassName,
				Rules: []networking.IngressRule{{
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{
							Paths: []networking.HTTPIngressPath{{Path: cesService.Location,
								PathType: &pathType,
								Backend: networking.IngressBackend{
									Service: &networking.IngressServiceBackend{
										Name: service.GetName(),
										Port: networking.ServiceBackendPort{
											Number: int32(cesService.Port),
										},
									}}}}}}}}}
			ingress.ObjectMeta.Annotations = map[string]string{"nginx.ingress.kubernetes.io/rewrite-target": cesService.Pass}
			err = ctrl.SetControllerReference(service, ingress, r.Scheme)
			if err != nil {
				return fmt.Errorf("failed to set controller reference for ingress: %w", err)
			}
			return nil
		})
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create ingress object: %w", err)
		}
		logger.Info("created or updated ingress object", "result", result)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		// Uncomment the following line adding a pointer to an instance of the controlled resource as an argument
		For(&corev1.Service{}).
		Complete(r)
}
