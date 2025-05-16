package expose

import (
	"context"
	"fmt"
	doguv2 "github.com/cloudogu/k8s-dogu-operator/v2/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strings"
	"testing"
)

var (
	testCIDR    = "0.0.0.0/0"
	netPolName  = "nginx-ingress-exposed"
	ingressName = "nginx-ingress"
	intStr80    = intstr.Parse("80")
	intStr443   = intstr.Parse("443")
	intStr5000  = intstr.Parse("5000")
	intStr5001  = intstr.Parse("5001")
	intStr5002  = intstr.Parse("5002")
	tcpProtocol = corev1.ProtocolTCP
	udpProtocol = corev1.ProtocolUDP

	jenkinsServiceName  = "jenkins"
	jenkinsExposedPorts = util.ExposedPorts{
		{Port: 5000, Protocol: corev1.ProtocolTCP, TargetPort: 5000},
	}
	updatedJenkinsExposedPorts = util.ExposedPorts{
		{Port: 5001, Protocol: corev1.ProtocolUDP, TargetPort: 5001},
		{Port: 5002, Protocol: corev1.ProtocolUDP, TargetPort: 5002},
	}

	serviceWithoutExposedPorts = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-ingress",
			Namespace: testNamespace,
		},
	}

	jenkinsExposedService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "jenkins",
			Namespace:   testNamespace,
			Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": `[{"protocol":"TCP","port":5000,"targetPort":5000}]`},
		},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
			{
				Port:       5000,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 5000},
				Protocol:   corev1.ProtocolTCP,
			},
		}},
	}

	nginxExposedService = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "nginx-ingress",
			Namespace:   testNamespace,
			Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": `[{"protocol":"TCP","port":80,"targetPort":80},{"protocol":"TCP","port":443,"targetPort":443}]`},
		},
		Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
			{
				Port:       80,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 80},
				Protocol:   corev1.ProtocolTCP,
			},
			{
				Port:       443,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 443},
				Protocol:   corev1.ProtocolTCP,
			},
		}},
	}
)

func Test_networkPolicyHandler_updateNetworkPolicy(t *testing.T) {
	initialNetpol, _, jenkinsNetpol, updatedJenkinsNetpol := getTestNetworkPolicies()

	type fields struct {
		mockNetworkPolicyInterface func() networkPolicyInterface
		mockIngressController      func() ingressController
		allowedCIDR                string
	}
	type args struct {
		ctx          context.Context
		serviceName  string
		exposedPorts util.ExposedPorts
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "should add port on new service",
			fields: fields{
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(initialNetpol, nil)
					networkPolicyInterfaceMock.EXPECT().Update(testCtx, jenkinsNetpol, metav1.UpdateOptions{}).Return(nil, nil)

					return networkPolicyInterfaceMock
				},
				mockIngressController: func() ingressController {
					ingressControllerMock := newMockIngressController(t)
					ingressControllerMock.EXPECT().GetName().Return("nginx-ingress")
					return ingressControllerMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:          testCtx,
				serviceName:  jenkinsServiceName,
				exposedPorts: jenkinsExposedPorts,
			},
			wantErr: assert.NoError,
		},
		{
			name: "should delete obsolete ports and add new",
			fields: fields{
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(jenkinsNetpol, nil)
					networkPolicyInterfaceMock.EXPECT().Update(testCtx, updatedJenkinsNetpol, metav1.UpdateOptions{}).Return(nil, nil)

					return networkPolicyInterfaceMock
				},
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:          testCtx,
				serviceName:  jenkinsServiceName,
				exposedPorts: updatedJenkinsExposedPorts,
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nph := &networkPolicyHandler{
				ingressController:      tt.fields.mockIngressController(),
				networkPolicyInterface: tt.fields.mockNetworkPolicyInterface(),
				allowedCIDR:            tt.fields.allowedCIDR,
			}
			tt.wantErr(t, nph.updateNetworkPolicy(tt.args.ctx, tt.args.serviceName, tt.args.exposedPorts), fmt.Sprintf("updateNetworkPolicy(%v, %v, %v)", tt.args.ctx, tt.args.serviceName, tt.args.exposedPorts))
		})
	}
}

