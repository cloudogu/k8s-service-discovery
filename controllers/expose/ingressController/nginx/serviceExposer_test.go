package nginx

import (
	"context"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	testNamespace = "ecosystem"
)

var (
	testCtx = context.Background()
)

func TestNewIngressNginxTCPUDPExposer(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)

		// when
		sut := NewIngressNginxTCPUDPExposer(configMapInterfaceMock)

		// then
		require.NotNil(t, sut)
		assert.Equal(t, configMapInterfaceMock, sut.configMapInterface)
	})
}

func Test_getConfigMapNameForProtocol(t *testing.T) {
	t.Run("should return the protocol in lower case with -services suffix", func(t *testing.T) {
		// when
		result := getConfigMapNameForProtocol(corev1.ProtocolTCP)

		// then
		require.Equal(t, "tcp-services", result)
	})
}

func Test_getServiceEntryKey(t *testing.T) {
	t.Run("should return the host port as string", func(t *testing.T) {
		// when
		result := getServiceEntryKey(util.ExposedPort{Port: 2222})

		// then
		require.Equal(t, "2222", result)
	})
}

func Test_getServiceEntryValue(t *testing.T) {
	t.Run("should return the namespace/servicename:containerport as string", func(t *testing.T) {
		// when
		result := getServiceEntryValue("ecosystem", "scm", util.ExposedPort{Port: 2222, TargetPort: 2222})

		// then
		require.Equal(t, "ecosystem/scm:2222", result)
	})
}

func Test_getServiceEntryValuePrefix(t *testing.T) {
	t.Run("should return the testNamespace/servicename", func(t *testing.T) {
		// when
		result := getServiceEntryValuePrefix("ecosystem", "scm")

		// then
		require.Equal(t, "ecosystem/scm", result)
	})
}

func Test_getExposedPortsByType(t *testing.T) {
	type args struct {
		exposedPorts util.ExposedPorts
		protocol     corev1.Protocol
	}
	tests := []struct {
		name string
		args args
		want util.ExposedPorts
	}{
		{name: "should return nil slice with no exposed ports", args: args{exposedPorts: util.ExposedPorts{}, protocol: corev1.ProtocolTCP}, want: util.ExposedPorts(nil)},
		{name: "should return nil slice with just udp ports", args: args{exposedPorts: util.ExposedPorts{{Port: 2222, Protocol: corev1.ProtocolUDP}}, protocol: corev1.ProtocolTCP}, want: util.ExposedPorts(nil)},
		{name: "should return nil slice with just udp ports without http or https", args: args{exposedPorts: util.ExposedPorts{{Port: 2222, Protocol: corev1.ProtocolUDP}, {Port: 80, Protocol: corev1.ProtocolUDP}, {Port: 443, Protocol: corev1.ProtocolUDP}}, protocol: "tcp"}, want: util.ExposedPorts(nil)},
		{name: "should return nil slice with just tcp ports", args: args{exposedPorts: util.ExposedPorts{{Port: 2222, Protocol: corev1.ProtocolTCP}}, protocol: corev1.ProtocolUDP}, want: util.ExposedPorts(nil)},
		{name: "should return nil slice with just tcp ports without http or https", args: args{exposedPorts: util.ExposedPorts{{Port: 2222, Protocol: corev1.ProtocolTCP}, {Port: 80, Protocol: corev1.ProtocolTCP}, {Port: 443, Protocol: corev1.ProtocolTCP}}, protocol: corev1.ProtocolUDP}, want: util.ExposedPorts(nil)},
		{name: "should return just tcp ports", args: args{exposedPorts: util.ExposedPorts{{Port: 2222, Protocol: corev1.ProtocolTCP}, {Port: 3333, Protocol: corev1.ProtocolUDP}}, protocol: corev1.ProtocolTCP}, want: util.ExposedPorts{{Port: 2222, Protocol: corev1.ProtocolTCP}}},
		{name: "should return just udp ports", args: args{exposedPorts: util.ExposedPorts{{Port: 2222, Protocol: corev1.ProtocolTCP}, {Port: 3333, Protocol: corev1.ProtocolUDP}}, protocol: corev1.ProtocolUDP}, want: util.ExposedPorts{{Port: 3333, Protocol: corev1.ProtocolUDP}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, getExposedPortsByType(tt.args.exposedPorts, tt.args.protocol), "getExposedPortsByType(%v, %v)", tt.args.exposedPorts, tt.args.protocol)
		})
	}
}

