package types

import (
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExposition_HasExposedPorts(t *testing.T) {
	tests := []struct {
		name string
		in   Exposition
		exp  bool
	}{
		{
			name: "return true when TCP ports are present",
			in: Exposition{
				Spec: expositionv1.ExpositionSpec{
					TCP: []expositionv1.TCPEntry{
						{Name: "ssh", Service: "my-service", Port: 2222},
					},
				},
			},
			exp: true,
		},
		{
			name: "return true when UDP ports are present",
			in: Exposition{
				Spec: expositionv1.ExpositionSpec{
					UDP: []expositionv1.UDPEntry{
						{Name: "dns", Service: "my-service", Port: 53},
					},
				},
			},
			exp: true,
		},
		{
			name: "return true when both TCP and UDP ports are present",
			in: Exposition{
				Spec: expositionv1.ExpositionSpec{
					TCP: []expositionv1.TCPEntry{
						{Name: "ssh", Service: "my-service", Port: 2222},
					},
					UDP: []expositionv1.UDPEntry{
						{Name: "dns", Service: "my-service", Port: 53},
					},
				},
			},
			exp: true,
		},
		{
			name: "return false when no ports are defined",
			in:   Exposition{},
			exp:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.exp, tt.in.HasExposedPorts())
		})
	}
}

func TestExposition_GetExposedPorts(t *testing.T) {
	tcpReqPort := int32(32222)
	udpReqPort := int32(15353)

	tests := []struct {
		name string
		in   Exposition
		exp  ExposedPorts
	}{
		{
			name: "return empty when no ports are defined",
			in:   Exposition{},
			exp:  ExposedPorts{},
		},
		{
			name: "return single TCP port without RequestedExternalPort",
			in: Exposition{
				ObjectMeta: metav1.ObjectMeta{Name: "my-exposition"},
				Spec: expositionv1.ExpositionSpec{
					TCP: []expositionv1.TCPEntry{
						{Name: "ssh", Service: "my-service", Port: 2222},
					},
				},
			},
			exp: ExposedPorts{
				{Name: "my-exposition-ssh", ServiceName: "my-service", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
			},
		},
		{
			name: "return single TCP port with RequestedExternalPort",
			in: Exposition{
				ObjectMeta: metav1.ObjectMeta{Name: "my-exposition"},
				Spec: expositionv1.ExpositionSpec{
					TCP: []expositionv1.TCPEntry{
						{Name: "ssh", Service: "my-service", Port: 2222, RequestedExternalPort: &tcpReqPort},
					},
				},
			},
			exp: ExposedPorts{
				{Name: "my-exposition-ssh", ServiceName: "my-service", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 32222},
			},
		},
		{
			name: "return single UDP port without RequestedExternalPort",
			in: Exposition{
				ObjectMeta: metav1.ObjectMeta{Name: "my-exposition"},
				Spec: expositionv1.ExpositionSpec{
					UDP: []expositionv1.UDPEntry{
						{Name: "dns", Service: "my-service", Port: 53},
					},
				},
			},
			exp: ExposedPorts{
				{Name: "my-exposition-dns", ServiceName: "my-service", Protocol: corev1.ProtocolUDP, Port: 53, TargetPort: 53},
			},
		},
		{
			name: "return single UDP port with RequestedExternalPort",
			in: Exposition{
				ObjectMeta: metav1.ObjectMeta{Name: "my-exposition"},
				Spec: expositionv1.ExpositionSpec{
					UDP: []expositionv1.UDPEntry{
						{Name: "dns", Service: "my-service", Port: 53, RequestedExternalPort: &udpReqPort},
					},
				},
			},
			exp: ExposedPorts{
				{Name: "my-exposition-dns", ServiceName: "my-service", Protocol: corev1.ProtocolUDP, Port: 53, TargetPort: 15353},
			},
		},
		{
			name: "return mixed TCP and UDP ports sorted by name",
			in: Exposition{
				ObjectMeta: metav1.ObjectMeta{Name: "my-exposition"},
				Spec: expositionv1.ExpositionSpec{
					TCP: []expositionv1.TCPEntry{
						{Name: "ssh", Service: "svc-a", Port: 2222},
						{Name: "ldap", Service: "svc-b", Port: 389},
					},
					UDP: []expositionv1.UDPEntry{
						{Name: "dns", Service: "svc-c", Port: 53},
						{Name: "ntp", Service: "svc-d", Port: 123},
					},
				},
			},
			exp: ExposedPorts{
				{Name: "my-exposition-dns", ServiceName: "svc-c", Protocol: corev1.ProtocolUDP, Port: 53, TargetPort: 53},
				{Name: "my-exposition-ldap", ServiceName: "svc-b", Protocol: corev1.ProtocolTCP, Port: 389, TargetPort: 389},
				{Name: "my-exposition-ntp", ServiceName: "svc-d", Protocol: corev1.ProtocolUDP, Port: 123, TargetPort: 123},
				{Name: "my-exposition-ssh", ServiceName: "svc-a", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ports := tt.in.GetExposedPorts()
			require.Equal(t, tt.exp, ports)
		})
	}
}