func Test_networkPolicyHandler_RemoveExposedPorts(t *testing.T) {
	initialNetpol, invalidAnnotationsNetpol, jenkinsNetpol, _ := getTestNetworkPolicies()
	type fields struct {
		mockIngressController      func() ingressController
		mockNetworkPolicyInterface func() networkPolicyInterface
		allowedCIDR                string
	}
	type args struct {
		ctx         context.Context
		serviceName string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr func(t *testing.T, err error, msg string)
	}{
		{
			name: "should delete exposed ports",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(jenkinsNetpol, nil)
					networkPolicyInterfaceMock.EXPECT().Update(testCtx, getInitialNetpolWithCIDR(testCIDR), metav1.UpdateOptions{}).Return(nil, nil)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:         testCtx,
				serviceName: jenkinsServiceName,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.NoError(t, err, msg)
			},
		},
		{
			name: "should do nothing if the networkpolicy doesnt exist",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:         testCtx,
				serviceName: jenkinsServiceName,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.NoError(t, err, msg)
			},
		},
		{
			name: "should return error on error getting networkpolicy",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:         testCtx,
				serviceName: jenkinsServiceName,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.Error(t, err, msg)
				assert.ErrorContains(t, err, "failed to get networkpolicy nginx-ingress-exposed")
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "should return error if the annotations are invalid",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(invalidAnnotationsNetpol, nil)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:         testCtx,
				serviceName: jenkinsServiceName,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.Error(t, err, msg)
				assert.ErrorContains(t, err, "failed to unmarshal service port mapping [{\"protocol\":80}] for service jenkins")
			},
		},
		{
			name: "should do nothing if no ports are defined in networkpolicy",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(initialNetpol, nil)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:         testCtx,
				serviceName: jenkinsServiceName,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.NoError(t, err, msg)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nph := &networkPolicyHandler{
				ingressController:      tt.fields.mockIngressController(),
				networkPolicyInterface: tt.fields.mockNetworkPolicyInterface(),
				allowedCIDR:            tt.fields.allowedCIDR,
			}
			tt.wantErr(t, nph.RemoveExposedPorts(tt.args.ctx, tt.args.serviceName), fmt.Sprintf("RemoveExposedPorts(%v, %v)", tt.args.ctx, tt.args.serviceName))
		})
	}
}

func Test_networkPolicyHandler_UpsertNetworkPoliciesForService(t *testing.T) {
	initialNetpol, _, jenkinsNetpol, _ := getTestNetworkPolicies()
	type fields struct {
		mockIngressController      func() ingressController
		mockNetworkPolicyInterface func() networkPolicyInterface
		allowedCIDR                string
	}
	type args struct {
		ctx     context.Context
		service *corev1.Service
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr func(t *testing.T, err error, msg string)
	}{
		{
			name: "should create networkpolicy if no exists",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
					networkPolicyInterfaceMock.EXPECT().Create(testCtx, getInitialNetpolWithCIDR(testCIDR), metav1.CreateOptions{}).Return(nil, nil)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     testCtx,
				service: nginxExposedService,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.NoError(t, err, msg)
			},
		},
		{
			name: "should return error on error creating networkpolicy",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
					networkPolicyInterfaceMock.EXPECT().Create(testCtx, getInitialNetpolWithCIDR(testCIDR), metav1.CreateOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     testCtx,
				service: nginxExposedService,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.Error(t, err, msg)
				assert.ErrorIs(t, err, assert.AnError)
				assert.ErrorContains(t, err, "failed to create networkpolicy nginx-ingress-exposed")
			},
		},
		{
			name: "should not create networkpolicy if no exposed ports exist",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     testCtx,
				service: serviceWithoutExposedPorts,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.NoError(t, err, msg)
			},
		},
		{
			name: "should return error on error getting networkpolicy",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     testCtx,
				service: nginxExposedService,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.Error(t, err, msg)
				assert.ErrorIs(t, err, assert.AnError)
				assert.ErrorContains(t, err, "failed to get networkpolicy nginx-ingress-exposed")
			},
		},
		{
			name: "should return error on error updating networkpolicy",
			fields: fields{
				mockIngressController: func() ingressController {
					return getIngressControllerMock(t)
				},
				mockNetworkPolicyInterface: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(initialNetpol, nil)
					networkPolicyInterfaceMock.EXPECT().Update(testCtx, jenkinsNetpol, metav1.UpdateOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     testCtx,
				service: jenkinsExposedService,
			},
			wantErr: func(t *testing.T, err error, msg string) {
				require.Error(t, err, msg)
				assert.ErrorIs(t, err, assert.AnError)
				assert.ErrorContains(t, err, "failed to update networkpolicy")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nph := &networkPolicyHandler{
				ingressController:      tt.fields.mockIngressController(),
				networkPolicyInterface: tt.fields.mockNetworkPolicyInterface(),
				allowedCIDR:            tt.fields.allowedCIDR,
			}
			tt.wantErr(t, nph.UpsertNetworkPoliciesForService(tt.args.ctx, tt.args.service), fmt.Sprintf("UpsertNetworkPoliciesForService(%v, %v)", tt.args.ctx, tt.args.service))
		})
	}
}

