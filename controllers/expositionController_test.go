package controllers

import (
	"context"
	"testing"

	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/adapter"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func int32Ptr(v int32) *int32 { return &v }

func stringPtr(v string) *string { return &v }

type assertingStatusWriter struct {
	t           *testing.T
	assertFunc  func(t *testing.T, obj client.Object)
	assertFuncs []func(t *testing.T, obj client.Object)
	updateErr   error
	updateErrs  []error
	calls       int
}

func (w *assertingStatusWriter) Create(context.Context, client.Object, client.Object, ...client.SubResourceCreateOption) error {
	w.t.Fatalf("unexpected status create")
	return nil
}

func (w *assertingStatusWriter) Update(_ context.Context, obj client.Object, _ ...client.SubResourceUpdateOption) error {
	callIndex := w.calls
	if len(w.assertFuncs) > 0 {
		require.Less(w.t, callIndex, len(w.assertFuncs))
		w.assertFuncs[callIndex](w.t, obj)
		w.calls++
	} else {
		w.assertFunc(w.t, obj)
	}
	if len(w.updateErrs) > 0 {
		require.Less(w.t, callIndex, len(w.updateErrs))
		return w.updateErrs[callIndex]
	}
	return w.updateErr
}

func (w *assertingStatusWriter) Patch(context.Context, client.Object, client.Patch, ...client.SubResourcePatchOption) error {
	w.t.Fatalf("unexpected status patch")
	return nil
}

func (w *assertingStatusWriter) Apply(context.Context, runtime.ApplyConfiguration, ...client.SubResourceApplyOption) error {
	w.t.Fatalf("unexpected status apply")
	return nil
}

func assertReadyCondition(
	t *testing.T,
	obj client.Object,
	conditionType string,
	status metav1.ConditionStatus,
	reason string,
	message string,
	observedGeneration int64,
) {
	t.Helper()

	cr, ok := obj.(*expositionv1.Exposition)
	require.True(t, ok)

	condition := meta.FindStatusCondition(cr.Status.Conditions, conditionType)
	require.NotNil(t, condition)
	assert.Equal(t, status, condition.Status)
	assert.Equal(t, reason, condition.Reason)
	assert.Equal(t, message, condition.Message)
	assert.Equal(t, observedGeneration, condition.ObservedGeneration)
}

func assertInitializedConditions(t *testing.T, obj client.Object, observedGeneration int64) {
	t.Helper()

	assertReadyCondition(t, obj, validConditionType, metav1.ConditionUnknown, "Initializing", "", observedGeneration)
	assertReadyCondition(t, obj, adapter.IngressesConditionType, metav1.ConditionUnknown, "Initializing", "", observedGeneration)
	assertReadyCondition(t, obj, adapter.NetworkPolicyConditionType, metav1.ConditionUnknown, "Initializing", "", observedGeneration)
}

func TestExpositionReconciler_Reconcile(t *testing.T) {
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
			name: "fail to get exposition",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &expositionv1.Exposition{}).
						Return(assert.AnError)
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					return NewMockExpositionService(t)
				},
			},
			req: controllerruntime.Request{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get exposition", i...)
			},
		},
		{
			name: "exposition not found",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &expositionv1.Exposition{}).
						Return(&apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					return NewMockExpositionService(t)
				},
			},
			req:     controllerruntime.Request{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: assert.NoError,
		},
		{
			name: "fail to process exposition",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &expositionv1.Exposition{}).
						Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							cr := obj.(*expositionv1.Exposition)
							cr.Name = "test"
							cr.Namespace = testNamespace
							cr.Generation = 2
						}).
						Return(nil)
					m.EXPECT().Status().Return(&assertingStatusWriter{
						t: t,
						assertFuncs: []func(t *testing.T, obj client.Object){
							func(t *testing.T, obj client.Object) {
								assertInitializedConditions(t, obj, 2)
							},
							func(t *testing.T, obj client.Object) {
								assertReadyCondition(
									t,
									obj,
									validConditionType,
									metav1.ConditionTrue,
									mappingSuccessfulConditionReason,
									mappingSuccessfulConditionMessage,
									2,
								)
							},
						},
					})
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
			req: controllerruntime.Request{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to process exposition from exposition CR", i...)
			},
		},
		{
			name: "success",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &expositionv1.Exposition{}).
						Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							cr := obj.(*expositionv1.Exposition)
							cr.Name = "test"
							cr.Namespace = testNamespace
							cr.Generation = 3
						}).
						Return(nil)
					m.EXPECT().Status().Return(&assertingStatusWriter{
						t: t,
						assertFuncs: []func(t *testing.T, obj client.Object){
							func(t *testing.T, obj client.Object) {
								assertInitializedConditions(t, obj, 3)
							},
							func(t *testing.T, obj client.Object) {
								assertReadyCondition(
									t,
									obj,
									validConditionType,
									metav1.ConditionTrue,
									mappingSuccessfulConditionReason,
									mappingSuccessfulConditionMessage,
									3,
								)
							},
						},
					})
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					m := NewMockExpositionService(t)
					m.EXPECT().
						ProcessExposition(t.Context(), mock.MatchedBy(func(e types.Exposition) bool {
							return e.Name == "test" && e.Namespace == testNamespace && e.SetOwner != nil && e.SetCondition != nil
						})).
						Return(nil)
					return m
				},
			},
			req:     controllerruntime.Request{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: assert.NoError,
		},
		{
			name: "fail to update validation status",
			fields: fields{
				ClientFn: func(t *testing.T) client.Client {
					m := newMockK8sClient(t)
					m.EXPECT().
						Get(t.Context(), expectedKey, &expositionv1.Exposition{}).
						Run(func(_ context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) {
							cr := obj.(*expositionv1.Exposition)
							cr.Name = "test"
							cr.Namespace = testNamespace
							cr.Generation = 4
						}).
						Return(nil)
					m.EXPECT().Status().Return(&assertingStatusWriter{
						t: t,
						assertFuncs: []func(t *testing.T, obj client.Object){
							func(t *testing.T, obj client.Object) {
								assertInitializedConditions(t, obj, 4)
							},
							func(t *testing.T, obj client.Object) {
								assertReadyCondition(
									t,
									obj,
									validConditionType,
									metav1.ConditionTrue,
									mappingSuccessfulConditionReason,
									mappingSuccessfulConditionMessage,
									4,
								)
							},
						},
						updateErrs: []error{nil, assert.AnError},
					})
					return m
				},
				ExpositionServiceFn: func(t *testing.T) ExpositionService {
					return NewMockExpositionService(t)
				},
			},
			req: controllerruntime.Request{NamespacedName: k8stypes.NamespacedName{Namespace: testNamespace, Name: "test"}},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ExpositionReconciler{
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

func Test_mapExpositionCRToHttpRoutes(t *testing.T) {
	tests := []struct {
		name string
		cr   *expositionv1.Exposition
		want []types.HttpRoute
	}{
		{
			name: "empty spec.HTTP yields nil",
			cr:   &expositionv1.Exposition{},
			want: nil,
		},
		{
			name: "one entry without rewrite",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "ldap"}, Spec: expositionv1.ExpositionSpec{HTTP: []expositionv1.HTTPEntry{
				{Name: "ui", Service: "ldap-ui", Port: 8080, Path: "/ldap"},
			}}},
			want: []types.HttpRoute{
				{Name: "ldap-ui-8080", Service: "ldap-ui", Port: 8080, Path: "/ldap", Rewrite: nil},
			},
		},
		{
			name: "entry with strip-prefix only",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "ldap"}, Spec: expositionv1.ExpositionSpec{HTTP: []expositionv1.HTTPEntry{
				{Name: "ui", Service: "ldap-ui", Port: 8080, Path: "/ldap", Rewrite: &expositionv1.Rewrite{
					StripPrefix: stringPtr("/ldap"),
				}},
			}}},
			want: []types.HttpRoute{
				{Name: "ldap-ui-8080", Service: "ldap-ui", Port: 8080, Path: "/ldap", Rewrite: &types.HttpRewrite{
					StripPrefix: stringPtr("/ldap"),
					Regex:       nil,
				}},
			},
		},
		{
			name: "entry with regex only",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "ldap"}, Spec: expositionv1.ExpositionSpec{HTTP: []expositionv1.HTTPEntry{
				{Name: "ui", Service: "ldap-ui", Port: 8080, Path: "/ldap", Rewrite: &expositionv1.Rewrite{
					Regex: &expositionv1.RegexRewrite{Pattern: "^/old", Replacement: "/new"},
				}},
			}}},
			want: []types.HttpRoute{
				{Name: "ldap-ui-8080", Service: "ldap-ui", Port: 8080, Path: "/ldap", Rewrite: &types.HttpRewrite{
					StripPrefix: nil,
					Regex:       &types.RegexReplacement{Pattern: "^/old", Replacement: "/new"},
				}},
			},
		},
		{
			name: "two entries preserve order",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "ldap"}, Spec: expositionv1.ExpositionSpec{HTTP: []expositionv1.HTTPEntry{
				{Name: "a", Service: "svc-a", Port: 80, Path: "/a"},
				{Name: "b", Service: "svc-b", Port: 81, Path: "/b"},
			}}},
			want: []types.HttpRoute{
				{Name: "ldap-a-80", Service: "svc-a", Port: 80, Path: "/a"},
				{Name: "ldap-b-81", Service: "svc-b", Port: 81, Path: "/b"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mapExpositionCRToHttpRoutes(tt.cr))
		})
	}
}