func Test_filterServices(t *testing.T) {
	emptyCm := &corev1.ConfigMap{}
	emptyMap := map[string]string{}
	scmCm := &corev1.ConfigMap{Data: map[string]string{"2222": "ecosystem/scm:2222"}}
	mixedCm := &corev1.ConfigMap{Data: map[string]string{"2222": "ecosystem/scm:2222", "3333": "ecosystem/ldap:3333"}}

	type args struct {
		cm          *corev1.ConfigMap
		namespace   string
		serviceName string
	}

	tests := []struct {
		name string
		args args
		want map[string]string
	}{
		{"should return empty map on empty cm", args{
			cm:          emptyCm,
			namespace:   testNamespace,
			serviceName: "scm",
		}, emptyMap},
		{"should return empty map on cm with only service ports", args{
			cm:          scmCm,
			namespace:   testNamespace,
			serviceName: "scm",
		}, emptyMap},
		{"should leave other ports", args{
			cm:          mixedCm,
			namespace:   testNamespace,
			serviceName: "scm",
		}, map[string]string{"3333": "ecosystem/ldap:3333"}},
		{"should remove all ports from service in testNamespace", args{
			cm:          scmCm,
			namespace:   testNamespace,
			serviceName: "scm",
		}, emptyMap},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, filterServices(tt.args.cm, tt.args.namespace, tt.args.serviceName), "filterServices(%v, %v, %v)", tt.args.cm, tt.args.namespace, tt.args.serviceName)
		})
	}
}

var (
	testMixedExposedPorts = util.ExposedPorts{
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       2222,
			TargetPort: 3333,
		},
		{
			Protocol:   corev1.ProtocolTCP,
			Port:       8888,
			TargetPort: 7777,
		},
		{
			Protocol:   corev1.ProtocolUDP,
			Port:       3333,
			TargetPort: 4444,
		},
	}
	testUDPExposedPorts = util.ExposedPorts{
		{
			Protocol:   corev1.ProtocolUDP,
			Port:       2222,
			TargetPort: 3333,
		},
	}
	testTargetServiceName = "ldap"
)

func TestIngressNginxTcpUpdExposer_ExposeOrUpdateExposedServices(t *testing.T) {
	t.Run("success with no existent configmaps", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "not found"))
		configMapInterfaceMock.EXPECT().Get(testCtx, "udp-services", metav1.GetOptions{}).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "not found"))
		expectedTCPConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "tcp-services", Namespace: testNamespace},
			Data: map[string]string{
				"2222": "ecosystem/ldap:3333",
				"8888": "ecosystem/ldap:7777",
			},
		}
		expectedUDPConfigMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "udp-services", Namespace: testNamespace},
			Data: map[string]string{
				"3333": "ecosystem/ldap:4444",
			},
		}
		configMapInterfaceMock.EXPECT().Create(testCtx, expectedTCPConfigMap, metav1.CreateOptions{}).Return(nil, nil)
		configMapInterfaceMock.EXPECT().Create(testCtx, expectedUDPConfigMap, metav1.CreateOptions{}).Return(nil, nil)
		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.ExposeOrUpdateExposedPorts(testCtx, testNamespace, testTargetServiceName, testMixedExposedPorts)

		// then
		require.NoError(t, err)
	})

	t.Run("should return nil if the service has no exposed ports", func(t *testing.T) {
		// given
		sut := ingressNginxTcpUpdExposer{}

		// when
		err := sut.ExposeOrUpdateExposedPorts(context.TODO(), testNamespace, testTargetServiceName, util.ExposedPorts{})

		// then
		require.Nil(t, err)
	})

	t.Run("should throw an error getting tcp-configmap", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(nil, assert.AnError)
		sut := ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.ExposeOrUpdateExposedPorts(testCtx, testNamespace, testTargetServiceName, testMixedExposedPorts)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get configmap tcp-services")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should throw an error getting udp-configmap", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "not found"))
		configMapInterfaceMock.EXPECT().Get(testCtx, "udp-services", metav1.GetOptions{}).Return(nil, assert.AnError)
		sut := ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.ExposeOrUpdateExposedPorts(testCtx, testNamespace, testTargetServiceName, testUDPExposedPorts)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get configmap udp-services")
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func Test_ingressNginxTcpUpdExposer_exposeOrUpdatePortsForProtocol(t *testing.T) {
	t.Run("should return nil if no legacy ports are in configmap and the service doesnt contain new ports", func(t *testing.T) {
		// given
		tcpCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "tcp-services", Namespace: testNamespace}, Data: map[string]string{"2222": "ecosystem/notldap:3333"}}
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(tcpCm, nil)
		sut := ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.exposeOrUpdatePortsForProtocol(testCtx, testNamespace, testTargetServiceName, testUDPExposedPorts, corev1.ProtocolTCP)

		// then
		require.Nil(t, err)
	})

	t.Run("should return error on update failure", func(t *testing.T) {
		// given
		tcpCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "udp-services", Namespace: testNamespace}, Data: map[string]string{"2222": "ecosystem/notldap:3333"}}
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "udp-services", metav1.GetOptions{}).Return(tcpCm, nil)
		configMapInterfaceMock.EXPECT().Update(testCtx, mock.IsType(&corev1.ConfigMap{}), metav1.UpdateOptions{}).Return(nil, assert.AnError)
		sut := ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.exposeOrUpdatePortsForProtocol(testCtx, testNamespace, testTargetServiceName, testUDPExposedPorts, corev1.ProtocolUDP)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to update configmap")
	})

	t.Run("should return error on creation failure", func(t *testing.T) {
		// given
		tcpCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "udp-services", Namespace: testNamespace}, Data: map[string]string{"2222": "ecosystem/ldap:3333"}}
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "udp-services", metav1.GetOptions{}).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "not found"))
		configMapInterfaceMock.EXPECT().Create(testCtx, tcpCm, metav1.CreateOptions{}).Return(nil, assert.AnError)
		sut := ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.exposeOrUpdatePortsForProtocol(testCtx, testNamespace, testTargetServiceName, testUDPExposedPorts, corev1.ProtocolUDP)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create configmap udp-services")
	})
}

