package ingressController

import (
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/ingressController/traefik"
	"github.com/stretchr/testify/require"
)

func TestParseIngressController(t *testing.T) {
	t.Run("should create traefik controller", func(t *testing.T) {
		// when
		controller := ParseIngressController(Dependencies{Controller: "traefik-ingress"})

		// then
		require.IsType(t, &traefik.IngressController{}, controller)
	})

	t.Run("should create traefik controller as default", func(t *testing.T) {
		// when
		controller := ParseIngressController(Dependencies{Controller: "does not exists in switch case"})

		// then
		require.IsType(t, &traefik.IngressController{}, controller)
	})
}
