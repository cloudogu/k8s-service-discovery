package expose

import (
	"context"
	"fmt"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testNamespace        = "my-namespace"
	testIngressClassName = "my-ingress-class-name"
)

func TestNewIngressUpdater(t *testing.T) {
	k8sClient := fake.NewClientBuilder().Build()
	actual := NewIngressUpdater(IngressUpdaterDependencies{
		Namespace:          testNamespace,
		IngressClassName:   testIngressClassName,
		MaintenanceAdapter: newMockMaintenanceAdapter(t),
		ReadyChecker:       NewMockDeploymentReadyChecker(t),
		Client:             k8sClient,
	})

	assert.Same(t, k8sClient, actual.client)
	assert.Equal(t, testNamespace, actual.namespace)
	assert.NotNil(t, actual.generator)
	assert.NotNil(t, actual.ingressDefinitionCreator)
}

func TestIngressUpdater_UpsertForService(t *testing.T) {
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
		Name:      "redmine",
		Namespace: testNamespace,
		UID:       "123",
	}}
	ownerReference := metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Service",
		Name:               "redmine",
		UID:                "123",
		Controller:         new(true),
		BlockOwnerDeletion: new(true),
	}
	ingressDefinition := domain.IngressDefinition{
		BaseName: "redmine",
		Type:     domain.TypeService,
		Dogu: &domain.DoguInformation{
			IsMaintenanceMode: false,
			IsStarting:        false,
		},
		OwnerReference: ownerReference,
		HttpRoutes: []domain.HttpRoute{
			{
				Name:    "redmine",
				Service: "redmine",
				Port:    8080,
				Path:    "/redmine",
				Rewrite: &domain.HttpRewrite{
					Regex: &domain.RegexReplacement{
						Pattern:     "/redmine(/|$)(.*)",
						Replacement: "/$2",
					},
				},
				AdditionalAnnotations: nil,
			},
		},
	}
	middlewareName := "redmine-redmine-rewrite"
	commonLabels := map[string]string{
		"app":                          "ces",
		"app.kubernetes.io/managed-by": "k8s-service-discovery",
		"k8s-service-discovery.cloudogu.com/owned-by": "redmine",
	}
	middlewares := []*traefikv1alpha1.Middleware{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:            middlewareName,
				Namespace:       testNamespace,
				OwnerReferences: []metav1.OwnerReference{ownerReference},
				Labels:          commonLabels,
			},
			Spec: traefikv1alpha1.MiddlewareSpec{
				ReplacePathRegex: &dynamic.ReplacePathRegex{
					Regex:       "/redmine(/|$)(.*)",
					Replacement: "/$2",
				},
			},
		},
	}
	ingresses := []*networkingv1.Ingress{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redmine-redmine",
				Namespace: testNamespace,
				Annotations: map[string]string{
					traefikMiddlewareAnnotationKey: fmt.Sprintf("%s-%s@kubernetescrd", testNamespace, middlewareName),
				},
				OwnerReferences: []metav1.OwnerReference{ownerReference},
				Labels:          commonLabels,
			},
			Spec: networkingv1.IngressSpec{
				IngressClassName: new(testIngressClassName),
				Rules: []networkingv1.IngressRule{
					{
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{{
									Path:     "/redmine",
									PathType: new(networkingv1.PathTypePrefix),
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "redmine",
											Port: networkingv1.ServiceBackendPort{
												Number: 8080,
											},
										},
									},
								}},
							},
						},
					},
				},
			},
		},
	}

	type fields struct {
		ingressDefinitionCreatorFn func(t *testing.T) ingressDefinitionCreator
		generatorFn                func(t *testing.T) ingressGenerator
		clientFn                   func(t *testing.T) client.Client
		namespace                  string
	}
	tests := []struct {
		name    string
		fields  fields
		service *corev1.Service
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "should fail to create ingress definition",
			fields: fields{
				ingressDefinitionCreatorFn: func(t *testing.T) ingressDefinitionCreator {
					m := newMockIngressDefinitionCreator(t)
					m.EXPECT().
						CreateFromService(t.Context(), service).
						Return(domain.IngressDefinition{}, assert.AnError)
					return m
				},
				generatorFn: func(t *testing.T) ingressGenerator {
					return newMockIngressGenerator(t)
				},
				clientFn: func(t *testing.T) client.Client {
					return fake.NewClientBuilder().Build()
				},
				namespace: testNamespace,
			},
			service: service,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to convert service to exposition definition", i...)
			},
		},
		{
			name: "should fail to upsert",
			fields: fields{
				ingressDefinitionCreatorFn: func(t *testing.T) ingressDefinitionCreator {
					m := newMockIngressDefinitionCreator(t)
					m.EXPECT().
						CreateFromService(t.Context(), service).
						Return(ingressDefinition, nil)
					return m
				},
				generatorFn: func(t *testing.T) ingressGenerator {
					m := newMockIngressGenerator(t)
					m.EXPECT().
						GenerateWithMiddlewares(ingressDefinition).
						Return(ingresses, middlewares)
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					selector, err := labels.Parse("k8s-service-discovery.cloudogu.com/owned-by=redmine")
					require.NoError(t, err)
					m.EXPECT().
						List(t.Context(), &networkingv1.IngressList{}, &client.ListOptions{Namespace: testNamespace, LabelSelector: selector}).
						Return(assert.AnError)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "redmine-redmine", Namespace: testNamespace}, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine", Namespace: testNamespace}}).
						Return(assert.AnError)
					m.EXPECT().
						List(t.Context(), &traefikv1alpha1.MiddlewareList{}, &client.ListOptions{Namespace: testNamespace, LabelSelector: selector}).
						Return(assert.AnError)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "redmine-redmine-rewrite", Namespace: testNamespace}, &traefikv1alpha1.Middleware{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine-rewrite", Namespace: testNamespace}}).
						Return(assert.AnError)
					return m
				},
				namespace: testNamespace,
			},
			service: service,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to upsert ingresses or middlewares for service \"redmine\"", i...) &&
					assert.ErrorContains(t, err, "failed to list existing ingresses", i...) &&
					assert.ErrorContains(t, err, "failed to list existing middlewares", i...) &&
					assert.ErrorContains(t, err, "failed to create or update ingress \"redmine-redmine\"", i...) &&
					assert.ErrorContains(t, err, "failed to create or update middleware \"redmine-redmine-rewrite\"", i...)
			},
		},
		{
			name: "should fail to delete outdated objects",
			fields: fields{
				ingressDefinitionCreatorFn: func(t *testing.T) ingressDefinitionCreator {
					m := newMockIngressDefinitionCreator(t)
					m.EXPECT().
						CreateFromService(t.Context(), service).
						Return(ingressDefinition, nil)
					return m
				},
				generatorFn: func(t *testing.T) ingressGenerator {
					m := newMockIngressGenerator(t)
					m.EXPECT().
						GenerateWithMiddlewares(ingressDefinition).
						Return(ingresses, middlewares)
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					selector, err := labels.Parse("k8s-service-discovery.cloudogu.com/owned-by=redmine")
					require.NoError(t, err)
					m.EXPECT().
						List(t.Context(), &networkingv1.IngressList{}, &client.ListOptions{Namespace: testNamespace, LabelSelector: selector}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*networkingv1.IngressList).Items = []networkingv1.Ingress{
								{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "redmine-redmine", Namespace: testNamespace}, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), ingresses[0]).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(assert.AnError)
					m.EXPECT().
						List(t.Context(), &traefikv1alpha1.MiddlewareList{}, &client.ListOptions{Namespace: testNamespace, LabelSelector: selector}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*traefikv1alpha1.MiddlewareList).Items = []traefikv1alpha1.Middleware{
								{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine-rewrite"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "redmine-redmine-rewrite", Namespace: testNamespace}, &traefikv1alpha1.Middleware{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine-rewrite", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), middlewares[0]).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &traefikv1alpha1.Middleware{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(assert.AnError)
					return m
				},
				namespace: testNamespace,
			},
			service: service,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to delete outdated ingress \"outdated\"", i...) &&
					assert.ErrorContains(t, err, "failed to delete outdated middleware \"outdated\"", i...)
			},
		},
		{
			name: "should succeed",
			fields: fields{
				ingressDefinitionCreatorFn: func(t *testing.T) ingressDefinitionCreator {
					m := newMockIngressDefinitionCreator(t)
					m.EXPECT().
						CreateFromService(t.Context(), service).
						Return(ingressDefinition, nil)
					return m
				},
				generatorFn: func(t *testing.T) ingressGenerator {
					m := newMockIngressGenerator(t)
					m.EXPECT().
						GenerateWithMiddlewares(ingressDefinition).
						Return(ingresses, middlewares)
					return m
				},
				clientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					selector, err := labels.Parse("k8s-service-discovery.cloudogu.com/owned-by=redmine")
					require.NoError(t, err)
					m.EXPECT().
						List(t.Context(), &networkingv1.IngressList{}, &client.ListOptions{Namespace: testNamespace, LabelSelector: selector}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*networkingv1.IngressList).Items = []networkingv1.Ingress{
								{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "redmine-redmine", Namespace: testNamespace}, &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), ingresses[0]).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(nil)
					m.EXPECT().
						List(t.Context(), &traefikv1alpha1.MiddlewareList{}, &client.ListOptions{Namespace: testNamespace, LabelSelector: selector}).
						Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
							list.(*traefikv1alpha1.MiddlewareList).Items = []traefikv1alpha1.Middleware{
								{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine-rewrite"}},
								{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}},
							}
						}).
						Return(nil)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Name: "redmine-redmine-rewrite", Namespace: testNamespace}, &traefikv1alpha1.Middleware{ObjectMeta: metav1.ObjectMeta{Name: "redmine-redmine-rewrite", Namespace: testNamespace}}).
						Return(nil)
					m.EXPECT().
						Update(t.Context(), middlewares[0]).
						Return(nil)
					m.EXPECT().
						Delete(t.Context(), &traefikv1alpha1.Middleware{ObjectMeta: metav1.ObjectMeta{Name: "outdated"}}).
						Return(nil)
					return m
				},
				namespace: testNamespace,
			},
			service: service,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := &IngressUpdater{
				ingressDefinitionCreator: tt.fields.ingressDefinitionCreatorFn(t),
				generator:                tt.fields.generatorFn(t),
				client:                   tt.fields.clientFn(t),
				namespace:                tt.fields.namespace,
			}
			tt.wantErr(t, i.UpsertForService(t.Context(), tt.service))
		})
	}
}
