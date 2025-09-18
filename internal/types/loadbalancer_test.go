package types

import (
	"slices"
	"strings"
	"testing"

	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

var (
	defaultExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
	defaultInternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyCluster)
)

func createManagedKeyCfg(s ...string) string {
	return strings.Join(s, configManagedAnnotationKeySeparator)
}

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

func TestParseLoadBalancer(t *testing.T) {
	tests := []struct {
		name string
		in   metav1.Object
		exp  bool
	}{
		{
			name: "parse k8s service to loadbalancer",
			in: &corev1.Service{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      LoadbalancerName,
					Namespace: "testNamespace",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeLoadBalancer,
				},
				Status: corev1.ServiceStatus{},
			},
			exp: true,
		},
		{
			name: "return false when object has wrong name",
			in: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "otherLoadBalancer",
					Namespace: "testNamespace",
				},
			},
			exp: false,
		},
		{
			name: "return false when object has wrong type",
			in: &corev1.ConfigMap{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      LoadbalancerName,
					Namespace: "testNamespace",
				},
			},
			exp: false,
		},
		{
			name: "return false when service has other type than LoadBalancer",
			in: &corev1.Service{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      LoadbalancerName,
					Namespace: "testNamespace",
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				},
				Status: corev1.ServiceStatus{},
			},
			exp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb, ok := ParseLoadBalancer(tt.in)

			assert.Equal(t, tt.exp, ok)
			if tt.exp {
				assert.Equal(t, tt.in, (*corev1.Service)(&lb))
			}
		})
	}

	t.Run("ensure annotations are set", func(t *testing.T) {
		in := &corev1.Service{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      LoadbalancerName,
				Namespace: "testNamespace",
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeLoadBalancer,
			},
			Status: corev1.ServiceStatus{},
		}

		lb, ok := ParseLoadBalancer(in)
		require.True(t, ok)
		require.NotNil(t, lb)
		require.NotNil(t, lb.Annotations)
	})
}

func TestCreateLoadBalancer(t *testing.T) {
	//given
	ePorts := ExposedPorts{
		{
			Name:        "a-80",
			ServiceName: "a",
			Protocol:    "TCP",
			Port:        80,
			TargetPort:  80,
		},
		{
			Name:        "a-443",
			ServiceName: "a",
			Protocol:    "TCP",
			Port:        443,
			TargetPort:  443,
		},
	}

	lbConfig := LoadbalancerConfig{
		Annotations: map[string]string{
			"test":  "loadBalancer",
			"bTest": "secondAnnotation",
		},
		InternalTrafficPolicy: "Cluster",
		ExternalTrafficPolicy: "Cluster",
	}

	selector := map[string]string{
		"k8s.cloudogu.com": "testLoadbalancer",
	}

	lb := CreateLoadBalancer("testNamespace", lbConfig, ePorts, selector)

	// assert general config
	assert.Equal(t, LoadbalancerName, lb.Name)
	assert.Equal(t, corev1.ServiceTypeLoadBalancer, lb.Spec.Type)
	assert.Equal(t, "testNamespace", lb.Namespace)
	assert.Equal(t, util.GetAppLabel(), lb.Labels)
	assert.Equal(t, []corev1.IPFamily{corev1.IPv4Protocol}, lb.Spec.IPFamilies)
	assert.Equal(t, ptr.To(corev1.IPFamilyPolicySingleStack), lb.Spec.IPFamilyPolicy)

	// assert spec config
	assert.Equal(t, corev1.ServiceExternalTrafficPolicyCluster, lb.Spec.ExternalTrafficPolicy)
	assert.Equal(t, ptr.To(corev1.ServiceInternalTrafficPolicyCluster), lb.Spec.InternalTrafficPolicy)

	// asser annotations
	require.NotNil(t, lb.Annotations)
	assert.Len(t, lb.GetAnnotations(), 3)

	lbAnnotations, ok := lb.GetAnnotations()[configManagedAnnotationKey]
	assert.True(t, ok)

	for k, v := range lbConfig.Annotations {
		slices.Contains(strings.Split(lbAnnotations, configManagedAnnotationKeySeparator), k)
		value, mOk := lb.Annotations[k]
		assert.True(t, mOk)
		assert.Equal(t, v, value)
	}

	// assert Ports
	assert.Len(t, lb.Spec.Ports, len(ePorts))
	for _, p := range ePorts {
		slices.Contains(lb.Spec.Ports, p.ToServicePort())
	}
}

