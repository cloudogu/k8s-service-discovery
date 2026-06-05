package controllers

import (
	"context"
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestExpositionReconciler_Reconcile(t *testing.T) {
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
			name: "fail to get exposition",
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
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &expositionv1.Exposition{}).
						Return(assert.AnError)
					return m
				},
			},
			req: controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get exposition", i...)
			},
		},
		{
			name: "exposition not found",
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
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &expositionv1.Exposition{}).
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
						UpsertForExposition(t.Context(), &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
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
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &expositionv1.Exposition{}).
						Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
							exp := obj.(*expositionv1.Exposition)
							exp.Name = "test"
							exp.Namespace = testNamespace
						}).
						Return(nil)
					return m
				},
			},
			req: controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to create/update ingress object of exposition [test]", i...)
			},
		},
		{
			name: "fail to upsert netpols",
			fields: fields{
				IngressUpdaterFn: func(t *testing.T) IngressUpdater {
					m := NewMockIngressUpdater(t)
					m.EXPECT().
						UpsertForExposition(t.Context(), &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(nil)
					return m
				},
				NetworkPolicyUpdaterFn: func(t *testing.T) NetworkPolicyUpdater {
					m := NewMockNetworkPolicyUpdater(t)
					m.EXPECT().
						UpsertNetworkPoliciesForExposition(t.Context(), &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(assert.AnError)
					return m
				},
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &expositionv1.Exposition{}).
						Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
							exp := obj.(*expositionv1.Exposition)
							exp.Name = "test"
							exp.Namespace = testNamespace
						}).
						Return(nil)
					return m
				},
			},
			req: controllerruntime.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to create/update network policies for exposition [test]", i...)
			},
		},
		{
			name: "success",
			fields: fields{
				IngressUpdaterFn: func(t *testing.T) IngressUpdater {
					m := NewMockIngressUpdater(t)
					m.EXPECT().
						UpsertForExposition(t.Context(), &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(nil)
					return m
				},
				NetworkPolicyUpdaterFn: func(t *testing.T) NetworkPolicyUpdater {
					m := NewMockNetworkPolicyUpdater(t)
					m.EXPECT().
						UpsertNetworkPoliciesForExposition(t.Context(), &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}}).
						Return(nil)
					return m
				},
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "test"}, &expositionv1.Exposition{}).
						Run(func(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) {
							exp := obj.(*expositionv1.Exposition)
							exp.Name = "test"
							exp.Namespace = testNamespace
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
			r := &ExpositionReconciler{
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