func Test_mapExpositionCRToTCPExposedPorts(t *testing.T) {
	tests := []struct {
		name string
		cr   *expositionv1.Exposition
		want types.ExposedPorts
	}{
		{
			name: "empty spec.TCP yields empty slice",
			cr:   &expositionv1.Exposition{},
			want: types.ExposedPorts{},
		},
		{
			name: "RequestedExternalPort nil falls back to Port",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "scm"}, Spec: expositionv1.ExpositionSpec{TCP: []expositionv1.TCPEntry{
				{Name: "ssh", Service: "scm", Port: 22, RequestedExternalPort: nil},
			}}},
			want: types.ExposedPorts{
				{Name: "scm-ssh", ServiceName: "scm", Protocol: corev1.ProtocolTCP, ServicePort: 22, RequestedExternalPort: 22},
			},
		},
		{
			name: "RequestedExternalPort set is preserved",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "scm"}, Spec: expositionv1.ExpositionSpec{TCP: []expositionv1.TCPEntry{
				{Name: "ssh", Service: "scm", Port: 22, RequestedExternalPort: int32Ptr(33000)},
			}}},
			want: types.ExposedPorts{
				{Name: "scm-ssh", ServiceName: "scm", Protocol: corev1.ProtocolTCP, ServicePort: 22, RequestedExternalPort: 33000},
			},
		},
		{
			name: "two entries mix fallback and explicit",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "scm"}, Spec: expositionv1.ExpositionSpec{TCP: []expositionv1.TCPEntry{
				{Name: "ssh", Service: "scm", Port: 22, RequestedExternalPort: nil},
				{Name: "ldaps", Service: "ldap", Port: 636, RequestedExternalPort: int32Ptr(33001)},
			}}},
			want: types.ExposedPorts{
				{Name: "scm-ssh", ServiceName: "scm", Protocol: corev1.ProtocolTCP, ServicePort: 22, RequestedExternalPort: 22},
				{Name: "scm-ldaps", ServiceName: "ldap", Protocol: corev1.ProtocolTCP, ServicePort: 636, RequestedExternalPort: 33001},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mapExpositionCRToTCPExposedPorts(tt.cr))
		})
	}
}

