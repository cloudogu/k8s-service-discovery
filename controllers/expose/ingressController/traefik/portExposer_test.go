package traefik

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testNamespace = "ecosystem"
)

func TestIngressNginxTcpUpdExposer_ExposePorts(t *testing.T) {
	tests := []struct {
		name           string
		inExposedPorts types.ExposedPorts
		inOwner        *metav1.OwnerReference
		setupMocks     func(m *mockConfigMapInterface, expTCPData, expUDPData map[string]string, expOwner bool)
		expTCPMapData  map[string]string
		expUDPMapData  map[string]string
		expErr         bool
		expErrStr      string
	}{
		{
			name: "set tcp and udp ports in both config maps",
			inExposedPorts: types.ExposedPorts{
				{Name: "a-50", ServiceName: "a", Protocol: corev1.ProtocolTCP, Port: 50, TargetPort: 60},
				{Name: "b-60", ServiceName: "b", Protocol: corev1.ProtocolUDP, Port: 60, TargetPort: 60},
				{Name: "c-70", ServiceName: "c", Protocol: corev1.ProtocolTCP, Port: 70, TargetPort: 60},
				{Name: "d-80", ServiceName: "d", Protocol: corev1.ProtocolUDP, Port: 80, TargetPort: 60},
			},
			inOwner:    &metav1.OwnerReference{},
			setupMocks: expectCreateCMWithAssertion(t),
			expTCPMapData: map[string]string{
				"50": createMapEntryValue("a", "60"),
				"70": createMapEntryValue("c", "60"),
			},
			expUDPMapData: map[string]string{
				"60": createMapEntryValue("b", "60"),
				"80": createMapEntryValue("d", "60"),
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "create tcp config map with empty values",
			inExposedPorts: types.ExposedPorts{
				{Name: "b-60", ServiceName: "b", Protocol: corev1.ProtocolUDP, Port: 60, TargetPort: 60},
				{Name: "d-80", ServiceName: "d", Protocol: corev1.ProtocolUDP, Port: 80, TargetPort: 60},
			},
			inOwner:       &metav1.OwnerReference{},
			setupMocks:    expectCreateCMWithAssertion(t),
			expTCPMapData: map[string]string{},
			expUDPMapData: map[string]string{
				"60": createMapEntryValue("b", "60"),
				"80": createMapEntryValue("d", "60"),
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "create udp config map with empty values",
			inExposedPorts: types.ExposedPorts{
				{Name: "a-50", ServiceName: "a", Protocol: corev1.ProtocolTCP, Port: 50, TargetPort: 60},
				{Name: "c-70", ServiceName: "c", Protocol: corev1.ProtocolTCP, Port: 70, TargetPort: 60},
			},
			inOwner:    &metav1.OwnerReference{},
			setupMocks: expectCreateCMWithAssertion(t),
			expTCPMapData: map[string]string{
				"50": createMapEntryValue("a", "60"),
				"70": createMapEntryValue("c", "60"),
			},
			expUDPMapData: map[string]string{},
			expErr:        false,
			expErrStr:     "",
		},
		{
			name: "ignore ports with unknown ports",
			inExposedPorts: types.ExposedPorts{
				{Name: "a-50", ServiceName: "a", Protocol: corev1.ProtocolTCP, Port: 50, TargetPort: 60},
				{Name: "b-60", ServiceName: "b", Protocol: corev1.ProtocolUDP, Port: 60, TargetPort: 60},
				{Name: "c-70", ServiceName: "c", Protocol: "invalid", Port: 70, TargetPort: 60},
			},
			inOwner:    &metav1.OwnerReference{},
			setupMocks: expectCreateCMWithAssertion(t),
			expTCPMapData: map[string]string{
				"50": createMapEntryValue("a", "60"),
			},
			expUDPMapData: map[string]string{
				"60": createMapEntryValue("b", "60"),
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "only set owner when defined",
			inExposedPorts: types.ExposedPorts{
				{Name: "a-50", ServiceName: "a", Protocol: corev1.ProtocolTCP, Port: 50, TargetPort: 60},
				{Name: "b-60", ServiceName: "b", Protocol: corev1.ProtocolUDP, Port: 60, TargetPort: 60},
			},
			inOwner:    nil,
			setupMocks: expectCreateCMWithAssertion(t),
			expTCPMapData: map[string]string{
				"50": createMapEntryValue("a", "60"),
			},
			expUDPMapData: map[string]string{
				"60": createMapEntryValue("b", "60"),
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "update tcp and udp config map when they already exist",
			inExposedPorts: types.ExposedPorts{
				{Name: "a-50", ServiceName: "a", Protocol: corev1.ProtocolTCP, Port: 50, TargetPort: 60},
				{Name: "d-80", ServiceName: "d", Protocol: corev1.ProtocolUDP, Port: 80, TargetPort: 60},
			},
			inOwner:    &metav1.OwnerReference{},
			setupMocks: expectUpdateCMWithAssertion(t),
			expTCPMapData: map[string]string{
				"50": createMapEntryValue("a", "60"),
			},
			expUDPMapData: map[string]string{
				"80": createMapEntryValue("d", "60"),
			},
			expErr:    false,
			expErrStr: "",
		},
		{
			name: "return error when cm cannot be created",
			inExposedPorts: types.ExposedPorts{
				{Name: "a-50", ServiceName: "a", Protocol: corev1.ProtocolTCP, Port: 50, TargetPort: 60},
				{Name: "b-60", ServiceName: "b", Protocol: corev1.ProtocolUDP, Port: 60, TargetPort: 60},
			},
			inOwner: &metav1.OwnerReference{},
			setupMocks: func(m *mockConfigMapInterface, expTCPData, expUDPData map[string]string, expOwner bool) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr:    true,
			expErrStr: "failed to create configMap",
		},
		{
			name: "return error when cm cannot be created",
			inExposedPorts: types.ExposedPorts{
				{Name: "a-50", ServiceName: "a", Protocol: corev1.ProtocolTCP, Port: 50, TargetPort: 60},
				{Name: "b-60", ServiceName: "b", Protocol: corev1.ProtocolUDP, Port: 60, TargetPort: 60},
			},
			inOwner: &metav1.OwnerReference{},
			setupMocks: func(m *mockConfigMapInterface, expTCPData, expUDPData map[string]string, expOwner bool) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(corev1.Resource("configmap"), "configmap"))
				m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr:    true,
			expErrStr: "failed to update configMap",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmClientMock := newMockConfigMapInterface(t)
			tt.setupMocks(cmClientMock, tt.expTCPMapData, tt.expUDPMapData, tt.inOwner != nil)

			exposer := PortExposer{}

			err := exposer.ExposePorts(context.TODO(), testNamespace, tt.inExposedPorts, tt.inOwner)

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func createMapEntryValue(svcName, targetPort string) string {
	return fmt.Sprintf("%s/%s:%s", testNamespace, svcName, targetPort)
}

func asserCM(t *testing.T, cm *corev1.ConfigMap, expData map[string]string, expOwner bool) {
	require.NotNil(t, cm)

	require.Equal(t, testNamespace, cm.Namespace)
	require.Equal(t, util.K8sCesServiceDiscoveryLabels, cm.Labels)
	require.Equal(t, expOwner, len(cm.OwnerReferences) > 0)

	require.True(t, maps.Equal(expData, cm.Data))
}

func expectCreateCMWithAssertion(t *testing.T) func(m *mockConfigMapInterface, expTCPData, expUDPData map[string]string, expOwner bool) {
	return func(m *mockConfigMapInterface, expTCPData, expUDPData map[string]string, expOwner bool) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, cm *corev1.ConfigMap, opts metav1.CreateOptions) {
				if cm.Name == "tcp-services" {
					asserCM(t, cm, expTCPData, expOwner)
					return
				}

				if cm.Name == "udp-services" {
					asserCM(t, cm, expUDPData, expOwner)
					return
				}

				assert.Fail(t, "unexpected config map name", "name", cm.Name)
			}).
			Return(&corev1.ConfigMap{}, nil)
	}
}

func expectUpdateCMWithAssertion(t *testing.T) func(m *mockConfigMapInterface, expTCPData, expUDPData map[string]string, expOwner bool) {
	return func(m *mockConfigMapInterface, expTCPData, expUDPData map[string]string, expOwner bool) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(corev1.Resource("configmap"), "configmap"))
		m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, cm *corev1.ConfigMap, opts metav1.UpdateOptions) {
				if cm.Name == "tcp-services" {
					asserCM(t, cm, expTCPData, expOwner)
					return
				}

				if cm.Name == "udp-services" {
					asserCM(t, cm, expUDPData, expOwner)
					return
				}

				assert.Fail(t, "unexpected config map name", "name", cm.Name)
			}).
			Return(&corev1.ConfigMap{}, nil)
	}
}
