package controllers

import (
	"math"
	"testing"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func Test_mapPortInt(t *testing.T) {
	tests := []struct {
		name    string
		in      int
		want    int32
		wantErr string
	}{
		{name: "zero", in: 0, want: 0},
		{name: "normal positive", in: 8080, want: 8080},
		{name: "max int32 boundary", in: math.MaxInt32, want: math.MaxInt32},
		{name: "negative rejected", in: -1, wantErr: "number is negative"},
		{name: "above max int32 rejected", in: math.MaxInt32 + 1, wantErr: "number is > 2147483647"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapPortInt(tt.in)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_normalizedProtocol(t *testing.T) {
	tests := []struct {
		name string
		in   corev1.Protocol
		want corev1.Protocol
	}{
		{name: "empty defaults to TCP", in: "", want: corev1.ProtocolTCP},
		{name: "TCP unchanged", in: corev1.ProtocolTCP, want: corev1.ProtocolTCP},
		{name: "UDP unchanged", in: corev1.ProtocolUDP, want: corev1.ProtocolUDP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizedProtocol(tt.in))
		})
	}
}

func Test_equalsServicePortExposedPort(t *testing.T) {
	tests := []struct {
		name        string
		servicePort corev1.ServicePort
		exposedPort types.ExposedPort
		want        bool
	}{
		{
			name:        "matching TCP",
			servicePort: corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80},
			exposedPort: types.ExposedPort{Protocol: corev1.ProtocolTCP, ServicePort: 80},
			want:        true,
		},
		{
			name:        "protocol mismatch",
			servicePort: corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80},
			exposedPort: types.ExposedPort{Protocol: corev1.ProtocolUDP, ServicePort: 80},
			want:        false,
		},
		{
			name:        "port mismatch",
			servicePort: corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80},
			exposedPort: types.ExposedPort{Protocol: corev1.ProtocolTCP, ServicePort: 81},
			want:        false,
		},
		{
			name:        "empty servicePort protocol normalises to TCP and matches",
			servicePort: corev1.ServicePort{Protocol: "", Port: 80},
			exposedPort: types.ExposedPort{Protocol: corev1.ProtocolTCP, ServicePort: 80},
			want:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, equalsServicePortExposedPort(tt.servicePort, tt.exposedPort))
		})
	}
}