func TestLoadBalancer_ToK8sService(t *testing.T) {
	t.Run("return k8s service when defined", func(t *testing.T) {
		lbService := &corev1.Service{
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

		lb := LoadBalancer(*lbService)
		assert.Equal(t, lbService, lb.ToK8sService())
	})

	t.Run("return nil when loadbalancer is not defined", func(t *testing.T) {
		var lb *LoadBalancer
		assert.Nil(t, lb.ToK8sService())
	})
}

func TestLoadBalancer_ApplyConfig(t *testing.T) {
	tests := []struct {
		name   string
		lb     LoadBalancer
		newCfg LoadbalancerConfig
		expLb  LoadBalancer
	}{
		{
			name: "Override existing config",
			lb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
			newCfg: LoadbalancerConfig{
				Annotations: map[string]string{
					"keyA": "ValueC",
					"keyB": "ValueD",
				},
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyLocal,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
			},
			expLb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueC",
						"keyB":                     "ValueD",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
				},
			},
		},
		{
			name: "Keep annotations keys that are not managed by config",
			lb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						"cloudKey":                 "CloudValue",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
			newCfg: LoadbalancerConfig{
				Annotations: map[string]string{
					"keyA": "ValueC",
					"keyB": "ValueD",
				},
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyLocal,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
			},
			expLb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueC",
						"keyB":                     "ValueD",
						"cloudKey":                 "CloudValue",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
				},
			},
		},
		{
			name: "delete managed values that aren't set in new config",
			lb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
			newCfg: LoadbalancerConfig{
				Annotations: map[string]string{
					"keyB": "ValueD",
				},
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyLocal,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
			},
			expLb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyB":                     "ValueD",
						configManagedAnnotationKey: createManagedKeyCfg("keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
				},
			},
		},
		{
			name: "delete managed keys when empty",
			lb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
			newCfg: LoadbalancerConfig{
				Annotations:           nil,
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyLocal,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
			},
			expLb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						configManagedAnnotationKey: "",
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
				},
			},
		},
		{
			name: "only delete managed keys when empty",
			lb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						"CloudKey":                 "CloudValue",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
			newCfg: LoadbalancerConfig{
				Annotations:           nil,
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyLocal,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
			},
			expLb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						configManagedAnnotationKey: "",
						"CloudKey":                 "CloudValue",
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
				},
			},
		},
		{
			name: "should add configManagedAnnotationKey when empty",
			lb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"CloudKey": "CloudValue",
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyCluster),
				},
			},
			newCfg: LoadbalancerConfig{
				Annotations:           nil,
				InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyLocal,
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
			},
			expLb: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						configManagedAnnotationKey: "",
						"CloudKey":                 "CloudValue",
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.lb.ApplyConfig(tt.newCfg)
			assert.Equal(t, tt.expLb, tt.lb)
		})
	}
}

func TestLoadBalancer_GetOwnerReference(t *testing.T) {
	t.Run("get valid owner reference", func(t *testing.T) {
		lb := LoadBalancer{
			ObjectMeta: metav1.ObjectMeta{
				Name: LoadbalancerName,
				UID:  "testUUID",
			},
		}

		s := runtime.NewScheme()
		err := corev1.AddToScheme(s)
		require.NoError(t, err)

		oR, err := lb.GetOwnerReference(s)
		require.NoError(t, err)
		require.NotNil(t, oR)

		assert.Equal(t, oR.Name, LoadbalancerName)
		assert.Equal(t, oR.Kind, "Service")
		assert.Equal(t, oR.UID, types.UID("testUUID"))
	})

	t.Run("wrap error when GVKForObject fails", func(t *testing.T) {
		lb := LoadBalancer{
			ObjectMeta: metav1.ObjectMeta{
				Name: LoadbalancerName,
				UID:  "testUUID",
			},
		}

		oR, err := lb.GetOwnerReference(runtime.NewScheme())
		assert.Error(t, err)
		assert.ErrorContains(t, err, "failed to get GroupVersionKind for loadbalancer")
		require.Nil(t, oR)
	})
}

func TestLoadBalancer_UpdateExposedPorts(t *testing.T) {
	tests := []struct {
		name           string
		inExposedPorts ExposedPorts
		lbPorts        []corev1.ServicePort
		exp            []corev1.ServicePort
	}{
		{
			name: "Update nodeports on new incoming ports",
			inExposedPorts: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 0},
				{"b", "", corev1.ProtocolUDP, 5, 6, 0},
				{"c", "", corev1.ProtocolUDP, 10, 7, 0},
			},
			lbPorts: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 1, intstr.FromInt32(2), 99},
				{"b", corev1.ProtocolUDP, nil, 5, intstr.FromInt32(6), 666},
			},
			exp: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 1, intstr.FromInt32(2), 99},
				{"b", corev1.ProtocolUDP, nil, 5, intstr.FromInt32(6), 666},
				{"c", corev1.ProtocolUDP, nil, 10, intstr.FromInt32(7), 0}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lb := LoadBalancer{
				Spec: corev1.ServiceSpec{
					Ports: tt.lbPorts,
				},
			}

			lb.UpdateExposedPorts(tt.inExposedPorts)

			assert.Equal(t, tt.exp, lb.Spec.Ports)
		})
	}
}

