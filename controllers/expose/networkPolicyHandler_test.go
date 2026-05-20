package expose

import (
	"context"
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/domain"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNetworkPolicyHandler_UpsertNetworkPoliciesForService(t *testing.T) {
	type fields struct {
		generatorFn func(t *testing.T) networkPolicyGenerator
		clientFn    func(t *testing.T) client.Client
		namespace   string
	}
	tests := []struct {
		name    string
		fields  fields
		service *corev1.Service
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "should fail to create definition",
			fields: fields{
				generatorFn: func(t *testing.T) networkPolicyGenerator {
					return newMockNetworkPolicyGenerator(t)
				},
				clientFn: func(t *testing.T) client.Client {
					return newMockK8sClient(t)
				},
				namespace: testNamespace,
			},
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:        "test",
				Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "invalid"},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to create exposed ports definition from service \"test\"", i...)
			},
		},
		{
			name: "should fail to upsert",
			fields: fields{
				generatorFn: func(t *testing.T) networkPolicyGenerator {
					m := newMockNetworkPolicyGenerator(t)
					m.EXPECT().
						Generate(domain.ExposedPortsDefinition{
							BaseName: "test-service",
							Type:     "service",
							OwnerReference: metav1.OwnerReference{
								APIVersion:         "v1",
								Kind:               "Service",
								Name:               "test-service",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
							TcpRoutes: nil,
							UdpRoutes: nil,
						}).
						Return([]*netv1.NetworkPolicy{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-netpol",
									Namespace: testNamespace,
									OwnerReferences: []metav1.OwnerReference{{
										APIVersion:         "v1",
										Kind:               "Service",
										Name:               "test-service",
										UID:                "123",
										Controller:         new(true),
										BlockOwnerDeletion: new(true),
									}},
								},
							},
						})
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						List(t.Context(), &netv1.NetworkPolicyList{},
							&client.ListOptions{Namespace: testNamespace, LabelSelector: selectorFromBaseName("test-service")}).
						Return(assert.AnError)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "test-netpol", Namespace: testNamespace}, &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol", Namespace: testNamespace}}).
						Return(assert.AnError)
					return m
				},
				namespace: testNamespace,
			},
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:        "test-service",
				Namespace:   testNamespace,
				UID:         "123",
				Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "[]"},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to upsert network policies for service \"test-service\"", i...) &&
					assert.ErrorContains(t, err, "failed to list existing ingresses", i...) &&
					assert.ErrorContains(t, err, "failed to create or update network policy \"test-netpol\"", i...)

			},
		},
		{
			name: "should fail to delete outdated objects",
			fields: fields{
				generatorFn: func(t *testing.T) networkPolicyGenerator {
					m := newMockNetworkPolicyGenerator(t)
					m.EXPECT().
						Generate(domain.ExposedPortsDefinition{
							BaseName: "test-service",
							Type:     "service",
							OwnerReference: metav1.OwnerReference{
								APIVersion:         "v1",
								Kind:               "Service",
								Name:               "test-service",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
							TcpRoutes: nil,
							UdpRoutes: nil,
						}).
						Return([]*netv1.NetworkPolicy{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-netpol",
									Namespace: testNamespace,
									OwnerReferences: []metav1.OwnerReference{{
										APIVersion:         "v1",
										Kind:               "Service",
										Name:               "test-service",
										UID:                "123",
										Controller:         new(true),
										BlockOwnerDeletion: new(true),
									}},
								},
							},
						})
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						List(t.Context(), &netv1.NetworkPolicyList{},
							&client.ListOptions{Namespace: testNamespace, LabelSelector: selectorFromBaseName("test-service")}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*netv1.NetworkPolicyList).Items = []netv1.NetworkPolicy{
								{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "test-netpol", Namespace: testNamespace}, &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
							Name:      "test-netpol",
							Namespace: testNamespace,
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion:         "v1",
								Kind:               "Service",
								Name:               "test-service",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							}},
						}}).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(assert.AnError)
					return m
				},
				namespace: testNamespace,
			},
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:        "test-service",
				Namespace:   testNamespace,
				UID:         "123",
				Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "[]"},
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to upsert network policies for service \"test-service\"", i...) &&
					assert.ErrorContains(t, err, "failed to delete outdated network policy \"outdated\"", i...)

			},
		},
		{
			name: "success",
			fields: fields{
				generatorFn: func(t *testing.T) networkPolicyGenerator {
					m := newMockNetworkPolicyGenerator(t)
					m.EXPECT().
						Generate(domain.ExposedPortsDefinition{
							BaseName: "test-service",
							Type:     "service",
							OwnerReference: metav1.OwnerReference{
								APIVersion:         "v1",
								Kind:               "Service",
								Name:               "test-service",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
							TcpRoutes: nil,
							UdpRoutes: nil,
						}).
						Return([]*netv1.NetworkPolicy{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-netpol",
									Namespace: testNamespace,
									OwnerReferences: []metav1.OwnerReference{{
										APIVersion:         "v1",
										Kind:               "Service",
										Name:               "test-service",
										UID:                "123",
										Controller:         new(true),
										BlockOwnerDeletion: new(true),
									}},
								},
							},
						})
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						List(t.Context(), &netv1.NetworkPolicyList{},
							&client.ListOptions{Namespace: testNamespace, LabelSelector: selectorFromBaseName("test-service")}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*netv1.NetworkPolicyList).Items = []netv1.NetworkPolicy{
								{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "test-netpol", Namespace: testNamespace}, &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
							Name:      "test-netpol",
							Namespace: testNamespace,
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion:         "v1",
								Kind:               "Service",
								Name:               "test-service",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							}},
						}}).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(nil)
					return m
				},
				namespace: testNamespace,
			},
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:        "test-service",
				Namespace:   testNamespace,
				UID:         "123",
				Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "[]"},
			}},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nph := &NetworkPolicyHandler{
				generator: tt.fields.generatorFn(t),
				client:    tt.fields.clientFn(t),
				namespace: tt.fields.namespace,
			}
			tt.wantErr(t, nph.UpsertNetworkPoliciesForService(t.Context(), tt.service))
		})
	}
}