func Test_validateExposedPorts(t *testing.T) {
	tests := []struct {
		name         string
		exposedPorts types.ExposedPorts
		servicePorts []corev1.ServicePort
		wantErrs     []string
	}{
		{
			name:         "empty exposed ports yields no error",
			exposedPorts: types.ExposedPorts{},
			servicePorts: nil,
		},
		{
			name: "all match yields no error",
			exposedPorts: types.ExposedPorts{
				{Protocol: corev1.ProtocolTCP, ServicePort: 80},
				{Protocol: corev1.ProtocolUDP, ServicePort: 5000},
			},
			servicePorts: []corev1.ServicePort{
				{Protocol: corev1.ProtocolTCP, Port: 80},
				{Protocol: corev1.ProtocolUDP, Port: 5000},
			},
		},
		{
			name: "one missing reported",
			exposedPorts: types.ExposedPorts{
				{Protocol: corev1.ProtocolTCP, ServicePort: 80},
				{Protocol: corev1.ProtocolUDP, ServicePort: 5000},
			},
			servicePorts: []corev1.ServicePort{
				{Protocol: corev1.ProtocolTCP, Port: 80},
			},
			wantErrs: []string{"port '5000' is not defined in service ports"},
		},
		{
			name: "all missing reported",
			exposedPorts: types.ExposedPorts{
				{Protocol: corev1.ProtocolTCP, ServicePort: 80},
				{Protocol: corev1.ProtocolUDP, ServicePort: 5000},
			},
			servicePorts: nil,
			wantErrs: []string{
				"port '80' is not defined in service ports",
				"port '5000' is not defined in service ports",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExposedPorts(tt.exposedPorts, tt.servicePorts)
			if len(tt.wantErrs) == 0 {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, w := range tt.wantErrs {
				assert.ErrorContains(t, err, w)
			}
		})
	}
}

func Test_mapServiceExposedPort(t *testing.T) {
	tests := []struct {
		name    string
		svcName string
		dto     ServiceExposedPortDTO
		want    types.ExposedPort
		wantErr string
	}{
		{
			name:    "lowercase tcp success",
			svcName: "ldap",
			dto:     ServiceExposedPortDTO{Protocol: "tcp", Port: 8080, TargetPort: 80},
			want: types.ExposedPort{
				Name:                  "ldap-expose-8080-80",
				ServiceName:           "ldap",
				Protocol:              corev1.ProtocolTCP,
				RequestedExternalPort: 8080,
				ServicePort:           80,
			},
		},
		{
			name:    "UDP success",
			svcName: "ldap",
			dto:     ServiceExposedPortDTO{Protocol: "UDP", Port: 5000, TargetPort: 5000},
			want: types.ExposedPort{
				Name:                  "ldap-expose-5000-5000",
				ServiceName:           "ldap",
				Protocol:              corev1.ProtocolUDP,
				RequestedExternalPort: 5000,
				ServicePort:           5000,
			},
		},
		{
			name:    "negative port rejected",
			svcName: "ldap",
			dto:     ServiceExposedPortDTO{Protocol: "TCP", Port: -1, TargetPort: 80},
			wantErr: "port is invalid",
		},
		{
			name:    "targetPort too large rejected",
			svcName: "ldap",
			dto:     ServiceExposedPortDTO{Protocol: "TCP", Port: 8080, TargetPort: math.MaxInt32 + 1},
			wantErr: "targetPort is invalid",
		},
		{
			name:    "unknown protocol rejected",
			svcName: "ldap",
			dto:     ServiceExposedPortDTO{Protocol: "ICMP", Port: 8080, TargetPort: 80},
			wantErr: "unsupported protocol for exposed port: ICMP",
		},
		{
			name:    "SCTP rejected (controllers package only allows TCP/UDP)",
			svcName: "ldap",
			dto:     ServiceExposedPortDTO{Protocol: "SCTP", Port: 8080, TargetPort: 80},
			wantErr: "unsupported protocol for exposed port: SCTP",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapServiceExposedPort(tt.svcName, tt.dto)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_mapExposedPortList(t *testing.T) {
	tests := []struct {
		name    string
		svcName string
		dtos    []ServiceExposedPortDTO
		want    types.ExposedPorts
		wantErr string
	}{
		{
			name:    "empty input",
			svcName: "ldap",
			dtos:    nil,
			want:    types.ExposedPorts{},
		},
		{
			name:    "all valid in order",
			svcName: "ldap",
			dtos: []ServiceExposedPortDTO{
				{Protocol: "TCP", Port: 8080, TargetPort: 80},
				{Protocol: "UDP", Port: 5000, TargetPort: 5000},
			},
			want: types.ExposedPorts{
				{Name: "ldap-expose-8080-80", ServiceName: "ldap", Protocol: corev1.ProtocolTCP, RequestedExternalPort: 8080, ServicePort: 80},
				{Name: "ldap-expose-5000-5000", ServiceName: "ldap", Protocol: corev1.ProtocolUDP, RequestedExternalPort: 5000, ServicePort: 5000},
			},
		},
		{
			name:    "partial failure: invalid entry skipped, valid kept",
			svcName: "ldap",
			dtos: []ServiceExposedPortDTO{
				{Protocol: "TCP", Port: 8080, TargetPort: 80},
				{Protocol: "ICMP", Port: 9000, TargetPort: 9000},
			},
			want: types.ExposedPorts{
				{Name: "ldap-expose-8080-80", ServiceName: "ldap", Protocol: corev1.ProtocolTCP, RequestedExternalPort: 8080, ServicePort: 80},
			},
			wantErr: "unsupported protocol for exposed port: ICMP",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapExposedPortList(tt.svcName, tt.dtos)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_mapExposedPorts(t *testing.T) {
	validAnnotation := `[{"protocol":"TCP","port":8080,"targetPort":80},{"protocol":"UDP","port":5000,"targetPort":5000}]`
	invalidEntryAnnotation := `[{"protocol":"TCP","port":-1,"targetPort":80}]`
	mismatchAnnotation := `[{"protocol":"TCP","port":8080,"targetPort":80}]`

	tests := []struct {
		name    string
		service *corev1.Service
		want    types.ExposedPorts
		wantErr string
	}{
		{
			name:    "nil annotations",
			service: &corev1.Service{},
			want:    types.ExposedPorts{},
		},
		{
			name: "annotation key absent",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"other": "x"}},
			},
			want: types.ExposedPorts{},
		},
		{
			name: "unmarshal failure",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{cesExposedPortsAnnotation: "not-json"}},
			},
			wantErr: "failed to unmarshal exposed ports",
		},
		{
			name: "mapping failure surfaces",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ldap",
					Annotations: map[string]string{cesExposedPortsAnnotation: invalidEntryAnnotation},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 80}}},
			},
			wantErr: "failed to map export ports list from service",
		},
		{
			name: "validation failure surfaces",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ldap",
					Annotations: map[string]string{cesExposedPortsAnnotation: mismatchAnnotation},
				},
				Spec: corev1.ServiceSpec{Ports: nil},
			},
			wantErr: "failed to validate exposed port annotation against service ports",
		},
		{
			name: "success",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ldap",
					Annotations: map[string]string{cesExposedPortsAnnotation: validAnnotation},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
					{Protocol: corev1.ProtocolTCP, Port: 80},
					{Protocol: corev1.ProtocolUDP, Port: 5000},
				}},
			},
			want: types.ExposedPorts{
				{Name: "ldap-expose-8080-80", ServiceName: "ldap", Protocol: corev1.ProtocolTCP, RequestedExternalPort: 8080, ServicePort: 80},
				{Name: "ldap-expose-5000-5000", ServiceName: "ldap", Protocol: corev1.ProtocolUDP, RequestedExternalPort: 5000, ServicePort: 5000},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapExposedPorts(tt.service)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_cesServiceDTO_hasRewriteConfig(t *testing.T) {
	assert.False(t, cesServiceDTO{Rewrite: ""}.hasRewriteConfig())
	assert.True(t, cesServiceDTO{Rewrite: "{}"}.hasRewriteConfig())
}

func Test_cesServiceDTO_getRewriteConfig(t *testing.T) {
	tests := []struct {
		name    string
		dto     cesServiceDTO
		want    *serviceRewriteDTO
		wantErr string
	}{
		{
			name:    "empty rewrite",
			dto:     cesServiceDTO{Rewrite: ""},
			wantErr: "cesService has no rewrite config",
		},
		{
			name:    "invalid JSON",
			dto:     cesServiceDTO{Rewrite: "not-json"},
			wantErr: "failed to read service rewrite",
		},
		{
			name: "valid JSON",
			dto:  cesServiceDTO{Rewrite: `{"pattern":"^/a","rewrite":"/b"}`},
			want: &serviceRewriteDTO{Pattern: "^/a", Rewrite: "/b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.dto.getRewriteConfig()
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_createHttpRoutes(t *testing.T) {
	stripPrefix := "/strip"
	_ = stripPrefix

	tests := []struct {
		name     string
		svcName  string
		services []cesServiceDTO
		want     []types.HttpRoute
		wantErr  string
	}{
		{
			name:     "empty input",
			svcName:  "ldap",
			services: nil,
			want:     []types.HttpRoute{},
		},
		{
			name:    "no rewrite when Pass equals Location",
			svcName: "ldap",
			services: []cesServiceDTO{
				{Name: "ui", Port: 8080, Location: "/ldap", Pass: "/ldap"},
			},
			want: []types.HttpRoute{
				{Name: "ldap-ui-8080", Service: "ldap", Port: 8080, Path: "/ldap", Rewrite: nil},
			},
		},
		{
			name:    "implicit regex rewrite when Pass differs from Location",
			svcName: "ldap",
			services: []cesServiceDTO{
				{Name: "ui", Port: 8080, Location: "/ldap", Pass: "/"},
			},
			want: []types.HttpRoute{
				{
					Name:    "ldap-ui-8080",
					Service: "ldap",
					Port:    8080,
					Path:    "/ldap",
					Rewrite: &types.HttpRewrite{Regex: &types.RegexReplacement{
						Pattern:     "/ldap(/|$)(.*)",
						Replacement: "/$2",
					}},
				},
			},
		},
		{
			name:    "explicit valid rewrite payload",
			svcName: "ldap",
			services: []cesServiceDTO{
				{Name: "ui", Port: 8080, Location: "/ldap", Pass: "/", Rewrite: `{"pattern":"^/foo","rewrite":"/bar"}`},
			},
			want: []types.HttpRoute{
				{
					Name:    "ldap-ui-8080",
					Service: "ldap",
					Port:    8080,
					Path:    "^/foo",
					Rewrite: &types.HttpRewrite{Regex: &types.RegexReplacement{
						Pattern:     "^/foo",
						Replacement: "/bar",
					}},
				},
			},
		},
		{
			name:    "invalid rewrite is skipped but valid entry kept",
			svcName: "ldap",
			services: []cesServiceDTO{
				{Name: "broken", Port: 8081, Location: "/x", Pass: "/x", Rewrite: "not-json"},
				{Name: "ui", Port: 8080, Location: "/ldap", Pass: "/ldap"},
			},
			want: []types.HttpRoute{
				{Name: "ldap-ui-8080", Service: "ldap", Port: 8080, Path: "/ldap", Rewrite: nil},
			},
			wantErr: "failed to get serviceRewrite config for ces service",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createHttpRoutes(tt.svcName, tt.services)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_mapCesServicesToHttpRoutes(t *testing.T) {
	servicesAnnotation := `[{"name":"ui","port":8080,"location":"/ldap","pass":"/ldap"}]`

	tests := []struct {
		name    string
		service *corev1.Service
		want    []types.HttpRoute
		wantErr string
	}{
		{
			name:    "no ports => empty",
			service: &corev1.Service{},
			want:    []types.HttpRoute{},
		},
		{
			name: "missing annotation => empty",
			service: &corev1.Service{
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
			},
			want: []types.HttpRoute{},
		},
		{
			name: "unmarshal failure",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{cesServiceAnnotation: "not-json"}},
				Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
			},
			want:    []types.HttpRoute{},
			wantErr: "failed to unmarshal ces services from dogu service annotation",
		},
		{
			name: "success",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ldap",
					Annotations: map[string]string{cesServiceAnnotation: servicesAnnotation},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 8080}}},
			},
			want: []types.HttpRoute{
				{Name: "ldap-ui-8080", Service: "ldap", Port: 8080, Path: "/ldap", Rewrite: nil},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapCesServicesToHttpRoutes(tt.service)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_mapServiceToExposition(t *testing.T) {
	scheme := testclient.NewClientBuilder().WithScheme(getScheme(t)).Build()

	servicesAnnotation := `[{"name":"ldap-ui","port":8080,"location":"/ldap","pass":"/ldap"}]`
	exposedPortsAnnotation := `[{"protocol":"TCP","port":8080,"targetPort":80},{"protocol":"UDP","port":5000,"targetPort":5000}]`

	tests := []struct {
		name     string
		service  *corev1.Service
		wantName string
		wantHTTP int
		wantTCP  int
		wantUDP  int
		wantErr  string
	}{
		{
			name: "invalid ces-services annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ldap",
					Namespace:   testNamespace,
					Annotations: map[string]string{cesServiceAnnotation: "not-json"},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Port: 80}}},
			},
			wantErr: "failed to map ces service to http routes",
		},
		{
			name: "invalid exposed-ports annotation",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "ldap",
					Namespace:   testNamespace,
					Annotations: map[string]string{cesExposedPortsAnnotation: "not-json"},
				},
			},
			wantErr: "failed to map ces exposed ports",
		},
		{
			name: "empty annotations",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "ldap", Namespace: testNamespace},
			},
			wantName: "ldap",
		},
		{
			name: "full success",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ldap",
					Namespace: testNamespace,
					Annotations: map[string]string{
						cesServiceAnnotation:      servicesAnnotation,
						cesExposedPortsAnnotation: exposedPortsAnnotation,
					},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
					{Protocol: corev1.ProtocolTCP, Port: 80},
					{Protocol: corev1.ProtocolUDP, Port: 5000},
					{Port: 8080},
				}},
			},
			wantName: "ldap",
			wantHTTP: 1,
			wantTCP:  1,
			wantUDP:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapServiceToExposition(tt.service, scheme)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, types.Exposition{}, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantName, got.Name)
			assert.Equal(t, testNamespace, got.Namespace)
			assert.Len(t, got.HttpRoutes, tt.wantHTTP)
			assert.Len(t, got.TcpRoutes, tt.wantTCP)
			assert.Len(t, got.UdpRoutes, tt.wantUDP)
			assert.NotNil(t, got.SetOwner)
		})
	}
}

