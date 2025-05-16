package ingressController

import (
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/ingressController/nginx"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseIngressController(t *testing.T) {
	t.Run("should create nginx controller", func(t *testing.T) {
		// when
		controller := ParseIngressController("nginx-ingress", nil)

		// then
		require.IsType(t, &nginx.IngressController{}, controller)
	})

	t.Run("should create nginx controller as default", func(t *testing.T) {
		// when
		controller := ParseIngressController("does not exists in switch case", nil)

		// then
		require.IsType(t, &nginx.IngressController{}, controller)
	})
}
