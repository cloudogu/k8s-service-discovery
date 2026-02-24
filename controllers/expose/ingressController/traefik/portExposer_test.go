package traefik

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	testNamespace = "ecosystem"
)

func TestPortExposer_ExposePorts(t *testing.T) {
	tests := []struct {
		name           string
		inExposedPorts types.ExposedPorts
		setupMocks     func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface)
		expErr         bool
		expErrStr      string
	}{
		{
			name: "successfully create TCP and UDP IngressRoutes",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-2222", ServiceName: "svc", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
				{Name: "svc-5353", ServiceName: "svc", Protocol: corev1.ProtocolUDP, Port: 5353, TargetPort: 5353},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError).Times(2)

				tcpClientMock := newMockIngressrouteTcpInterface(t)
				tcpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Run(func(ctx context.Context, route *traefikv1alpha1.IngressRouteTCP, opts metav1.CreateOptions) {
						assertIngressRouteTCP(t, route, "svc", 2222, 2222)
					}).
					Return(&traefikv1alpha1.IngressRouteTCP{}, nil)
				traefikMock.EXPECT().IngressRouteTCPs(testNamespace).Return(tcpClientMock)

				udpClientMock := newMockIngressrouteUdpInterface(t)
				udpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Run(func(ctx context.Context, route *traefikv1alpha1.IngressRouteUDP, opts metav1.CreateOptions) {
						assertIngressRouteUDP(t, route, "svc", 5353, 5353)
					}).
					Return(&traefikv1alpha1.IngressRouteUDP{}, nil)
				traefikMock.EXPECT().IngressRouteUDPs(testNamespace).Return(udpClientMock)
			},
			expErr: false,
		},
		{
			name: "ignore unsupported protocol",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-2222", ServiceName: "svc", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
				{Name: "svc-9999", ServiceName: "svc", Protocol: "SCTP", Port: 9999, TargetPort: 9999},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError).Times(2)

				tcpClientMock := newMockIngressrouteTcpInterface(t)
				tcpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Return(&traefikv1alpha1.IngressRouteTCP{}, nil)
				traefikMock.EXPECT().IngressRouteTCPs(testNamespace).Return(tcpClientMock)
			},
			expErr: false,
		},
		{
			name: "update IngressRouteTCP when it already exists",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-2222", ServiceName: "svc", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError)

				existing := &traefikv1alpha1.IngressRouteTCP{}
				existing.ResourceVersion = "1"

				tcpClientMock := newMockIngressrouteTcpInterface(t)
				tcpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "svc-2222-tcp"))
				tcpClientMock.EXPECT().Get(mock.Anything, "svc-2222-tcp", mock.Anything).
					Return(existing, nil)
				tcpClientMock.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
					Run(func(ctx context.Context, route *traefikv1alpha1.IngressRouteTCP, opts metav1.UpdateOptions) {
						require.Equal(t, "1", route.ResourceVersion)
					}).
					Return(&traefikv1alpha1.IngressRouteTCP{}, nil)
				traefikMock.EXPECT().IngressRouteTCPs(testNamespace).Return(tcpClientMock)
			},
			expErr: false,
		},
		{
			name: "update IngressRouteUDP when it already exists",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-5353", ServiceName: "svc", Protocol: corev1.ProtocolUDP, Port: 5353, TargetPort: 5353},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError)

				existing := &traefikv1alpha1.IngressRouteUDP{}
				existing.ResourceVersion = "42"

				udpClientMock := newMockIngressrouteUdpInterface(t)
				udpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "svc-5353-udp"))
				udpClientMock.EXPECT().Get(mock.Anything, "svc-5353-udp", mock.Anything).
					Return(existing, nil)
				udpClientMock.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
					Run(func(ctx context.Context, route *traefikv1alpha1.IngressRouteUDP, opts metav1.UpdateOptions) {
						require.Equal(t, "42", route.ResourceVersion)
					}).
					Return(&traefikv1alpha1.IngressRouteUDP{}, nil)
				traefikMock.EXPECT().IngressRouteUDPs(testNamespace).Return(udpClientMock)
			},
			expErr: false,
		},
		{
			name: "set owner references from ingress on IngressRouteTCP",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-2222", ServiceName: "svc", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ownerRef := metav1.OwnerReference{Name: "some-owner", UID: "abc123"}
				ingressObj := &networkingv1.Ingress{}
				ingressObj.SetOwnerReferences([]metav1.OwnerReference{ownerRef})

				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(ingressObj, nil)

				tcpClientMock := newMockIngressrouteTcpInterface(t)
				tcpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Run(func(ctx context.Context, route *traefikv1alpha1.IngressRouteTCP, opts metav1.CreateOptions) {
						require.Len(t, route.GetOwnerReferences(), 1)
						require.Equal(t, "some-owner", route.GetOwnerReferences()[0].Name)
					}).
					Return(&traefikv1alpha1.IngressRouteTCP{}, nil)
				traefikMock.EXPECT().IngressRouteTCPs(testNamespace).Return(tcpClientMock)
			},
			expErr: false,
		},
		{
			name: "return error when IngressRouteTCP cannot be created",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-2222", ServiceName: "svc", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError)

				tcpClientMock := newMockIngressrouteTcpInterface(t)
				tcpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, assert.AnError)
				traefikMock.EXPECT().IngressRouteTCPs(testNamespace).Return(tcpClientMock)
			},
			expErr:    true,
			expErrStr: "failed to expose tcp port",
		},
		{
			name: "return error when IngressRouteTCP already exists but cannot be fetched for update",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-2222", ServiceName: "svc", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError)

				tcpClientMock := newMockIngressrouteTcpInterface(t)
				tcpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "svc-2222-tcp"))
				tcpClientMock.EXPECT().Get(mock.Anything, "svc-2222-tcp", mock.Anything).
					Return(nil, assert.AnError)
				traefikMock.EXPECT().IngressRouteTCPs(testNamespace).Return(tcpClientMock)
			},
			expErr:    true,
			expErrStr: "failed to expose tcp port",
		},
		{
			name: "return error when IngressRouteTCP cannot be updated",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-2222", ServiceName: "svc", Protocol: corev1.ProtocolTCP, Port: 2222, TargetPort: 2222},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError)

				existing := &traefikv1alpha1.IngressRouteTCP{}
				existing.ResourceVersion = "1"

				tcpClientMock := newMockIngressrouteTcpInterface(t)
				tcpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, apierrors.NewAlreadyExists(schema.GroupResource{}, "svc-2222-tcp"))
				tcpClientMock.EXPECT().Get(mock.Anything, "svc-2222-tcp", mock.Anything).
					Return(existing, nil)
				tcpClientMock.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, assert.AnError)
				traefikMock.EXPECT().IngressRouteTCPs(testNamespace).Return(tcpClientMock)
			},
			expErr:    true,
			expErrStr: "failed to expose tcp port",
		},
		{
			name: "return error when IngressRouteUDP cannot be created",
			inExposedPorts: types.ExposedPorts{
				{Name: "svc-5353", ServiceName: "svc", Protocol: corev1.ProtocolUDP, Port: 5353, TargetPort: 5353},
			},
			setupMocks: func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {
				ingressMock.EXPECT().Get(mock.Anything, "svc", mock.Anything).Return(nil, assert.AnError)

				udpClientMock := newMockIngressrouteUdpInterface(t)
				udpClientMock.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
					Return(nil, assert.AnError)
				traefikMock.EXPECT().IngressRouteUDPs(testNamespace).Return(udpClientMock)
			},
			expErr:    true,
			expErrStr: "failed to expose udp port",
		},
		{
			name:           "handle empty exposed ports list",
			inExposedPorts: types.ExposedPorts{},
			setupMocks:     func(traefikMock *mockTraefikInterface, ingressMock *mockIngressInterface) {},
			expErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traefikMock := newMockTraefikInterface(t)
			ingressMock := newMockIngressInterface(t)
			tt.setupMocks(traefikMock, ingressMock)

			sut := PortExposer{
				traefikInterface: traefikMock,
				ingressInterface: ingressMock,
				namespace:        testNamespace,
			}

			err := sut.ExposePorts(context.TODO(), testNamespace, tt.inExposedPorts)

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.expErrStr)
				return
			}

			require.NoError(t, err)
		})
	}
}