func Test_mapServiceToExposition_SetOwner(t *testing.T) {
	scheme := testclient.NewClientBuilder().WithScheme(getScheme(t)).Build()
	owner := &corev1.Service{
		TypeMeta:   metav1.TypeMeta{Kind: "Service", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "ldap", Namespace: testNamespace, UID: "owner-uid"},
	}

	exposition, err := mapServiceToExposition(owner, scheme)
	require.NoError(t, err)
	require.NotNil(t, exposition.SetOwner)

	target := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "tgt", Namespace: testNamespace}}
	require.NoError(t, exposition.SetOwner(target))

	require.Len(t, target.OwnerReferences, 1)
	assert.Equal(t, "ldap", target.OwnerReferences[0].Name)
	require.NotNil(t, target.OwnerReferences[0].Controller)
	assert.True(t, *target.OwnerReferences[0].Controller)
}

func Test_isDoguService(t *testing.T) {
	tests := []struct {
		name string
		obj  client.Object
		want bool
	}{
		{
			name: "not a Service",
			obj:  &corev1.ConfigMap{},
			want: false,
		},
		{
			name: "NodePort Service is rejected",
			obj: &corev1.Service{
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeNodePort},
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{doguv2.DoguLabelName: "ldap"}},
			},
			want: false,
		},
		{
			name: "LoadBalancer Service is rejected",
			obj: &corev1.Service{
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{doguv2.DoguLabelName: "ldap"}},
			},
			want: false,
		},
		{
			name: "ClusterIP with nil labels",
			obj: &corev1.Service{
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
			},
			want: false,
		},
		{
			name: "ClusterIP with empty labels",
			obj: &corev1.Service{
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}},
			},
			want: false,
		},
		{
			name: "ClusterIP without dogu label",
			obj: &corev1.Service{
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "ldap"}},
			},
			want: false,
		},
		{
			name: "valid dogu Service",
			obj: &corev1.Service{
				Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{doguv2.DoguLabelName: "ldap"}},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isDoguService(tt.obj))
		})
	}
}