func TestNewNetworkPolicyHandler(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		netInterfaceMock := newMockNetworkPolicyInterface(t)
		ingressControllerMock := newMockIngressController(t)

		// when
		handler := NewNetworkPolicyHandler(netInterfaceMock, ingressControllerMock, "0.0.0.0/0")

		// then
		require.NotNil(t, handler)
		assert.Equal(t, netInterfaceMock, handler.networkPolicyInterface)
		assert.Equal(t, ingressControllerMock, handler.ingressController)
		assert.Equal(t, "0.0.0.0/0", handler.allowedCIDR)
	})
}

func getIngressControllerMock(t *testing.T) ingressController {
	ingressControllerMock := newMockIngressController(t)
	ingressControllerMock.EXPECT().GetName().Return(ingressName)

	return ingressControllerMock
}

func getTestNetworkPolicies() (initialNetpol, invalidAnnotationsNetpol, jenkinsNetpol, updatedJenkinsNetpol *netv1.NetworkPolicy) {
	initialNetpol = getInitialNetpolWithCIDR("10.0.0.0/8")
	invalidAnnotationsNetpol = getNetPol(netPolName, map[string]string{"k8s.cloudogu.com/ces-exposed-ports-jenkins": `[{"protocol":80}]`}, []netv1.NetworkPolicyPort{}, testCIDR)
	jenkinsNetpol = getNetPol(netPolName, map[string]string{
		"k8s.cloudogu.com/ces-exposed-ports-nginx-ingress": `[{"protocol":"TCP","port":80,"targetPort":80},{"protocol":"TCP","port":443,"targetPort":443}]`,
		"k8s.cloudogu.com/ces-exposed-ports-jenkins":       `[{"protocol":"TCP","port":5000,"targetPort":5000}]`},
		[]netv1.NetworkPolicyPort{
			{
				Port:     &intStr80,
				Protocol: &tcpProtocol,
			},
			{
				Port:     &intStr443,
				Protocol: &tcpProtocol,
			},
			{
				Port:     &intStr5000,
				Protocol: &tcpProtocol,
			},
		}, testCIDR)
	updatedJenkinsNetpol = getNetPol(netPolName, map[string]string{
		"k8s.cloudogu.com/ces-exposed-ports-nginx-ingress": `[{"protocol":"TCP","port":80,"targetPort":80},{"protocol":"TCP","port":443,"targetPort":443}]`,
		"k8s.cloudogu.com/ces-exposed-ports-jenkins":       `[{"protocol":"UDP","port":5001,"targetPort":5001},{"protocol":"UDP","port":5002,"targetPort":5002}]`},
		[]netv1.NetworkPolicyPort{
			{
				Port:     &intStr80,
				Protocol: &tcpProtocol,
			},
			{
				Port:     &intStr443,
				Protocol: &tcpProtocol,
			},
			{
				Port:     &intStr5001,
				Protocol: &udpProtocol,
			},
			{
				Port:     &intStr5002,
				Protocol: &udpProtocol,
			},
		}, testCIDR)
	return
}

func getNetPol(netpolName string, annotations map[string]string, ports []netv1.NetworkPolicyPort, cidr string) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        netpolName,
			Namespace:   testNamespace,
			Annotations: annotations,
			Labels:      map[string]string{"app": "ces", "app.kubernetes.io/name": "k8s-service-discovery"},
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{doguv2.DoguLabelName: "nginx-ingress"}},
			PolicyTypes: []netv1.PolicyType{netv1.PolicyTypeIngress},
			Ingress: []netv1.NetworkPolicyIngressRule{
				{
					Ports: ports,
					From: []netv1.NetworkPolicyPeer{
						{
							IPBlock: &netv1.IPBlock{
								CIDR: cidr,
							},
						},
					},
				},
			},
		},
	}
}

