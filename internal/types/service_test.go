package types

import (
	"testing"

	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestParseService(t *testing.T) {
	tests := []struct {
		name string
		in   metav1.Object
		exp  bool
	}{
		{
			name: "parse k8s service to service",
			in: &corev1.Service{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testNamespace",
					Labels: map[string]string{
						k8sv2.DoguLabelName: "testDogu",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
				Status: corev1.ServiceStatus{},
			},
			exp: true,
		},
		{
			name: "return false when object has wrong type",
			in: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testNamespace",
					Labels: map[string]string{
						k8sv2.DoguLabelName: "testDogu",
					},
				},
			},
			exp: false,
		},
		{
			name: "return false when labels are empty",
			in: &corev1.Service{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testNamespace",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
				Status: corev1.ServiceStatus{},
			},
			exp: false,
		},
		{
			name: "return false when dogu.name label is missing",
			in: &corev1.Service{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testNamespace",
					Labels: map[string]string{
						"invalid": "testDogu",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
				Status: corev1.ServiceStatus{},
			},
			exp: false,
		},
		{
			name: "return false when service has other type than ClusterIP",
			in: &corev1.Service{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testNamespace",
					Labels: map[string]string{
						k8sv2.DoguLabelName: "testDogu",
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
				Status: corev1.ServiceStatus{},
			},
			exp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doguService, ok := ParseService(tt.in)

			assert.Equal(t, tt.exp, ok)
			if tt.exp {
				assert.Equal(t, tt.in, (*corev1.Service)(&doguService))
			}
		})
	}

	t.Run("ensure annotations are set", func(t *testing.T) {
		in := &corev1.Service{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "testNamespace",
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
			Status: corev1.ServiceStatus{},
		}

		doguService, ok := ParseService(in)
		require.True(t, ok)
		require.NotNil(t, doguService)
		require.NotNil(t, doguService.Annotations)
	})
}

func TestService_HasExposedPorts(t *testing.T) {
	tests := []struct {
		name string
		in   Service
		exp  bool
	}{
		{
			name: "return true when ces-exposed-ports annotation is set",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
					},
				},
			},
			exp: true,
		},
		{
			name: "return true when ces-exposed-ports are empty",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[]`,
					},
				},
			},
			exp: true,
		},
		{
			name: "return true when ces-exposed-ports are invalid",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `invalid`,
					},
				},
			},
			exp: true,
		},
		{
			name: "return false when ces-exposed-ports is missing",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"otherKey": `otherValue`,
					},
				},
			},
			exp: false,
		},
		{
			name: "return false when annotations are empty",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			exp: false,
		},
		{
			name: "return false when annotations are nil",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			exp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.exp, tt.in.HasExposedPorts())
		})
	}
}

func TestService_GetExposedPorts(t *testing.T) {
	tests := []struct {
		name      string
		in        Service
		exp       ExposedPorts
		expErr    bool
		expErrStr string
	}{
		{
			name: "return single exposed port",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
					},
				},
			},
			exp: ExposedPorts{
				{"test-50000", "test", corev1.ProtocolTCP, 50000, 50000, 0},
			},
		},
		{
			name: "support SCTP protocol",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"sctp","port":50000,"targetPort":50000}]`,
					},
				},
			},
			exp: ExposedPorts{
				{"test-50000", "test", corev1.ProtocolSCTP, 50000, 50000, 0},
			},
		},
		{
			name: "return multiple exposed ports",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000},{"protocol":"UDP","port":1,"targetPort":1}]`,
					},
				},
			},
			exp: ExposedPorts{
				{"test-1", "test", corev1.ProtocolUDP, 1, 1, 0},
				{"test-50000", "test", corev1.ProtocolTCP, 50000, 50000, 0},
			},
		},
		{
			name: "return empty exposed ports when ces-exposed-ports annotation is empty",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[]`,
					},
				},
			},
			exp: ExposedPorts{},
		},
		{
			name: "return empty exposed ports when ces-exposed-ports annotation is not set",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			exp: ExposedPorts{},
		},
		{
			name: "ignore name set in annotation",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"Name": "PORT", "protocol":"tcp","port":50000,"targetPort":50000}]`,
					},
				},
			},
			exp: ExposedPorts{
				{"test-50000", "test", corev1.ProtocolTCP, 50000, 50000, 0},
			},
		},
		{
			name: "ignore nodePort set in annotation",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000, "nodePort": 5}]`,
					},
				},
			},
			exp: ExposedPorts{
				{"test-50000", "test", corev1.ProtocolTCP, 50000, 50000, 0},
			},
		},
		{
			name: "return error when json is invalid",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `invalid`,
					},
				},
			},
			expErr:    true,
			expErrStr: "failed to unmarshal exposed ports",
		},
		{
			name: "return error when port is out of range",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"tcp","port":500000000000000000,"targetPort":50000}]`,
					},
				},
			},
			expErr:    true,
			expErrStr: "port is invalid",
		},
		{
			name: "return error when port is negative",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"tcp","port":-1,"targetPort":50000}]`,
					},
				},
			},
			expErr:    true,
			expErrStr: "number is negative",
		},
		{
			name: "return error when target port is out of range",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":500000000000000000}]`,
					},
				},
			},
			expErr:    true,
			expErrStr: "targetPort is invalid",
		},
		{
			name: "return error when unknown protocol is used",
			in: Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `[{"protocol":"invalid","port":50000,"targetPort":50000}]`,
					},
				},
			},
			expErr:    true,
			expErrStr: "unsupported protocol for exposed port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports, err := tt.in.GetExposedPorts()

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.Equal(t, tt.exp, ports)
		})
	}
}