func Test_doguServicePredicate(t *testing.T) {
	validDoguSvc := &corev1.Service{
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		ObjectMeta: metav1.ObjectMeta{Name: "ldap", Labels: map[string]string{doguv2.DoguLabelName: "ldap"}},
	}
	nonDoguObj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cm"}}

	p := doguServicePredicate()

	t.Run("Create accepts dogu service", func(t *testing.T) {
		assert.True(t, p.Create(event.CreateEvent{Object: validDoguSvc}))
	})
	t.Run("Create rejects non-dogu object", func(t *testing.T) {
		assert.False(t, p.Create(event.CreateEvent{Object: nonDoguObj}))
	})
	t.Run("Update accepts dogu service", func(t *testing.T) {
		assert.True(t, p.Update(event.UpdateEvent{ObjectOld: validDoguSvc, ObjectNew: validDoguSvc}))
	})
	t.Run("Update rejects non-dogu object", func(t *testing.T) {
		assert.False(t, p.Update(event.UpdateEvent{ObjectOld: nonDoguObj, ObjectNew: nonDoguObj}))
	})
	t.Run("Delete accepts dogu service", func(t *testing.T) {
		assert.True(t, p.Delete(event.DeleteEvent{Object: validDoguSvc}))
	})
	t.Run("Delete rejects non-dogu object", func(t *testing.T) {
		assert.False(t, p.Delete(event.DeleteEvent{Object: nonDoguObj}))
	})
	t.Run("Generic accepts dogu service", func(t *testing.T) {
		assert.True(t, p.Generic(event.GenericEvent{Object: validDoguSvc}))
	})
	t.Run("Generic rejects non-dogu object", func(t *testing.T) {
		assert.False(t, p.Generic(event.GenericEvent{Object: nonDoguObj}))
	})
}
