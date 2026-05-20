package domain

import (
	"reflect"
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateExposedPortsDefinitionFromService(t *testing.T) {
	tests := []struct {
		name    string
		service *corev1.Service
		want    ExposedPortsDefinition
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "no routes if annotation not present",
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test", UID: "123"}},
			want: ExposedPortsDefinition{
				BaseName: "test",
				Type:     "service",
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				TcpRoutes: nil,
				UdpRoutes: nil,
			},
			wantErr: assert.NoError,
		},
		{
			name: "fail to parse ports",
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "invalid"},
			}},
			want: ExposedPortsDefinition{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to unmarshal ces exposed ports annotation \"k8s-dogu-operator.cloudogu.com/ces-exposed-ports\" from service \"test\"", i...)
			},
		},
		{
			name: "port not in service spec",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					UID:         "123",
					Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": `[{"port": 2222, "targetPort": 22}]`},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
					{
						Protocol: "invalid",
						Port:     2222,
					},
					{
						Protocol: "tcp",
						Port:     22,
					},
				}},
			},
			want: ExposedPortsDefinition{
				BaseName: "test",
				Type:     "service",
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				TcpRoutes: nil,
				UdpRoutes: nil,
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "invalid service annotation \"k8s-dogu-operator.cloudogu.com/ces-exposed-ports\". port '2222' is not defined in service ports", i...)
			},
		},
		{
			name: "invalid protocol",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "123",
					Annotations: map[string]string{
						"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": `[{"protocol": "invalid", "port": 2222, "targetPort": 22}]`,
					},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{
					Protocol: "invalid",
					Port:     2222,
				}}},
			},
			want: ExposedPortsDefinition{
				BaseName: "test",
				Type:     "service",
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				TcpRoutes: nil,
				UdpRoutes: nil,
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "invalid protocol \"invalid\" for exposed port 2222", i...)
			},
		},
		{
			name: "success",
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "123",
					Annotations: map[string]string{
						"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": `[{"protocol": "tcp", "port": 2222, "targetPort": 22}, {"protocol": "udp", "port": 3333, "targetPort": 33}]`,
					},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
					{
						Protocol: "tcp",
						Port:     2222,
					},
					{
						Protocol: "udp",
						Port:     3333,
					},
				}},
			},
			want: ExposedPortsDefinition{
				BaseName: "test",
				Type:     "service",
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				TcpRoutes: []ExposedPortRoute{{
					Name:                  "port-2222-22",
					Service:               "test",
					Port:                  2222,
					RequestedExternalPort: new(int32(22)),
					Protocol:              nil,
				}},
				UdpRoutes: []ExposedPortRoute{{
					Name:                  "port-3333-33",
					Service:               "test",
					Port:                  3333,
					RequestedExternalPort: new(int32(33)),
					Protocol:              nil,
				}},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CreateExposedPortsDefinitionFromService(tt.service)
			tt.wantErr(t, err)
			if err != nil {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CreateExposedPortsDefinitionFromService() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateExposedPortsDefinitionFromExposition(t *testing.T) {
	exposition := &expositionv1.Exposition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			UID:  "123",
		},
		Spec: expositionv1.ExpositionSpec{
			TCP: []expositionv1.TCPEntry{{
				Name:                  "port-2222-22",
				Service:               "test",
				Port:                  2222,
				RequestedExternalPort: new(int32(22)),
				Protocol:              new("ssh"),
			}},
			UDP: []expositionv1.UDPEntry{{
				Name:                  "port-3333-33",
				Service:               "test",
				Port:                  3333,
				RequestedExternalPort: nil,
				Protocol:              nil,
			}},
		},
	}
	want := ExposedPortsDefinition{
		BaseName: "test",
		Type:     "exposition",
		OwnerReference: metav1.OwnerReference{
			APIVersion:         "k8s.cloudogu.com/v1",
			Kind:               "Exposition",
			Name:               "test",
			UID:                "123",
			Controller:         new(true),
			BlockOwnerDeletion: new(true),
		},
		TcpRoutes: []ExposedPortRoute{{
			Name:                  "port-2222-22",
			Service:               "test",
			Port:                  2222,
			RequestedExternalPort: new(int32(22)),
			Protocol:              new("ssh"),
		}},
		UdpRoutes: []ExposedPortRoute{{
			Name:                  "port-3333-33",
			Service:               "test",
			Port:                  3333,
			RequestedExternalPort: nil,
			Protocol:              nil,
		}},
	}
	assert.Equal(t, want, CreateExposedPortsDefinitionFromExposition(exposition))
}
