package controllers

import (
	"context"
	"testing"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func getScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := new(runtime.NewSchemeBuilder(
		corev1.AddToScheme,
		appsv1.AddToScheme,
		networkingv1.AddToScheme,
		traefikapi.AddToScheme,
		expositionv1.AddToScheme,
	)).AddToScheme(scheme)
	require.NoError(t, err)
	return scheme
}

func TestServiceReconciler_Reconcile(t *testing.T) {
	expectedKey := client.ObjectKey{Namespace: testNamespace, Name: "test"}

	type fields struct {
		ClientFn            func(t *testing.T) client.Client
		ExpositionServiceFn func(t *testing.T) ExpositionService
	}
	tests := []struct {
		name    string
		fields  fields
		req     controllerruntime.Request
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "fail to get dogu service",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &corev1.Service{}).
						Return(assert.AnError)
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					return NewMockExpositionService(t)
				},
			},
			req: controllerruntime.Request{NamespacedName: expectedKey},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get dogu service", i...)
			},
		},
		{
			name: "dogu service not found",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &corev1.Service{}).
						Return(&errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					return NewMockExpositionService(t)
				},
			},
			req:     controllerruntime.Request{NamespacedName: expectedKey},
			wantErr: assert.NoError,
		},
		{
			name: "fail to map dogu service to exposition",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &corev1.Service{}).
						Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							svc := obj.(*corev1.Service)
							svc.Name = "test"
							svc.Namespace = testNamespace
							svc.Annotations = map[string]string{cesExposedPortsAnnotation: "not-json"}
						}).
						Return(nil)
					m.EXPECT().Scheme().Return(getScheme(t)).Maybe()
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					return NewMockExpositionService(t)
				},
			},
			req: controllerruntime.Request{NamespacedName: expectedKey},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to map dogu service to exposition", i...)
			},
		},
		{
			name: "fail to process exposition",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &corev1.Service{}).
						Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							svc := obj.(*corev1.Service)
							svc.Name = "test"
							svc.Namespace = testNamespace
						}).
						Return(nil)
					m.EXPECT().Scheme().Return(getScheme(t)).Maybe()
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					m := NewMockExpositionService(t)
					m.EXPECT().
						ProcessExposition(t.Context(), mock.MatchedBy(func(e types.Exposition) bool {
							return e.Name == "test" && e.Namespace == testNamespace && e.SetOwner != nil
						})).
						Return(assert.AnError)
					return m
				},
			},
			req: controllerruntime.Request{NamespacedName: expectedKey},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to process exposition from dogu service", i...)
			},
		},
		{
			name: "success",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &corev1.Service{}).
						Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							svc := obj.(*corev1.Service)
							svc.Name = "test"
							svc.Namespace = testNamespace
						}).
						Return(nil)
					m.EXPECT().Scheme().Return(getScheme(t)).Maybe()
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					m := NewMockExpositionService(t)
					m.EXPECT().
						ProcessExposition(t.Context(), mock.MatchedBy(func(e types.Exposition) bool {
							return e.Name == "test" && e.Namespace == testNamespace && e.SetOwner != nil
						})).
						Return(nil)
					return m
				},
			},
			req:     controllerruntime.Request{NamespacedName: expectedKey},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ServiceReconciler{
				Client:            tt.fields.ClientFn(t),
				ExpositionService: tt.fields.ExpositionServiceFn(t),
			}
			got, err := r.Reconcile(t.Context(), tt.req)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, reconcile.Result{}, got)
		})
	}
}

func TestServiceReconciler_mapRequestsFromDoguCR(t *testing.T) {
	tests := []struct {
		name string
		obj  client.Object
		want []reconcile.Request
	}{
		{
			name: "object is not a Dogu",
			obj:  &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "ldap", Namespace: testNamespace}},
			want: nil,
		},
		{
			name: "valid Dogu enqueues its Service",
			obj:  &doguv2.Dogu{ObjectMeta: metav1.ObjectMeta{Name: "ldap", Namespace: testNamespace}},
			want: []reconcile.Request{{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "ldap"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mapRequestsFromDoguCR(t.Context(), tt.obj))
		})
	}
}

func doguWithHealthy(status metav1.ConditionStatus) *doguv2.Dogu {
	return &doguv2.Dogu{Status: doguv2.DoguStatus{Conditions: []metav1.Condition{
		{Type: doguv2.ConditionHealthy, Status: status},
	}}}
}

func doguWithoutConditions() *doguv2.Dogu {
	return &doguv2.Dogu{}
}

func TestDoguHealthyConditionChangedPredicate_Update(t *testing.T) {
	tests := []struct {
		name   string
		oldObj client.Object
		newObj client.Object
		want   bool
	}{
		{
			name:   "old object is not a Dogu",
			oldObj: &corev1.Service{},
			newObj: doguWithHealthy(metav1.ConditionTrue),
			want:   false,
		},
		{
			name:   "new object is not a Dogu",
			oldObj: doguWithHealthy(metav1.ConditionTrue),
			newObj: &corev1.Service{},
			want:   false,
		},
		{
			name:   "both have no healthy condition",
			oldObj: doguWithoutConditions(),
			newObj: doguWithoutConditions(),
			want:   false,
		},
		{
			name:   "healthy condition added",
			oldObj: doguWithoutConditions(),
			newObj: doguWithHealthy(metav1.ConditionTrue),
			want:   true,
		},
		{
			name:   "healthy condition removed",
			oldObj: doguWithHealthy(metav1.ConditionTrue),
			newObj: doguWithoutConditions(),
			want:   true,
		},
		{
			name:   "status unchanged True",
			oldObj: doguWithHealthy(metav1.ConditionTrue),
			newObj: doguWithHealthy(metav1.ConditionTrue),
			want:   false,
		},
		{
			name:   "status changed True to False",
			oldObj: doguWithHealthy(metav1.ConditionTrue),
			newObj: doguWithHealthy(metav1.ConditionFalse),
			want:   true,
		},
		{
			name:   "status changed False to Unknown",
			oldObj: doguWithHealthy(metav1.ConditionFalse),
			newObj: doguWithHealthy(metav1.ConditionUnknown),
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := doguHealthyConditionChangedPredicate()
			got := p.Update(event.UpdateEvent{ObjectOld: tt.oldObj, ObjectNew: tt.newObj})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDoguHealthyConditionChangedPredicate_Create(t *testing.T) {
	p := doguHealthyConditionChangedPredicate()
	assert.False(t, p.Create(event.CreateEvent{Object: doguWithHealthy(metav1.ConditionTrue)}))
}

func TestDoguHealthyConditionChangedPredicate_Delete(t *testing.T) {
	p := doguHealthyConditionChangedPredicate()
	assert.False(t, p.Delete(event.DeleteEvent{Object: doguWithHealthy(metav1.ConditionTrue)}))
}
