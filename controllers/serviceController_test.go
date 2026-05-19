package controllers

import (
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/stretchr/testify/require"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func getScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := new(runtime.NewSchemeBuilder(
		corev1.AddToScheme,
		appsv1.AddToScheme,
		networkingv1.AddToScheme,
		traefikapi.AddToScheme,
		expositionv1.AddToScheme,
	)).AddToScheme(scheme)
	require.NoError(t, err)
	return scheme
}
