package domain

import (
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIngressDefinitionCreator_CreateFromExposition(t *testing.T) {
	type fields struct {
		Maintenance  maintenanceAdapter
		ReadyChecker deploymentReadyChecker
	}
	tests := []struct {
		name       string
		fieldsFn   func(t *testing.T) fields
		exposition *expositionv1.Exposition
		want       IngressDefinition
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "fail to get maintenance status",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get maintenance status", i...)
			},
			want: IngressDefinition{},
			exposition: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"dogu.name": "test"},
			}},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, assert.AnError)
				return fields{
					Maintenance: maintenance,
				}
			},
		},
		{
			name: "fail to check readiness",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to check deployment readiness for \"test\"", i...)
			},
			want: IngressDefinition{},
			exposition: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"dogu.name": "test"},
			}},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, assert.AnError)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name:    "succeed for dogu",
			wantErr: assert.NoError,
			want: IngressDefinition{
				BaseName: "test",
				Type:     "exposition",
				Dogu: &DoguInformation{
					IsMaintenanceMode: true,
					IsStarting:        true,
				},
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "k8s.cloudogu.com/v1",
					Kind:               "Exposition",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				HttpRoutes: []HttpRoute{{
					Name:    "route",
					Service: "svc",
					Port:    8080,
					Path:    "/redmine",
					Rewrite: &HttpRewrite{StripPrefix: new("/redmine")},
				}},
			},
			exposition: &expositionv1.Exposition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					UID:    "123",
					Labels: map[string]string{"dogu.name": "test"},
				},
				Spec: expositionv1.ExpositionSpec{
					HTTP: []expositionv1.HTTPEntry{{
						Name:    "route",
						Service: "svc",
						Port:    8080,
						Path:    "/redmine",
						Rewrite: &expositionv1.Rewrite{StripPrefix: new("/redmine")},
					}},
				},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, true, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, nil)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name:    "succeed",
			wantErr: assert.NoError,
			want: IngressDefinition{
				BaseName: "test",
				Type:     "exposition",
				Dogu:     nil,
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "k8s.cloudogu.com/v1",
					Kind:               "Exposition",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				HttpRoutes: []HttpRoute{{
					Name:    "route",
					Service: "svc",
					Port:    8080,
					Path:    "/usermgt",
					Rewrite: &HttpRewrite{Regex: &RegexReplacement{
						Replacement: "/", Pattern: "/usermgt",
					}},
				}},
			},
			exposition: &expositionv1.Exposition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					UID:  "123",
				},
				Spec: expositionv1.ExpositionSpec{
					HTTP: []expositionv1.HTTPEntry{{
						Name:    "route",
						Service: "svc",
						Port:    8080,
						Path:    "/usermgt",
						Rewrite: &expositionv1.Rewrite{Regex: &expositionv1.RegexRewrite{
							Replacement: "/", Pattern: "/usermgt",
						}},
					}},
				},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				checker := newMockDeploymentReadyChecker(t)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := tt.fieldsFn(t)
			c := &IngressDefinitionCreator{
				Maintenance:  mocks.Maintenance,
				ReadyChecker: mocks.ReadyChecker,
			}
			got, err := c.CreateFromExposition(t.Context(), tt.exposition)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIngressDefinitionCreator_CreateFromService(t *testing.T) {
	type fields struct {
		Maintenance  maintenanceAdapter
		ReadyChecker deploymentReadyChecker
	}
	tests := []struct {
		name     string
		fieldsFn func(t *testing.T) fields
		service  *corev1.Service
		want     IngressDefinition
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name: "fail to get maintenance status",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get maintenance status", i...)
			},
			want: IngressDefinition{},
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"dogu.name": "test"},
			}},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, assert.AnError)
				return fields{
					Maintenance: maintenance,
				}
			},
		},
		{
			name: "fail to check readiness",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to check deployment readiness for \"test\"", i...)
			},
			want: IngressDefinition{},
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:   "test",
				Labels: map[string]string{"dogu.name": "test"},
			}},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, assert.AnError)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name: "fail to unmarshal service annotations",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to get ces services", i...) &&
					assert.ErrorContains(t, err, "failed to unmarshal ces services", i...)
			},
			want: IngressDefinition{},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Labels:      map[string]string{"dogu.name": "test"},
					Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-services": "invalid"},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{
					Name: "redmine",
					Port: 8080,
				}}},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, nil)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name: "fail to unmarshal additional ingress annotations",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to get additional ingress annotations from dogu service 'test'", i...)
			},
			want: IngressDefinition{},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"dogu.name": "test"},
					Annotations: map[string]string{
						"k8s-dogu-operator.cloudogu.com/ces-services":                   "[]",
						"k8s-dogu-operator.cloudogu.com/additional-ingress-annotations": "invalid",
					},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{
					Name: "redmine",
					Port: 8080,
				}}},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, nil)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name: "fail to get rewrite config",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to get serviceRewrite config for ces service \"redmine\"", i...) &&
					assert.ErrorContains(t, err, "failed to get http routes", i...)
			},
			want: IngressDefinition{},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"dogu.name": "test"},
					Annotations: map[string]string{
						"k8s-dogu-operator.cloudogu.com/ces-services": `[{"name": "redmine", "port": 8080, "location": "/redmine", "rewrite": "invalid"}]`,
					},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{
					Name: "redmine",
					Port: 8080,
				}}},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, nil)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name:    "succeed with empty ports",
			wantErr: assert.NoError,
			want: IngressDefinition{
				BaseName: "test",
				Type:     "service",
				Dogu: &DoguInformation{
					IsMaintenanceMode: false,
					IsStarting:        true,
				},
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				HttpRoutes: nil,
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					UID:    "123",
					Labels: map[string]string{"dogu.name": "test"},
					Annotations: map[string]string{
						"k8s-dogu-operator.cloudogu.com/ces-services": `[{"name": "redmine", "port": 8080, "location": "/redmine", "rewrite": "invalid"}]`,
					},
				},
				Spec: corev1.ServiceSpec{Ports: nil},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, nil)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name:    "succeed with no service annotation",
			wantErr: assert.NoError,
			want: IngressDefinition{
				BaseName: "test",
				Type:     "service",
				Dogu: &DoguInformation{
					IsMaintenanceMode: false,
					IsStarting:        true,
				},
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				HttpRoutes: nil,
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					UID:         "123",
					Labels:      map[string]string{"dogu.name": "test"},
					Annotations: nil,
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "redmine", Port: 8080}}},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, nil)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
		{
			name:    "succeed with http routes",
			wantErr: assert.NoError,
			want: IngressDefinition{
				BaseName: "test",
				Type:     "service",
				Dogu: &DoguInformation{
					IsMaintenanceMode: false,
					IsStarting:        true,
				},
				OwnerReference: metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Service",
					Name:               "test",
					UID:                "123",
					Controller:         new(true),
					BlockOwnerDeletion: new(true),
				},
				HttpRoutes: []HttpRoute{
					{
						Name:    "redmine",
						Service: "test",
						Port:    8080,
						Path:    "/redmine",
						Rewrite: &HttpRewrite{Regex: &RegexReplacement{Replacement: "/redmine/api/$2", Pattern: "/redmine(/|$)(.*)"}},
					},
					{
						Name:    "redmine2",
						Service: "test",
						Port:    8081,
						Path:    "/redmine",
						Rewrite: &HttpRewrite{Regex: &RegexReplacement{Replacement: "/", Pattern: "/redmine"}},
					},
				},
			},
			service: &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					UID:    "123",
					Labels: map[string]string{"dogu.name": "test"},
					Annotations: map[string]string{
						"k8s-dogu-operator.cloudogu.com/ces-services": `[{"name": "redmine", "port": 8080, "location": "/redmine", "pass": "/redmine/api"},{"name": "redmine2", "port": 8081, "location": "/redmine", "pass": "/redmine", "rewrite": "{\"pattern\": \"/redmine\", \"rewrite\": \"/\"}"}]`,
					},
				},
				Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
					{Name: "redmine", Port: 8080},
					{Name: "redmine2", Port: 8081},
				}},
			},
			fieldsFn: func(t *testing.T) fields {
				maintenance := newMockMaintenanceAdapter(t)
				maintenance.EXPECT().GetStatus(t.Context()).
					Return(repository.MaintenanceModeDescription{}, false, nil)
				checker := newMockDeploymentReadyChecker(t)
				checker.EXPECT().IsReady(t.Context(), "test").
					Return(false, nil)
				return fields{
					Maintenance:  maintenance,
					ReadyChecker: checker,
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mocks := tt.fieldsFn(t)
			c := &IngressDefinitionCreator{
				Maintenance:  mocks.Maintenance,
				ReadyChecker: mocks.ReadyChecker,
			}
			got, err := c.CreateFromService(t.Context(), tt.service)
			tt.wantErr(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
