package expose

import (
	"context"
	"fmt"
	"testing"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
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
		{Port: 5000, Protocol: util.ProtocolTCP, TargetPort: 5000},
	}
	updatedJenkinsExposedPorts = util.ExposedPorts{
		{Port: 5001, Protocol: util.ProtocolUDP, TargetPort: 5001},
		{Port: 5002, Protocol: util.ProtocolUDP, TargetPort: 5002},
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
					networkPolicyInterfaceMock.EXPECT().Get(t.Context(), netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
					networkPolicyInterfaceMock.EXPECT().Create(t.Context(), getInitialNetpolWithCIDR(testCIDR), metav1.CreateOptions{}).Return(nil, nil)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     t.Context(),
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
					networkPolicyInterfaceMock.EXPECT().Get(t.Context(), netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
					networkPolicyInterfaceMock.EXPECT().Create(t.Context(), getInitialNetpolWithCIDR(testCIDR), metav1.CreateOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     t.Context(),
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
					networkPolicyInterfaceMock.EXPECT().Get(t.Context(), netPolName, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     t.Context(),
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
					networkPolicyInterfaceMock.EXPECT().Get(t.Context(), netPolName, metav1.GetOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     t.Context(),
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
					networkPolicyInterfaceMock.EXPECT().Get(t.Context(), netPolName, metav1.GetOptions{}).Return(initialNetpol, nil)
					networkPolicyInterfaceMock.EXPECT().Update(t.Context(), jenkinsNetpol, metav1.UpdateOptions{}).Return(nil, assert.AnError)

					return networkPolicyInterfaceMock
				},
				allowedCIDR: testCIDR,
			},
			args: args{
				ctx:     t.Context(),
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
			nph := &NetworkPolicyHandler{}
			tt.wantErr(t, nph.UpsertNetworkPoliciesForService(tt.args.ctx, tt.args.service), fmt.Sprintf("UpsertNetworkPoliciesForService(%v, %v)", tt.args.ctx, tt.args.service))
		})
	}
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
