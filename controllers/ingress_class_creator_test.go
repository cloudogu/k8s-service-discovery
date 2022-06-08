package controllers_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/cloudogu/k8s-service-discovery/controllers"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/stretchr/testify/require"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func getScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	return scheme
}

func TestNewIngressClassCreator(t *testing.T) {
	clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
	creator := controllers.NewIngressClassCreator(clientMock, "my-ingress-class")

	require.NotNil(t, creator)
}

func TestIngressClassCreator_CreateIngressClass(t *testing.T) {
	t.Run("ingress class does not exists and is begin created", func(t *testing.T) {
		t.Parallel()

		// given
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator := controllers.NewIngressClassCreator(clientMock, "my-ingress-class")

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
		creator := controllers.NewIngressClassCreator(clientMock, "my-ingress-class")

		// when
		err := creator.CreateIngressClass(ctrl.Log.WithName("test"))

		// then
		require.NoError(t, err)
	})
}
