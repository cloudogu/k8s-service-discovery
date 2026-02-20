package traefik

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTraefikController(t *testing.T) {
	ingressMock := newMockIngressInterface(t)

	tests := []struct {
		name              string
		inControllerType  string
		expControllerType controllerType
	}{
		{
			name:              "default",
			inControllerType:  "",
			expControllerType: gateway,
		},
		{
			name:              "k8s-ces-gateway component",
			inControllerType:  GatewayControllerName,
			expControllerType: gateway,
		},
		{
			name:              "nginx-ingress dogu",
			inControllerType:  IngressControllerName,
			expControllerType: ingress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := NewTraefikController(IngressControllerDependencies{
				IngressInterface: ingressMock,
				IngressClassName: "test",
				ControllerType:   tt.inControllerType,
			})

			require.NotNil(t, ctrl)
			require.NotNil(t, ctrl.ingressInterface)
			require.Equal(t, "test", ctrl.ingressClassName)
			require.Equal(t, tt.expControllerType, ctrl.controllerType)
		})
	}
}

func Test_controller_GetName(t *testing.T) {
	tests := []struct {
		name             string
		inControllerType controllerType
		expName          string
	}{
		{
			name:             "default",
			inControllerType: 0,
			expName:          GatewayControllerName,
		},
		{
			name:             "k8s-ces-gateway component",
			inControllerType: gateway,
			expName:          GatewayControllerName,
		},
		{
			name:             "nginx-ingress dogu",
			inControllerType: ingress,
			expName:          IngressControllerName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := &IngressController{controllerType: tt.inControllerType}
			assert.Equal(t, tt.expName, ctrl.GetName())
		})
	}
}

func Test_controller_GetRewriteAnnotationKey(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		sut := &IngressController{}

		// when
		key := sut.GetRewriteAnnotationKey()

		// then
		require.Equal(t, "traefik.ingress.kubernetes.io/router.middlewares", key)
	})
}

func TestIngressController_GetSelector(t *testing.T) {
	tests := []struct {
		name             string
		inControllerType controllerType
		expSelector      map[string]string
	}{
		{
			name:             "default",
			inControllerType: 0,
			expSelector:      selectorMap[gateway],
		},
		{
			name:             "k8s-ces-gateway component",
			inControllerType: gateway,
			expSelector:      selectorMap[gateway],
		},
		{
			name:             "nginx-ingress dogu",
			inControllerType: ingress,
			expSelector:      selectorMap[ingress],
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := &IngressController{controllerType: tt.inControllerType}
			assert.Equal(t, tt.expSelector, ctrl.GetSelector())
		})
	}
}

func Test_controllerType_String(t *testing.T) {
	tests := []struct {
		name             string
		inControllerType controllerType
		expName          string
	}{
		{
			name:             "default",
			inControllerType: 0,
			expName:          GatewayControllerName,
		},
		{
			name:             "k8s-ces-gateway component",
			inControllerType: gateway,
			expName:          GatewayControllerName,
		},
		{
			name:             "nginx-ingress dogu",
			inControllerType: ingress,
			expName:          IngressControllerName,
		},
		{
			name:             "invalid",
			inControllerType: 100,
			expName:          "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := &IngressController{controllerType: tt.inControllerType}
			assert.Equal(t, tt.expName, ctrl.String())
		})
	}
}