func Test_mapExpositionCRToUDPExposedPorts(t *testing.T) {
	tests := []struct {
		name string
		cr   *expositionv1.Exposition
		want types.ExposedPorts
	}{
		{
			name: "empty spec.UDP yields empty slice",
			cr:   &expositionv1.Exposition{},
			want: types.ExposedPorts{},
		},
		{
			name: "RequestedExternalPort nil falls back to Port",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "scm"}, Spec: expositionv1.ExpositionSpec{UDP: []expositionv1.UDPEntry{
				{Name: "dns", Service: "dns", Port: 53, RequestedExternalPort: nil},
			}}},
			want: types.ExposedPorts{
				{Name: "scm-dns", ServiceName: "dns", Protocol: corev1.ProtocolUDP, ServicePort: 53, RequestedExternalPort: 53},
			},
		},
		{
			name: "RequestedExternalPort set is preserved",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "scm"}, Spec: expositionv1.ExpositionSpec{UDP: []expositionv1.UDPEntry{
				{Name: "dns", Service: "dns", Port: 53, RequestedExternalPort: int32Ptr(35353)},
			}}},
			want: types.ExposedPorts{
				{Name: "scm-dns", ServiceName: "dns", Protocol: corev1.ProtocolUDP, ServicePort: 53, RequestedExternalPort: 35353},
			},
		},
		{
			name: "two entries mix fallback and explicit",
			cr: &expositionv1.Exposition{ObjectMeta: metav1.ObjectMeta{Name: "scm"}, Spec: expositionv1.ExpositionSpec{UDP: []expositionv1.UDPEntry{
				{Name: "dns", Service: "dns", Port: 53, RequestedExternalPort: nil},
				{Name: "ntp", Service: "ntp", Port: 123, RequestedExternalPort: int32Ptr(35123)},
			}}},
			want: types.ExposedPorts{
				{Name: "scm-dns", ServiceName: "dns", Protocol: corev1.ProtocolUDP, ServicePort: 53, RequestedExternalPort: 53},
				{Name: "scm-ntp", ServiceName: "ntp", Protocol: corev1.ProtocolUDP, ServicePort: 123, RequestedExternalPort: 35123},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mapExpositionCRToUDPExposedPorts(tt.cr))
		})
	}
}

