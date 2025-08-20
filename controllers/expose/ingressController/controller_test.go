package ingressController

import (
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/ingressController/nginx"
	"github.com/stretchr/testify/require"
)

func TestParseIngressController(t *testing.T) {
	t.Run("should create nginx controller", func(t *testing.T) {
		// when
		controller := ParseIngressController(Dependencies{Controller: "nginx-ingress"})

		// then
		require.IsType(t, &nginx.IngressController{}, controller)
	})

	t.Run("should create nginx controller as default", func(t *testing.T) {
		// when
		controller := ParseIngressController(Dependencies{Controller: "does not exists in switch case"})

		// then
		require.IsType(t, &nginx.IngressController{}, controller)
	})
}
