package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestParseLoadbalancerConfig(t *testing.T) {
	tests := []struct {
		name      string
		in        *corev1.ConfigMap
		expConfig LoadbalancerConfig
		expErr    bool
		expErrStr string
	}{
		{
			name: "Parse valid config",
			in: &corev1.ConfigMap{Data: map[string]string{
				"config.yaml": `
annotations:
  a: test
  b: cloudogu
internalTrafficPolicy: Cluster
externalTrafficPolicy: Local
`,
			}},
			expConfig: LoadbalancerConfig{
				Annotations: map[string]string{
					"a": "test",
					"b": "cloudogu",
				},
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyCluster,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "Parse empty annotations",
			in: &corev1.ConfigMap{Data: map[string]string{
				"config.yaml": `
annotations:
internalTrafficPolicy: Cluster
externalTrafficPolicy: Local
`,
			}},
			expConfig: LoadbalancerConfig{
				Annotations:           nil,
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyCluster,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "Parse annotations not set",
			in: &corev1.ConfigMap{Data: map[string]string{
				"config.yaml": `
internalTrafficPolicy: Cluster
externalTrafficPolicy: Local
`,
			}},
			expConfig: LoadbalancerConfig{
				Annotations:           nil,
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyCluster,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "Use default values when config.yaml is empty",
			in: &corev1.ConfigMap{Data: map[string]string{
				"config.yaml": ``,
			}},
			expConfig: LoadbalancerConfig{
				Annotations:           nil,
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyCluster,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "return error on wrong internalTrafficPolicy type",
			in: &corev1.ConfigMap{Data: map[string]string{
				"config.yaml": `
internalTrafficPolicy: invalid
externalTrafficPolicy: Local
`,
			}},
			expErr:    true,
			expErrStr: "internalTrafficPolicy has invalid type",
		},
		{
			name: "return error on wrong externalTrafficPolicy type",
			in: &corev1.ConfigMap{Data: map[string]string{
				"config.yaml": `
internalTrafficPolicy: Local
externalTrafficPolicy: invalid
`,
			}},
			expErr:    true,
			expErrStr: "externalTrafficPolicy has invalid type",
		},
		{
			name: "return error on wrong externalTrafficPolicy type",
			in: &corev1.ConfigMap{Data: map[string]string{
				"config.yaml": `invalid`,
			}},
			expErr:    true,
			expErrStr: "failed to unmarshal loadbalancer from config map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb, err := ParseLoadbalancerConfig(tt.in)

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.Equal(t, tt.expConfig, lb)
		})
	}
}
