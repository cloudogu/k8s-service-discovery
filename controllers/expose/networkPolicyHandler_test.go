package expose

import (
	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
