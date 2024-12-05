package expose

import (
	"context"
	"fmt"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	networking "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ingressClassCreationEventReason = "IngressClassCreation"
)

// ingressClassCreator is responsible to create a cluster wide ingress class in the cluster.
type ingressClassCreator struct {
	className             string
	namespace             string
	eventRecorder         eventRecorder
	ingressController     ingressController
	ingressClassInterface ingressClassInterface
	deploymentInterface   deploymentInterface
}

// NewIngressClassCreator creates a new ingress class creator.
func NewIngressClassCreator(clientset clientSetInterface, className string, namespace string, recorder eventRecorder, controller ingressController) *ingressClassCreator {
	icInterface := clientset.NetworkingV1().IngressClasses()
	deployInterface := clientset.AppsV1().Deployments(namespace)

	return &ingressClassCreator{
		className:             className,
		namespace:             namespace,
		eventRecorder:         recorder,
		ingressController:     controller,
		ingressClassInterface: icInterface,
		deploymentInterface:   deployInterface,
	}
}

// CreateIngressClass check whether the ingress class for the generator exists. If not it will be created.
func (icc ingressClassCreator) CreateIngressClass(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("checking for existing ingress class [%s]", icc.className))
	ok, err := icc.isIngressClassAvailable(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if ingress class [%s] exists: %w", icc.className, err)
	}

	deployment, err := icc.deploymentInterface.Get(ctx, "k8s-service-discovery-controller-manager", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("create ingress class: failed to get deployment [k8s-service-discovery-controller-manager]: %w", err)
	}

	if ok {
		icc.eventRecorder.Eventf(deployment, corev1.EventTypeNormal, ingressClassCreationEventReason, "Ingress class [%s] already exists.", icc.className)
		ctrl.LoggerFrom(ctx).Info(fmt.Sprintf("ingress class [%s] already exists -> skip creation", icc.className))
		return nil
	}

	ingressClassResource := &networking.IngressClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   icc.className,
			Labels: util.K8sCesServiceDiscoveryLabels,
		},
		Spec: networking.IngressClassSpec{
			Controller: icc.ingressController.GetControllerSpec(),
		},
	}

	_, err = icc.ingressClassInterface.Create(ctx, ingressClassResource, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("cannot create ingress class [%s] with clientset: %w", icc.className, err)
	}
	icc.eventRecorder.Eventf(deployment, corev1.EventTypeNormal, ingressClassCreationEventReason, "Ingress class [%s] created.", icc.className)

	return nil
}

// isIngressClassAvailable check whether an ingress class with the given name exists in the current namespace.
func (icc ingressClassCreator) isIngressClassAvailable(ctx context.Context) (bool, error) {
	_, err := icc.ingressClassInterface.Get(ctx, icc.className, metav1.GetOptions{})
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