func assertIngressRouteTCP(t *testing.T, route *traefikv1alpha1.IngressRouteTCP, serviceName string, port, targetPort int32) {
	t.Helper()

	require.NotNil(t, route)
	require.Equal(t, testNamespace, route.Namespace)
	require.Equal(t, util.K8sCesServiceDiscoveryLabels, route.Labels)
	require.Equal(t, fmt.Sprintf("%s-%d-tcp", serviceName, port), route.Name)

	require.Len(t, route.Spec.EntryPoints, 1)
	require.Equal(t, fmt.Sprintf("tcp-%d", port), route.Spec.EntryPoints[0])

	require.Len(t, route.Spec.Routes, 1)
	require.Equal(t, "HostSNI(`*`)", route.Spec.Routes[0].Match)

	require.Len(t, route.Spec.Routes[0].Services, 1)
	svc := route.Spec.Routes[0].Services[0]
	require.Equal(t, serviceName, svc.Name)
	require.Equal(t, testNamespace, svc.Namespace)
	require.Equal(t, intstr.FromInt32(targetPort), svc.Port)
}

func assertIngressRouteUDP(t *testing.T, route *traefikv1alpha1.IngressRouteUDP, serviceName string, port, targetPort int32) {
	t.Helper()

	require.NotNil(t, route)
	require.Equal(t, testNamespace, route.Namespace)
	require.Equal(t, util.K8sCesServiceDiscoveryLabels, route.Labels)
	require.Equal(t, fmt.Sprintf("%s-%d-udp", serviceName, port), route.Name)

	require.Len(t, route.Spec.EntryPoints, 1)
	require.Equal(t, fmt.Sprintf("udp-%d", port), route.Spec.EntryPoints[0])

	require.Len(t, route.Spec.Routes, 1)
	require.Len(t, route.Spec.Routes[0].Services, 1)
	svc := route.Spec.Routes[0].Services[0]
	require.Equal(t, serviceName, svc.Name)
	require.Equal(t, testNamespace, svc.Namespace)
	require.Equal(t, intstr.FromInt32(targetPort), svc.Port)
}
