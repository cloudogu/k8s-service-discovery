package nginx

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNginxController(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)

		// when
		sut := NewNginxController(configMapInterfaceMock)

		// then
		require.NotNil(t, sut.ingressNginxTcpUpdExposer)
		assert.Equal(t, configMapInterfaceMock, sut.ingressNginxTcpUpdExposer.configMapInterface)
	})
}

func Test_controller_GetName(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		sut := &IngressController{}

		// when
		name := sut.GetName()

		// then
		require.Equal(t, "nginx-ingress", name)
	})
}

func Test_controller_GetControllerSpec(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		sut := &IngressController{}

		// when
		spec := sut.GetControllerSpec()

		// then
		require.Equal(t, "k8s.io/nginx-ingress", spec)
	})
}

func Test_controller_GetRewriteAnnotationKey(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		sut := &IngressController{}

		// when
		key := sut.GetRewriteAnnotationKey()

		// then
		require.Equal(t, "nginx.ingress.kubernetes.io/rewrite-target", key)
	})
}

func Test_controller_GetUseRegexKey(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		sut := &IngressController{}

		// when
		key := sut.GetUseRegexKey()

		// then
		require.Equal(t, "nginx.ingress.kubernetes.io/use-regex", key)
	})
}
