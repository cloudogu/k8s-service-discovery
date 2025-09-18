package types

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestCreateDefaultPorts(t *testing.T) {
	ports := CreateDefaultPorts()

	expHttpPort := ExposedPort{
		Name:       "http",
		Protocol:   corev1.ProtocolTCP,
		Port:       httpPort,
		TargetPort: httpPort,
	}

	expHttpsPort := ExposedPort{
		Name:       "https",
		Protocol:   corev1.ProtocolTCP,
		Port:       httpsPort,
		TargetPort: httpsPort,
	}

	assert.True(t, slices.Contains(ports, expHttpPort))
	assert.True(t, slices.Contains(ports, expHttpsPort))
}

func TestExposedPort_ToServicePort(t *testing.T) {
	exposedPort := ExposedPort{
		Name:       "test",
		Protocol:   corev1.ProtocolUDP,
		Port:       12345,
		TargetPort: 67890,
		nodePort:   400,
	}

	expServicePort := corev1.ServicePort{
		Name:       "test",
		Protocol:   corev1.ProtocolUDP,
		Port:       12345,
		TargetPort: intstr.FromInt32(67890),
		NodePort:   400,
	}

	assert.Equal(t, expServicePort, exposedPort.ToServicePort())
}

func TestExposedPorts_SortByName(t *testing.T) {
	exposedPorts := ExposedPorts{
		{"http", "", corev1.ProtocolTCP, 80, 80, 1234},
		{"https", "", corev1.ProtocolTCP, 443, 443, 342342450},
		{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
		{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
	}

	expectedOrder := ExposedPorts{
		{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
		{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
		{"http", "", corev1.ProtocolTCP, 80, 80, 1234},
		{"https", "", corev1.ProtocolTCP, 443, 443, 342342450},
	}

	assert.False(t, slices.Equal(expectedOrder, exposedPorts))

	exposedPorts.SortByName()
	assert.True(t, slices.Equal(expectedOrder, exposedPorts))
}

func TestExposedPorts_Equals(t *testing.T) {
	tests := []struct {
		name     string
		exPorts1 ExposedPorts
		exPorts2 ExposedPorts
		expEqual bool
	}{
		{
			name:     "be true when both are empty",
			exPorts1: ExposedPorts{},
			exPorts2: ExposedPorts{},
			expEqual: true,
		},
		{
			name: "be true when values are same and in order",
			exPorts1: ExposedPorts{
				{"http", "", corev1.ProtocolTCP, 80, 80, 1234},
				{"https", "", corev1.ProtocolTCP, 443, 443, 342342450},
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			exPorts2: ExposedPorts{
				{"http", "", corev1.ProtocolTCP, 80, 80, 1234},
				{"https", "", corev1.ProtocolTCP, 443, 443, 342342450},
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			expEqual: true,
		},
		{
			name: "be true when values are same but in different order",
			exPorts1: ExposedPorts{
				{"https", "", corev1.ProtocolTCP, 443, 443, 342342450},
				{"http", "", corev1.ProtocolTCP, 80, 80, 1234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
			},
			exPorts2: ExposedPorts{
				{"http", "", corev1.ProtocolTCP, 80, 80, 1234},
				{"https", "", corev1.ProtocolTCP, 443, 443, 342342450},
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			expEqual: true,
		},
		{
			name: "be false when name differs",
			exPorts1: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			exPorts2: ExposedPorts{
				{"DIFFER", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			expEqual: false,
		},
		{
			name: "be false when protocol differs",
			exPorts1: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			exPorts2: ExposedPorts{
				{"alpha", "", corev1.ProtocolTCP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			expEqual: false,
		},
		{
			name: "be false when port differs",
			exPorts1: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			exPorts2: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 0, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			expEqual: false,
		},
		{
			name: "be false when target port differs",
			exPorts1: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			exPorts2: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 0, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			expEqual: false,
		},
		{
			name: "be false when node port differs",
			exPorts1: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			exPorts2: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 0},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			expEqual: false,
		},
		{
			name: "be false number of elements differs",
			exPorts1: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
				{"beta", "", corev1.ProtocolSCTP, 392, 391, 121232},
			},
			exPorts2: ExposedPorts{
				{"alpha", "", corev1.ProtocolUDP, 391, 391, 12123234},
			},
			expEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expEqual, tt.exPorts1.Equals(tt.exPorts2))
		})
	}
}

func TestExposedPorts_ToServicePorts(t *testing.T) {
	tests := []struct {
		name string
		in   ExposedPorts
		exp  []corev1.ServicePort
	}{
		{
			name: "map ExposedPorts to ServicePorts",
			in: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 3},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
			exp: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 1, intstr.FromInt32(2), 3},
				{"b", corev1.ProtocolUDP, nil, 5, intstr.FromInt32(6), 7},
			},
		},
		{
			name: "should return ServicePorts sorted by name",
			in: ExposedPorts{
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
				{"a", "", corev1.ProtocolTCP, 1, 2, 3},
			},
			exp: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 1, intstr.FromInt32(2), 3},
				{"b", corev1.ProtocolUDP, nil, 5, intstr.FromInt32(6), 7},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.exp, tt.in.ToServicePorts())
		})
	}
}

func TestExposedPorts_SetNodePorts(t *testing.T) {
	tests := []struct {
		name           string
		inExposedPorts ExposedPorts
		inServicePorts []corev1.ServicePort
		exp            ExposedPorts
	}{
		{
			name: "set node port from service ports when name, protocol, port and target port match",
			inExposedPorts: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 0},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
			inServicePorts: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 1, intstr.FromInt32(2), 99},
				{"b", corev1.ProtocolUDP, nil, 5, intstr.FromInt32(6), 666},
			},
			exp: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 99},
				{"b", "", corev1.ProtocolUDP, 5, 6, 666},
			},
		},
		{
			name: "keep node port when service ports are different",
			inExposedPorts: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 0},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
			inServicePorts: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 2, intstr.FromInt32(2), 99},
				{"b", corev1.ProtocolUDP, nil, 6, intstr.FromInt32(6), 666},
			},
			exp: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 0},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
		},
		{
			name: "keep node port of b when service port for b does not exist",
			inExposedPorts: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 0},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
			inServicePorts: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 1, intstr.FromInt32(2), 99},
			},
			exp: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 99},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
		},
		{
			name: "keep node port when service ports are empty",
			inExposedPorts: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 0},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
			inServicePorts: []corev1.ServicePort{},
			exp: ExposedPorts{
				{"a", "", corev1.ProtocolTCP, 1, 2, 0},
				{"b", "", corev1.ProtocolUDP, 5, 6, 7},
			},
		},
		{
			name:           "empty ExposedPorts",
			inExposedPorts: ExposedPorts{},
			inServicePorts: []corev1.ServicePort{
				{"a", corev1.ProtocolTCP, nil, 2, intstr.FromInt32(2), 99},
				{"b", corev1.ProtocolUDP, nil, 6, intstr.FromInt32(6), 666},
			},
			exp: ExposedPorts{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.inExposedPorts.SetNodePorts(tt.inServicePorts)
			assert.Equal(t, tt.exp, tt.inExposedPorts)
		})
	}
}

func TestExposedPort_PortString(t *testing.T) {
	exPort := ExposedPort{Port: 50000}
	require.Equal(t, "50000", exPort.PortString())
}
