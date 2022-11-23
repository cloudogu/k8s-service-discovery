package controllers

import (
	"context"
	"github.com/cloudogu/k8s-service-discovery/controllers/mocks"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	"testing"

	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNewIngressClassCreator(t *testing.T) {
	namespace := "test"
	clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
	creator := NewIngressClassCreator(clientMock, "my-ingress-class", namespace, mocks.NewEventRecorder(t))

	require.NotNil(t, creator)
}

func TestIngressClassCreator_CreateIngressClass(t *testing.T) {
	namespace := "test"
	t.Run("failed to get deployment", func(t *testing.T) {
		t.Parallel()

		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects().Build()
		creator := NewIngressClassCreator(clientMock, "my-ingress-class", namespace, nil)

		// when
		err := creator.CreateIngressClass(context.Background())

		// then
		require.Error(t, err)
		require.ErrorContains(t, err, "create ingress class: failed to get deployment [k8s-service-discovery]")
	})

	t.Run("ingress class does not exists and is begin created", func(t *testing.T) {
		t.Parallel()

		// given
		deployment := &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		recorderMock := mocks.NewEventRecorder(t)
		recorderMock.On("Eventf", mock.IsType(deployment), "Normal", "Creation", "Ingress class [%s] created.", "my-ingress-class")
		creator := NewIngressClassCreator(clientMock, "my-ingress-class", namespace, recorderMock)

		// when
		err := creator.CreateIngressClass(context.Background())

		// then
		require.NoError(t, err)
	})

	t.Run("ingress class does already exist and is not begin created", func(t *testing.T) {
		t.Parallel()

		// given
		ingressClass := &networking.IngressClass{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-ingress-class",
				Namespace: "",
			},
			Spec: networking.IngressClassSpec{
				Controller: "k8s.io/nginx-ingress",
			},
		}
		deployment := &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(ingressClass, deployment).Build()
		recorderMock := mocks.NewEventRecorder(t)
		recorderMock.On("Eventf", mock.IsType(deployment), "Warning", "ErrCreation", "Ingress class [%s] already exists.", "my-ingress-class")
		creator := NewIngressClassCreator(clientMock, "my-ingress-class", namespace, recorderMock)

		// when
		err := creator.CreateIngressClass(context.Background())

		// then
		require.NoError(t, err)
	})
}
