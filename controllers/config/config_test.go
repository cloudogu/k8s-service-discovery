package config

import (
	"os"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tUnsetenv unsets key for the duration of the test and restores the original value afterwards.
func tUnsetenv(t *testing.T, key string) {
	t.Helper()
	prev, ok := os.LookupEnv(key)
	t.Cleanup(func() {
		if ok {
			os.Setenv(key, prev)
		} else {
			os.Unsetenv(key)
		}
	})
	os.Unsetenv(key)
}

func TestReadIngressController(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T)
		exp   string
	}{
		{
			name: "return ingress controller name",
			setup: func(t *testing.T) {
				t.Setenv(ingressControllerEnvVar, "traefik")
			},
			exp: "traefik",
		},
		{
			name: "return empty string when env var is not set",
			setup: func(t *testing.T) {
				tUnsetenv(t, ingressControllerEnvVar)
			},
			exp: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			assert.Equal(t, tt.exp, ReadIngressController())
		})
	}
}

func TestReadWatchNamespace(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		exp       string
		expErr    bool
		expErrStr string
	}{
		{
			name: "return namespace when env var is set",
			setup: func(t *testing.T) {
				t.Setenv(namespaceEnvVar, "mynamespace")
			},
			exp: "mynamespace",
		},
		{
			name: "return error when env var is not set",
			setup: func(t *testing.T) {
				tUnsetenv(t, namespaceEnvVar)
			},
			expErr:    true,
			expErrStr: "WATCH_NAMESPACE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			result, err := ReadWatchNamespace()

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.exp, result)
		})
	}
}

func TestReadNetworkPolicyCIDR(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		exp       string
		expErr    bool
		expErrStr string
	}{
		{
			name: "return CIDR when env var is set",
			setup: func(t *testing.T) {
				t.Setenv(networkPolicyCIDREnvVar, "10.0.0.0/8")
			},
			exp: "10.0.0.0/8",
		},
		{
			name: "return error when env var is not set",
			setup: func(t *testing.T) {
				tUnsetenv(t, networkPolicyCIDREnvVar)
			},
			expErr:    true,
			expErrStr: "NETWORK_POLICIES_CIDR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			result, err := ReadNetworkPolicyCIDR()

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.exp, result)
		})
	}
}

func TestReadNetworkPolicyEnabled(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		exp       bool
		expErr    bool
		expErrStr string
	}{
		{
			name: "return true when enabled",
			setup: func(t *testing.T) {
				t.Setenv(networkPolicyEnabledEnvVar, "true")
			},
			exp: true,
		},
		{
			name: "return false when disabled",
			setup: func(t *testing.T) {
				t.Setenv(networkPolicyEnabledEnvVar, "false")
			},
			exp: false,
		},
		{
			name: "return true and error when env var is not set",
			setup: func(t *testing.T) {
				tUnsetenv(t, networkPolicyEnabledEnvVar)
			},
			exp:       true,
			expErr:    true,
			expErrStr: "NETWORK_POLICIES_ENABLED",
		},
		{
			name: "return true and error on invalid value",
			setup: func(t *testing.T) {
				t.Setenv(networkPolicyEnabledEnvVar, "invalid")
			},
			exp:       true,
			expErr:    true,
			expErrStr: "invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			result, err := ReadNetworkPolicyEnabled()

			assert.Equal(t, tt.exp, result)
			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestReadExpositionConfig(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T)
		exp       controllers.ExpositionConfig
		expErr    bool
		expErrStr string
	}{
		{
			name: "return disabled config when all is disabled",
			setup: func(t *testing.T) {
				t.Setenv(expositionExposePortsEnvVar, "false")
				t.Setenv(expositionDiscoverServicesEnvVar, "false")
				t.Setenv(expositionDiscoverExpositionCrEnvVar, "false")
			},
			exp: controllers.ExpositionConfig{
				ExposePorts:         false,
				DiscoverServices:    false,
				DiscoverExpositions: false,
			},
		},
		{
			name: "return full config when all env vars are set",
			setup: func(t *testing.T) {
				t.Setenv(expositionExposePortsEnvVar, "true")
				t.Setenv(expositionDiscoverServicesEnvVar, "true")
				t.Setenv(expositionDiscoverExpositionCrEnvVar, "false")
			},
			exp: controllers.ExpositionConfig{
				ExposePorts:         true,
				DiscoverServices:    true,
				DiscoverExpositions: false,
			},
		},
		{
			name: "return error when EXPOSITION_EXPOSE_PORTS is not set",
			setup: func(t *testing.T) {
				tUnsetenv(t, expositionExposePortsEnvVar)
			},
			expErr:    true,
			expErrStr: "EXPOSITION_EXPOSE_PORTS",
		},
		{
			name: "return error when EXPOSITION_EXPOSE_PORTS is invalid",
			setup: func(t *testing.T) {
				t.Setenv(expositionExposePortsEnvVar, "invalid")
			},
			expErr:    true,
			expErrStr: "EXPOSITION_EXPOSE_PORTS",
		},
		{
			name: "return error when EXPOSITION_DISCOVER_SERVICES is not set",
			setup: func(t *testing.T) {
				t.Setenv(expositionExposePortsEnvVar, "true")
				tUnsetenv(t, expositionDiscoverServicesEnvVar)
			},
			expErr:    true,
			expErrStr: "EXPOSITION_DISCOVER_SERVICES",
		},
		{
			name: "return error when EXPOSITION_DISCOVER_SERVICES is invalid",
			setup: func(t *testing.T) {
				t.Setenv(expositionExposePortsEnvVar, "true")
				t.Setenv(expositionDiscoverServicesEnvVar, "invalid")
			},
			expErr:    true,
			expErrStr: "EXPOSITION_DISCOVER_SERVICES",
		},
		{
			name: "return error when EXPOSITION_DISCOVER_EXPOSITION_CR is not set",
			setup: func(t *testing.T) {
				t.Setenv(expositionExposePortsEnvVar, "true")
				t.Setenv(expositionDiscoverServicesEnvVar, "true")
				tUnsetenv(t, expositionDiscoverExpositionCrEnvVar)
			},
			expErr:    true,
			expErrStr: "EXPOSITION_DISCOVER_EXPOSITION_CR",
		},
		{
			name: "return error when EXPOSITION_DISCOVER_EXPOSITION_CR is invalid",
			setup: func(t *testing.T) {
				t.Setenv(expositionExposePortsEnvVar, "true")
				t.Setenv(expositionDiscoverServicesEnvVar, "true")
				t.Setenv(expositionDiscoverExpositionCrEnvVar, "invalid")
			},
			expErr:    true,
			expErrStr: "EXPOSITION_DISCOVER_EXPOSITION_CR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)
			result, err := ReadExpositionConfig()

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.exp, result)
		})
	}
}
