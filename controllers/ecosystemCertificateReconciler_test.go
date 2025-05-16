package controllers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestNewEcosystemCertificateReconciler(t *testing.T) {
	certSync := newMockCertificateSynchronizer(t)
	reconciler := NewEcosystemCertificateReconciler(certSync)
	assert.NotEmpty(t, reconciler)
}

func Test_ecosystemCertificatePredicate(t *testing.T) {
	certificatePredicate := ecosystemCertificatePredicate()
	t.Run("test create", func(t *testing.T) {
		assert.False(t, certificatePredicate.Create(event.CreateEvent{Object: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "some-other-name"}}}))
		assert.True(t, certificatePredicate.Create(event.CreateEvent{Object: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ecosystem-certificate"}}}))
	})

	t.Run("test delete", func(t *testing.T) {
		assert.False(t, certificatePredicate.Delete(event.DeleteEvent{Object: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "some-other-name"}}}))
		assert.True(t, certificatePredicate.Delete(event.DeleteEvent{Object: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ecosystem-certificate"}}}))
	})

	t.Run("test update", func(t *testing.T) {
		assert.False(t, certificatePredicate.Update(event.UpdateEvent{ObjectOld: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "some-other-name"}}}))
		assert.True(t, certificatePredicate.Update(event.UpdateEvent{ObjectOld: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ecosystem-certificate"}}}))
	})

	t.Run("test generic", func(t *testing.T) {
		assert.False(t, certificatePredicate.Generic(event.GenericEvent{Object: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "some-other-name"}}}))
		assert.True(t, certificatePredicate.Generic(event.GenericEvent{Object: &v1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ecosystem-certificate"}}}))
	})
}

func Test_ecosystemCertificateReconciler_Reconcile(t *testing.T) {
	request := controllerruntime.Request{NamespacedName: types.NamespacedName{Name: certificateSecretName, Namespace: "ecosystem"}}
	tests := []struct {
		name       string
		certSyncFn func(t *testing.T) certificateSynchronizer
		want       controllerruntime.Result
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name: "should fail",
			certSyncFn: func(t *testing.T) certificateSynchronizer {
				m := newMockCertificateSynchronizer(t)
				m.EXPECT().Synchronize(testCtx).Return(assert.AnError)
				return m
			},
			want: controllerruntime.Result{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError)
			},
		},
		{
			name: "should succeed",
			certSyncFn: func(t *testing.T) certificateSynchronizer {
				m := newMockCertificateSynchronizer(t)
				m.EXPECT().Synchronize(testCtx).Return(nil)
				return m
			},
			want:    controllerruntime.Result{},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ecosystemCertificateReconciler{
				certSync: tt.certSyncFn(t),
			}
			got, err := r.Reconcile(testCtx, request)
			if !tt.wantErr(t, err, fmt.Sprintf("Reconcile(%v, %v)", testCtx, request)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Reconcile(%v, %v)", testCtx, request)
		})
	}
}
