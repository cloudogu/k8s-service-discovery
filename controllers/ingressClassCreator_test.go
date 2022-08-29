package controllers

import (
	"testing"

	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/stretchr/testify/require"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewIngressClassCreator(t *testing.T) {
	clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
	creator := NewIngressClassCreator(clientMock, "my-ingress-class")

	require.NotNil(t, creator)
}

func TestIngressClassCreator_CreateIngressClass(t *testing.T) {
	t.Run("ingress class does not exists and is begin created", func(t *testing.T) {
		t.Parallel()

		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator := NewIngressClassCreator(clientMock, "my-ingress-class")

		// when
		err := creator.CreateIngressClass(ctrl.Log.WithName("test"))

		// then
		require.NoError(t, err)
	})

	t.Run("ingress class does already exist and is not begin created", func(t *testing.T) {
		t.Parallel()

		// given
		ingressClass := &networking.IngressClass{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name: "myIngressClass",
			},
			Spec: networking.IngressClassSpec{
				Controller: "k8s.io/nginx-ingress",
			},
		}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(ingressClass).Build()
		creator := NewIngressClassCreator(clientMock, "my-ingress-class")

		// when
		err := creator.CreateIngressClass(ctrl.Log.WithName("test"))

		// then
		require.NoError(t, err)
	})
}
