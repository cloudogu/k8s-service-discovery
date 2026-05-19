package controllers

import (
	"context"
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	type fields struct {
		IngressUpdaterFn       func(t *testing.T) IngressUpdater
		NetworkPolicyUpdaterFn func(t *testing.T) NetworkPolicyUpdater
		ClientFn               func(t *testing.T) client.Client
	}
	tests := []struct {
		name    string
		fields  fields
		req     controllerruntime.Request
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "fail to get service",
			fields: fields{
				IngressUpdaterFn: func(t *testing.T) IngressUpdater {
					m := NewMockIngressUpdater(t)
					return m
				},
				NetworkPolicyUpdaterFn: func(t *testing.T) NetworkPolicyUpdater {
					m := NewMockNetworkPolicyUpdater(t)
					return m
				},
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &corev1.Service{}).
						Return(assert.AnError)
					return m
				},
			},
			req: controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get service", i...)
			},
		},
		{
			name: "service not found",
			fields: fields{
				IngressUpdaterFn: func(t *testing.T) IngressUpdater {
					m := NewMockIngressUpdater(t)
					return m
				},
				NetworkPolicyUpdaterFn: func(t *testing.T) NetworkPolicyUpdater {
					m := NewMockNetworkPolicyUpdater(t)
					return m
				},
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &corev1.Service{}).
						Return(&errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})
					return m
				},
			},
			req:     controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: assert.NoError,
		},
		{
			name: "fail to upsert ingress",
			fields: fields{
				IngressUpdaterFn: func(t *testing.T) IngressUpdater {
					m := NewMockIngressUpdater(t)
					m.EXPECT().
						UpsertForService(t.Context(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(assert.AnError)
					return m
				},
				NetworkPolicyUpdaterFn: func(t *testing.T) NetworkPolicyUpdater {
					m := NewMockNetworkPolicyUpdater(t)
					return m
				},
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &corev1.Service{}).
						Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
							svc := obj.(*corev1.Service)
							svc.Name = "test"
							svc.Namespace = testNamespace
						}).
						Return(nil)
					return m
				},
			},
			req: controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to create/update ingress object of service [test]", i...)
			},
		},
		{
			name: "fail to upsert netpols",
			fields: fields{
				IngressUpdaterFn: func(t *testing.T) IngressUpdater {
					m := NewMockIngressUpdater(t)
					m.EXPECT().
						UpsertForService(t.Context(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(nil)
					return m
				},
				NetworkPolicyUpdaterFn: func(t *testing.T) NetworkPolicyUpdater {
					m := NewMockNetworkPolicyUpdater(t)
					m.EXPECT().
						UpsertNetworkPoliciesForService(t.Context(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(assert.AnError)
					return m
				},
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &corev1.Service{}).
						Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
							svc := obj.(*corev1.Service)
							svc.Name = "test"
							svc.Namespace = testNamespace
						}).
						Return(nil)
					return m
				},
			},
			req: controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to create/update network policies for service [test]", i...)
			},
		},
		{
			name: "success",
			fields: fields{
				IngressUpdaterFn: func(t *testing.T) IngressUpdater {
					m := NewMockIngressUpdater(t)
					m.EXPECT().
						UpsertForService(t.Context(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(nil)
					return m
				},
				NetworkPolicyUpdaterFn: func(t *testing.T) NetworkPolicyUpdater {
					m := NewMockNetworkPolicyUpdater(t)
					m.EXPECT().
						UpsertNetworkPoliciesForService(t.Context(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(nil)
					return m
				},
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &corev1.Service{}).
						Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
							svc := obj.(*corev1.Service)
							svc.Name = "test"
							svc.Namespace = testNamespace
						}).
						Return(nil)
					return m
				},
			},
			req:     controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ServiceReconciler{
				IngressUpdater:       tt.fields.IngressUpdaterFn(t),
				NetworkPolicyUpdater: tt.fields.NetworkPolicyUpdaterFn(t),
				Client:               tt.fields.ClientFn(t),
			}
			got, err := r.Reconcile(t.Context(), tt.req)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, reconcile.Result{}, got)
		})
	}
}

func TestServiceReconciler_mapRequestsFromMaintenanceConfigMap(t *testing.T) {
	doguLabelReq, err := labels.NewRequirement("dogu.name", selection.Exists, nil)
	require.NoError(t, err)
	doguLabelSelector := labels.NewSelector().Add(*doguLabelReq)
	tests := []struct {
		name     string
		clientFn func(t *testing.T) client.Client
		object   client.Object
		want     []reconcile.Request
	}{
		{
			name:   "name doesn't match",
			object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "some-name"}},
			clientFn: func(t *testing.T) client.Client {
				return newMockK8sClient(t)
			},
			want: nil,
		},
		{
			name:   "fail to list",
			object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "maintenance", Namespace: testNamespace}},
			clientFn: func(t *testing.T) client.Client {
				m := newMockK8sClient(t)
				m.EXPECT().
					List(t.Context(), &corev1.ServiceList{}, &client.ListOptions{
						Namespace:     testNamespace,
						LabelSelector: doguLabelSelector,
					}).Return(assert.AnError)
				return m
			},
			want: nil,
		},
		{
			name:   "success",
			object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "maintenance", Namespace: testNamespace}},
			clientFn: func(t *testing.T) client.Client {
				m := newMockK8sClient(t)
				m.EXPECT().
					List(t.Context(), &corev1.ServiceList{}, &client.ListOptions{
						Namespace:     testNamespace,
						LabelSelector: doguLabelSelector,
					}).Run(func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) {
					list.(*corev1.ServiceList).Items = []corev1.Service{
						{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Labels: map[string]string{"dogu.name": "dogu1"}}},
						{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Labels: map[string]string{"dogu.name": "dogu2"}}},
						{ObjectMeta: metav1.ObjectMeta{Namespace: testNamespace, Labels: map[string]string{"dogu.name": "dogu3"}}},
					}
				}).Return(nil)
				return m
			},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "dogu1"}},
				{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "dogu2"}},
				{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "dogu3"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ServiceReconciler{
				Client: tt.clientFn(t),
			}
			assert.Equal(t, tt.want, r.mapRequestsFromMaintenanceConfigMap(t.Context(), tt.object))
		})
	}
}