func TestLoadBalancer_Equals(t *testing.T) {
	tests := []struct {
		name  string
		in    LoadBalancer
		other LoadBalancer
		exp   bool
	}{
		{
			name: "equal when all managed fields match",
			in: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
						{
							Name:       "B",
							Protocol:   corev1.ProtocolUDP,
							Port:       63,
							TargetPort: intstr.FromInt32(64),
							NodePort:   5678,
						},
					},
				},
			},
			other: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
						{
							Name:       "B",
							Protocol:   corev1.ProtocolUDP,
							Port:       63,
							TargetPort: intstr.FromInt32(64),
							NodePort:   5678,
						},
					},
				},
			},
			exp: true,
		},
		{
			name: "equal when unmanaged annotations differ",
			in: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						"CloudKey":                 "CloudValue",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
			},
			other: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						"PremiseKey":               "PremiseValue",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
			},
			exp: true,
		},
		{
			name: "equal when managed annotations are empty",
			in: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":     "ValueA",
						"keyB":     "ValueB",
						"CloudKey": "CloudValue",
					},
				},
			},
			other: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"A":          "A",
						"B":          "B",
						"PremiseKey": "PremiseValue",
					},
				},
			},
			exp: true,
		},
		{
			name: "equal when status differs",
			in: LoadBalancer{
				Status: corev1.ServiceStatus{
					Conditions: []metav1.Condition{
						{
							Status: "Installed",
						},
					},
				},
			},
			other: LoadBalancer{
				Status: corev1.ServiceStatus{
					Conditions: []metav1.Condition{
						{
							Status: "ERROR",
						},
					},
				},
			},
			exp: true,
		},
		{
			name: "equal when node ports differ",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
						{
							Name:       "B",
							Protocol:   corev1.ProtocolUDP,
							Port:       63,
							TargetPort: intstr.FromInt32(64),
							NodePort:   5678,
						},
					},
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   5678,
						},
						{
							Name:       "B",
							Protocol:   corev1.ProtocolUDP,
							Port:       63,
							TargetPort: intstr.FromInt32(64),
							NodePort:   1234,
						},
					},
				},
			},
			exp: true,
		},
		{
			name: "not equal when number of ports are different",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "B",
							Protocol:   corev1.ProtocolUDP,
							Port:       63,
							TargetPort: intstr.FromInt32(64),
							NodePort:   5678,
						},
					},
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
						{
							Name:       "B",
							Protocol:   corev1.ProtocolUDP,
							Port:       63,
							TargetPort: intstr.FromInt32(64),
							NodePort:   5678,
						},
					},
				},
			},
			exp: false,
		},
		{
			name: "not equal when name differs",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
					},
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "B",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
					},
				},
			},
			exp: false,
		},
		{
			name: "not equal when protocol differs",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
					},
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolUDP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
					},
				},
			},
			exp: false,
		},
		{
			name: "not equal when port differs",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
					},
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       443,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
					},
				},
			},
			exp: false,
		},
		{
			name: "not equal when target port differs",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(81),
							NodePort:   1234,
						},
					},
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
					Ports: []corev1.ServicePort{
						{
							Name:       "A",
							Protocol:   corev1.ProtocolTCP,
							Port:       80,
							TargetPort: intstr.FromInt32(443),
							NodePort:   1234,
						},
					},
				},
			},
			exp: false,
		},
		{
			name: "not equal when object name differs",
			in: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{Name: "A"},
			},
			other: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{Name: "B"},
			},
			exp: false,
		},
		{
			name: "not equal when managed ExternalTrafficPolicy spec differs",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyCluster,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
				},
			},
			exp: false,
		},
		{
			name: "not equal when managed InternalTrafficPolicy spec differs",
			in: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: defaultInternalTrafficPolicy,
				},
			},
			other: LoadBalancer{
				Spec: corev1.ServiceSpec{
					ExternalTrafficPolicy: defaultExternalTrafficPolicy,
					InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
				},
			},
			exp: false,
		},
		{
			name: "not equal when managed annotations differ in terms of key value pairs",
			in: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyB":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyB"),
					},
				},
			},
			other: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						"keyC":                     "ValueC",
						configManagedAnnotationKey: createManagedKeyCfg("keyA", "keyC"),
					},
				},
			},
			exp: false,
		},
		{
			name: "not equal when managed annotations differ in terms values",
			in: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						configManagedAnnotationKey: createManagedKeyCfg("keyA"),
					},
				},
			},
			other: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA"),
					},
				},
			},
			exp: false,
		},
		{
			name: "not equal when managed annotations differ in terms of invalid managed keys",
			in: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyA":                     "ValueA",
						configManagedAnnotationKey: createManagedKeyCfg("keyA"),
					},
				},
			},
			other: LoadBalancer{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keyB":                     "ValueB",
						configManagedAnnotationKey: createManagedKeyCfg("keyA"),
					},
				},
			},
			exp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.exp, tt.in.Equals(tt.other))
		})
	}
}