func Test_mapExpositionCRToExposition(t *testing.T) {
	scheme := getScheme(t)
	c := newMockK8sClient(t)
	cr := &expositionv1.Exposition{
		ObjectMeta: metav1.ObjectMeta{Name: "ldap", Namespace: testNamespace, UID: "owner-uid"},
		Spec: expositionv1.ExpositionSpec{
			HTTP: []expositionv1.HTTPEntry{{Name: "ui", Service: "ldap-ui", Port: 8080, Path: "/ldap"}},
			TCP:  []expositionv1.TCPEntry{{Name: "ssh", Service: "scm", Port: 22}},
			UDP:  []expositionv1.UDPEntry{{Name: "dns", Service: "dns", Port: 53}},
		},
	}

	t.Run("happy path populates every section", func(t *testing.T) {
		got, err := mapExpositionCRToExposition(cr, c)
		require.NoError(t, err)
		assert.Equal(t, "ldap", got.Name)
		assert.Equal(t, testNamespace, got.Namespace)
		assert.Len(t, got.HttpRoutes, 1)
		assert.Len(t, got.TcpRoutes, 1)
		assert.Len(t, got.UdpRoutes, 1)
		assert.NotNil(t, got.SetOwner)
		assert.NotNil(t, got.SetCondition)
	})

	t.Run("SetOwner wires the CR as controller", func(t *testing.T) {
		c := newMockK8sClient(t)
		c.EXPECT().Scheme().Return(scheme)
		got, err := mapExpositionCRToExposition(cr, c)
		require.NoError(t, err)

		target := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "tgt", Namespace: testNamespace}}
		require.NoError(t, got.SetOwner(target))

		require.Len(t, target.OwnerReferences, 1)
		assert.Equal(t, "ldap", target.OwnerReferences[0].Name)
		require.NotNil(t, target.OwnerReferences[0].Controller)
		assert.True(t, *target.OwnerReferences[0].Controller)
	})
}