func TestNetworkPolicyHandler_UpsertNetworkPoliciesForExposition(t *testing.T) {
	type fields struct {
		generatorFn func(t *testing.T) networkPolicyGenerator
		clientFn    func(t *testing.T) client.Client
		namespace   string
	}
	tests := []struct {
		name       string
		fields     fields
		exposition *expositionv1.Exposition
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "should fail to upsert",
			fields: fields{
				generatorFn: func(t *testing.T) networkPolicyGenerator {
					m := newMockNetworkPolicyGenerator(t)
					m.EXPECT().
						Generate(domain.ExposedPortsDefinition{
							BaseName: "test-exposition",
							Type:     "exposition",
							OwnerReference: metav1.OwnerReference{
								APIVersion:         "k8s.cloudogu.com/v1",
								Kind:               "Exposition",
								Name:               "test-exposition",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
							TcpRoutes: nil,
							UdpRoutes: nil,
						}).
						Return([]*netv1.NetworkPolicy{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-netpol",
									Namespace: testNamespace,
									OwnerReferences: []metav1.OwnerReference{{
										APIVersion:         "k8s.cloudogu.com/v1",
										Kind:               "Exposition",
										Name:               "test-exposition",
										UID:                "123",
										Controller:         new(true),
										BlockOwnerDeletion: new(true),
									}},
								},
							},
						})
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						List(t.Context(), &netv1.NetworkPolicyList{},
							&client.ListOptions{Namespace: testNamespace, LabelSelector: selectorFromBaseName("test-exposition")}).
						Return(assert.AnError)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "test-netpol", Namespace: testNamespace}, &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol", Namespace: testNamespace}}).
						Return(assert.AnError)
					return m
				},
				namespace: testNamespace,
			},
			exposition: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{
				Name:      "test-exposition",
				Namespace: testNamespace,
				UID:       "123",
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to upsert network policies for exposition \"test-exposition\"", i...) &&
					assert.ErrorContains(t, err, "failed to list existing ingresses", i...) &&
					assert.ErrorContains(t, err, "failed to create or update network policy \"test-netpol\"", i...)

			},
		},
		{
			name: "should fail to delete outdated objects",
			fields: fields{
				generatorFn: func(t *testing.T) networkPolicyGenerator {
					m := newMockNetworkPolicyGenerator(t)
					m.EXPECT().
						Generate(domain.ExposedPortsDefinition{
							BaseName: "test-exposition",
							Type:     "exposition",
							OwnerReference: metav1.OwnerReference{
								APIVersion:         "k8s.cloudogu.com/v1",
								Kind:               "Exposition",
								Name:               "test-exposition",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
							TcpRoutes: nil,
							UdpRoutes: nil,
						}).
						Return([]*netv1.NetworkPolicy{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-netpol",
									Namespace: testNamespace,
									OwnerReferences: []metav1.OwnerReference{{
										APIVersion:         "k8s.cloudogu.com/v1",
										Kind:               "Exposition",
										Name:               "test-exposition",
										UID:                "123",
										Controller:         new(true),
										BlockOwnerDeletion: new(true),
									}},
								},
							},
						})
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						List(t.Context(), &netv1.NetworkPolicyList{},
							&client.ListOptions{Namespace: testNamespace, LabelSelector: selectorFromBaseName("test-exposition")}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*netv1.NetworkPolicyList).Items = []netv1.NetworkPolicy{
								{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "test-netpol", Namespace: testNamespace}, &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
							Name:      "test-netpol",
							Namespace: testNamespace,
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion:         "k8s.cloudogu.com/v1",
								Kind:               "Exposition",
								Name:               "test-exposition",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							}},
						}}).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(assert.AnError)
					return m
				},
				namespace: testNamespace,
			},
			exposition: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{
				Name:      "test-exposition",
				Namespace: testNamespace,
				UID:       "123",
			}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to upsert network policies for exposition \"test-exposition\"", i...) &&
					assert.ErrorContains(t, err, "failed to delete outdated network policy \"outdated\"", i...)

			},
		},
		{
			name: "success",
			fields: fields{
				generatorFn: func(t *testing.T) networkPolicyGenerator {
					m := newMockNetworkPolicyGenerator(t)
					m.EXPECT().
						Generate(domain.ExposedPortsDefinition{
							BaseName: "test-exposition",
							Type:     "exposition",
							OwnerReference: metav1.OwnerReference{
								APIVersion:         "k8s.cloudogu.com/v1",
								Kind:               "Exposition",
								Name:               "test-exposition",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							},
							TcpRoutes: nil,
							UdpRoutes: nil,
						}).
						Return([]*netv1.NetworkPolicy{
							{
								ObjectMeta: metav1.ObjectMeta{
									Name:      "test-netpol",
									Namespace: testNamespace,
									OwnerReferences: []metav1.OwnerReference{{
										APIVersion:         "k8s.cloudogu.com/v1",
										Kind:               "Exposition",
										Name:               "test-exposition",
										UID:                "123",
										Controller:         new(true),
										BlockOwnerDeletion: new(true),
									}},
								},
							},
						})
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						List(t.Context(), &netv1.NetworkPolicyList{},
							&client.ListOptions{Namespace: testNamespace, LabelSelector: selectorFromBaseName("test-exposition")}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*netv1.NetworkPolicyList).Items = []netv1.NetworkPolicy{
								{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "test-netpol", Namespace: testNamespace}, &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test-netpol", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{
							Name:      "test-netpol",
							Namespace: testNamespace,
							OwnerReferences: []metav1.OwnerReference{{
								APIVersion:         "k8s.cloudogu.com/v1",
								Kind:               "Exposition",
								Name:               "test-exposition",
								UID:                "123",
								Controller:         new(true),
								BlockOwnerDeletion: new(true),
							}},
						}}).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(nil)
					return m
				},
				namespace: testNamespace,
			},
			exposition: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{
				Name:      "test-exposition",
				Namespace: testNamespace,
				UID:       "123",
			}},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nph := &NetworkPolicyHandler{
				generator: tt.fields.generatorFn(t),
				client:    tt.fields.clientFn(t),
				namespace: tt.fields.namespace,
			}
			tt.wantErr(t, nph.UpsertNetworkPoliciesForExposition(t.Context(), tt.exposition))
		})
	}
}
