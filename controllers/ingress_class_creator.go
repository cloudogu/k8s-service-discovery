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

type IngressClassCreator struct {
	Client    client.Client `json:"client"`
	ClassName string        `json:"class_name"`
}

// NewIngressClassCreator creates a new ingress class creator.
func NewIngressClassCreator(client client.Client, className string) (IngressClassCreator, error) {
	return IngressClassCreator{
		Client:    client,
		ClassName: className,
	}, nil
}

// CreateIngressClass check whether the ingress class for the generator exists. If not it will be created.
func (icc IngressClassCreator) CreateIngressClass(logger logr.Logger) error {
	logger.Info(fmt.Sprintf("checking for existing ingress class [%s]", icc.ClassName))
	ok, err := icc.isIngressClassAvailable()
	if err != nil {
		return fmt.Errorf("failed to check if ingress class [%s] exists: %w", icc.ClassName, err)
	}
	if ok {
		logger.Info(fmt.Sprintf("ingress class [%s] already exists -> skip creation", icc.ClassName))
		return nil
	}

	ingressClassResource := &networking.IngressClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name: icc.ClassName,
		},
		Spec: networking.IngressClassSpec{
			Controller: "k8s.io/ingress-nginx",
		},
	}

	err = icc.Client.Create(context.Background(), ingressClassResource)
	if err != nil {
		return fmt.Errorf("cannot create ingress class [%s] with clientset: %w", icc.ClassName, err)
	}

	return nil
}

// isIngressClassAvailable check whether an ingress class with the given name exists in the current namespace.
func (icc IngressClassCreator) isIngressClassAvailable() (bool, error) {
	ingressClassKey := types.NamespacedName{
		Namespace: "",
		Name:      icc.ClassName,
	}
	ingressClass := &networking.IngressClass{}
	err := icc.Client.Get(context.Background(), ingressClassKey, ingressClass)
	if err != nil && apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to get ingress class [%s] with clientset: %w", icc.ClassName, err)
	}

	return true, nil
}

func (icc IngressClassCreator) Start(context.Context) error {
	return icc.CreateIngressClass(ctrl.Log.WithName("ingress-class-creator"))
}