func Test_ingressNginxTcpUpdExposer_createNginxExposeConfigMapForProtocol(t *testing.T) {
	t.Run("should return nil if the service contains no matching protocol ports", func(t *testing.T) {
		// given
		sut := &ingressNginxTcpUpdExposer{}

		// when
		cm, err := sut.createNginxExposeConfigMapForProtocol(context.TODO(), "ecosystem", testTargetServiceName, testUDPExposedPorts, corev1.ProtocolTCP)

		// then
		require.Nil(t, err)
		require.Nil(t, cm)
	})
}

func Test_ingressNginxTcpUpdExposer_DeleteExposedServices(t *testing.T) {
	t.Run("should return nil if the service doesnt contain exposed ports", func(t *testing.T) {
		// given
		tcpCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "tcp-services", Namespace: testNamespace}, Data: map[string]string{"1234": "ecosystem/notldap:1234"}}
		udpCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "udp-services", Namespace: testNamespace}, Data: map[string]string{"1234": "ecosystem/notldap:1234"}}
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(tcpCm, nil)
		configMapInterfaceMock.EXPECT().Get(testCtx, "udp-services", metav1.GetOptions{}).Return(udpCm, nil)
		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.DeleteExposedPorts(testCtx, testNamespace, testTargetServiceName)

		// then
		require.Nil(t, err)
	})

	t.Run("should return error on getting tcp-services configmap failure", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(nil, assert.AnError)
		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.DeleteExposedPorts(testCtx, testNamespace, testTargetServiceName)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get configmap tcp-services")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should return error on getting udp-services configmap failure", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(&corev1.ConfigMap{}, nil)
		configMapInterfaceMock.EXPECT().Get(testCtx, "udp-services", metav1.GetOptions{}).Return(nil, assert.AnError)
		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.DeleteExposedPorts(testCtx, testNamespace, testTargetServiceName)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get configmap udp-services")
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func Test_ingressNginxTcpUpdExposer_deletePortsForProtocol(t *testing.T) {
	t.Run("return nil if configmap is not found", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "not-found"))
		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.deletePortsForProtocolWithRetry(testCtx, testNamespace, testTargetServiceName, corev1.ProtocolTCP)

		// then
		require.Nil(t, err)
	})

	t.Run("return nil if configmap has nil data", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(&corev1.ConfigMap{}, nil)
		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}
		// when
		err := sut.deletePortsForProtocolWithRetry(testCtx, testNamespace, testTargetServiceName, corev1.ProtocolTCP)

		// then
		require.Nil(t, err)
	})

	t.Run("return nil if configmap has no data", func(t *testing.T) {
		// given
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(&corev1.ConfigMap{Data: map[string]string{}}, nil)
		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}
		// when
		err := sut.deletePortsForProtocolWithRetry(testCtx, testNamespace, testTargetServiceName, corev1.ProtocolTCP)

		// then
		require.Nil(t, err)
	})

	t.Run("should delete all service entries from configmap", func(t *testing.T) {
		// given
		tcpCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "tcp-services", Namespace: testNamespace}, Data: map[string]string{"2222": "ecosystem/ldap:3333", "1234": "ecosystem/notldap:1234"}}
		updatedCm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "tcp-services", Namespace: testNamespace}, Data: map[string]string{"1234": "ecosystem/notldap:1234"}}
		configMapInterfaceMock := newMockConfigMapInterface(t)
		configMapInterfaceMock.EXPECT().Get(testCtx, "tcp-services", metav1.GetOptions{}).Return(tcpCm, nil)
		configMapInterfaceMock.EXPECT().Update(testCtx, updatedCm, metav1.UpdateOptions{}).Return(nil, nil)

		sut := &ingressNginxTcpUpdExposer{configMapInterface: configMapInterfaceMock}

		// when
		err := sut.deletePortsForProtocolWithRetry(testCtx, testNamespace, testTargetServiceName, corev1.ProtocolTCP)

		// then
		require.Nil(t, err)
	})
}