func getInitialNetpolWithCIDR(cidr string) *netv1.NetworkPolicy {
	return getNetPol(netPolName, map[string]string{"k8s.cloudogu.com/ces-exposed-ports-nginx-ingress": `[{"protocol":"TCP","port":80,"targetPort":80},{"protocol":"TCP","port":443,"targetPort":443}]`},
		[]netv1.NetworkPolicyPort{
			{
				Port:     &intStr80,
				Protocol: &tcpProtocol,
			},
			{
				Port:     &intStr443,
				Protocol: &tcpProtocol,
			},
		}, cidr)
}

func Test_getServicePortMappingAnnotationKey(t *testing.T) {
	t.Run("should generate generate name with more than 63 chars including prefix", func(t *testing.T) {
		// given
		extraLongServiceName := "testtesttesttesttesttesttesttesttesttesttestttttt"
		// service name with length > 45 and <= 63 are legit but must be shortened with our prefix.
		assert.Len(t, extraLongServiceName, 49)

		// when
		key := getServicePortMappingAnnotationKey(extraLongServiceName)

		// remove dns prefix
		name := strings.ReplaceAll(key, "k8s.cloudogu.com/", "")

		// then
		assert.Len(t, name, 63)
	})
}

func Test_networkPolicyHandler_RemoveNetworkPolicy(t *testing.T) {
	netpol, _, _, _ := getTestNetworkPolicies()

	type fields struct {
		ingressControllerMock      func() ingressController
		networkPolicyInterfaceMock func() networkPolicyInterface
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr func(t *testing.T, err error, msg string)
	}{
		{
			name: "should delete networkpolicy if existent",
			fields: fields{
				ingressControllerMock: func() ingressController {
					return getIngressControllerMock(t)
				},
				networkPolicyInterfaceMock: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(netpol, nil)
					networkPolicyInterfaceMock.EXPECT().Delete(testCtx, netPolName, metav1.DeleteOptions{}).Return(nil)

					return networkPolicyInterfaceMock
				},
			},
			args: args{ctx: testCtx},
			wantErr: func(t *testing.T, err error, msg string) {
				require.NoError(t, err, msg)
			},
		},
		{
			name: "should do nothing if networkpolicy is no existent",
			fields: fields{
				ingressControllerMock: func() ingressController {
					return getIngressControllerMock(t)
				},
				networkPolicyInterfaceMock: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))

					return networkPolicyInterfaceMock
				},
			},
			args: args{ctx: testCtx},
			wantErr: func(t *testing.T, err error, msg string) {
				require.NoError(t, err, msg)
			},
		},
		{
			name: "should return error on error getting policy",
			fields: fields{
				ingressControllerMock: func() ingressController {
					return getIngressControllerMock(t)
				},
				networkPolicyInterfaceMock: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
			},
			args: args{ctx: testCtx},
			wantErr: func(t *testing.T, err error, msg string) {
				require.Error(t, err, msg)
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "should return error on error deleting policy",
			fields: fields{
				ingressControllerMock: func() ingressController {
					return getIngressControllerMock(t)
				},
				networkPolicyInterfaceMock: func() networkPolicyInterface {
					networkPolicyInterfaceMock := newMockNetworkPolicyInterface(t)
					networkPolicyInterfaceMock.EXPECT().Get(testCtx, netPolName, metav1.GetOptions{}).Return(netpol, nil)
					networkPolicyInterfaceMock.EXPECT().Delete(testCtx, netPolName, metav1.DeleteOptions{}).Return(assert.AnError)

					return networkPolicyInterfaceMock
				},
			},
			args: args{ctx: testCtx},
			wantErr: func(t *testing.T, err error, msg string) {
				require.Error(t, err, msg)
				assert.ErrorIs(t, err, assert.AnError)
				assert.ErrorContains(t, err, "failed to delete network policy nginx-ingress-exposed")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nph := &networkPolicyHandler{
				ingressController:      tt.fields.ingressControllerMock(),
				networkPolicyInterface: tt.fields.networkPolicyInterfaceMock(),
			}
			tt.wantErr(t, nph.RemoveNetworkPolicy(tt.args.ctx), fmt.Sprintf("RemoveNetworkPolicy(%v)", tt.args.ctx))
		})
	}
}
