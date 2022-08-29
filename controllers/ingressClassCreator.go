package controllers

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"

	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
	networking "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ingressClassCreator is responsible to create a cluster wide ingress class in the cluster.
type ingressClassCreator struct {
	client    client.Client
	className string
}

// NewIngressClassCreator creates a new ingress class creator.
func NewIngressClassCreator(client client.Client, className string) *ingressClassCreator {
	return &ingressClassCreator{
		client:    client,
		className: className,
	}
}

// CreateIngressClass check whether the ingress class for the generator exists. If not it will be created.
func (icc ingressClassCreator) CreateIngressClass(logger logr.Logger) error {
	logger.Info(fmt.Sprintf("checking for existing ingress class [%s]", icc.className))
	ok, err := icc.isIngressClassAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if ingress class [%s] exists: %w", icc.className, err)
	}
	if ok {
		logger.Info(fmt.Sprintf("ingress class [%s] already exists -> skip creation", icc.className))
		return nil
	}

	ingressClassResource := &networking.IngressClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: icc.className,
		},
		Spec: networking.IngressClassSpec{
			Controller: "k8s.io/nginx-ingress",
		},
	}

	err = icc.client.Create(context.Background(), ingressClassResource)
	if err != nil {
		return fmt.Errorf("cannot create ingress class [%s] with clientset: %w", icc.className, err)
	}

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

func (icc ingressClassCreator) Start(context.Context) error {
	return icc.CreateIngressClass(ctrl.Log.WithName("ingress-class-creator"))
}
