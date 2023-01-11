package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	networking "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ingressClassCreationEventReason = "IngressClassCreation"
)

// ingressClassCreator is responsible to create a cluster wide ingress class in the cluster.
type ingressClassCreator struct {
	client        client.Client
	className     string
	namespace     string
	eventRecorder record.EventRecorder
}

// NewIngressClassCreator creates a new ingress class creator.
func NewIngressClassCreator(client client.Client, className string, namespace string, recorder record.EventRecorder) *ingressClassCreator {
	return &ingressClassCreator{
		client:        client,
		className:     className,
		namespace:     namespace,
		eventRecorder: recorder,
	}
}

// CreateIngressClass check whether the ingress class for the generator exists. If not it will be created.
func (icc ingressClassCreator) CreateIngressClass(ctx context.Context) error {
	log.FromContext(ctx).Info(fmt.Sprintf("checking for existing ingress class [%s]", icc.className))
	ok, err := icc.isIngressClassAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if ingress class [%s] exists: %w", icc.className, err)
	}

	deployment := &appsv1.Deployment{}
	err = icc.client.Get(ctx, types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: icc.namespace}, deployment)
	if err != nil {
		return fmt.Errorf("create ingress class: failed to get deployment [k8s-service-discovery-controller-manager]: %w", err)
	}

	if ok {
		icc.eventRecorder.Eventf(deployment, corev1.EventTypeNormal, ingressClassCreationEventReason, "Ingress class [%s] already exists.", icc.className)
		log.FromContext(ctx).Info(fmt.Sprintf("ingress class [%s] already exists -> skip creation", icc.className))
		return nil
	}

	ingressClassResource := &networking.IngressClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   icc.className,
			Labels: map[string]string{"app": "ces", "app.kubernetes.io/name": "k8s-service-discovery"},
		},
		Spec: networking.IngressClassSpec{
			Controller: "k8s.io/nginx-ingress",
		},
	}

	err = icc.client.Create(context.Background(), ingressClassResource)
	if err != nil {
		return fmt.Errorf("cannot create ingress class [%s] with clientset: %w", icc.className, err)
	}
	icc.eventRecorder.Eventf(deployment, corev1.EventTypeNormal, ingressClassCreationEventReason, "Ingress class [%s] created.", icc.className)

	return nil
}

// isIngressClassAvailable check whether an ingress class with the given name exists in the current namespace.
func (icc ingressClassCreator) isIngressClassAvailable() (bool, error) {
	ingressClassKey := types.NamespacedName{
		Namespace: "",
		Name:      icc.className,
	}
	ingressClass := &networking.IngressClass{}
	err := icc.client.Get(context.Background(), ingressClassKey, ingressClass)
	if err != nil && apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to get ingress class [%s] with clientset: %w", icc.className, err)
	}

	return true, nil
}

func (icc ingressClassCreator) Start(ctx context.Context) error {
	return icc.CreateIngressClass(ctx)
}
