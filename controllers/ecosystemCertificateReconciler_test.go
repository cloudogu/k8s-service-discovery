package controllers

import (
	"fmt"
	"github.com/cloudogu/k8s-registry-lib/config"
	regErrs "github.com/cloudogu/k8s-registry-lib/errors"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"testing"

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

func Test_ecosystemCertificateReconciler_Reconcile(t *testing.T) {
	type fields struct {
		secretInterfaceFn  func(t *testing.T) SecretClient
		globalConfigRepoFn func(t *testing.T) GlobalConfigRepository
	}
	tests := []struct {
		name    string
		fields  fields
		req     controllerruntime.Request
		want    controllerruntime.Result
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "should fail to get secret",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(nil, assert.AnError)
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					return m
				},
			},
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: "ecosystem-certificate"},
			},
			want: controllerruntime.Result{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i) &&
					assert.ErrorContains(t, err, "failed to get ecosystem certificate secret", i)
			},
		},
		{
			name: "should return without error if secret not found",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(nil, &errors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonNotFound}})
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					return m
				},
			},
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: "ecosystem-certificate"},
			},
			want:    controllerruntime.Result{},
			wantErr: assert.NoError,
		},
		{
			name: "should fail to find secret key",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&v1.Secret{}, nil)
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					return m
				},
			},
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: "ecosystem-certificate"},
			},
			want: controllerruntime.Result{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "could not find certificate in ecosystem certificate secret", i)
			},
		},
		{
			name: "should fail to get global config",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(createCertificateSecret(), nil)
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					m.EXPECT().Get(testCtx).Return(config.GlobalConfig{}, assert.AnError)
					return m
				},
			},
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: "ecosystem-certificate"},
			},
			want: controllerruntime.Result{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i) &&
					assert.ErrorContains(t, err, "failed to get global config", i) &&
					assert.ErrorContains(t, err, "failed to update ecosystem certificate in global config", i)
			},
		},
		{
			name: "should fail to set ecosystem certificate in global config object",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(createCertificateSecret(), nil)
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					globalConfig := config.CreateGlobalConfig(config.Entries{serverCertificateID + "/subkey_producing_error": ""})
					m.EXPECT().Get(testCtx).Return(globalConfig, nil)
					return m
				},
			},
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: "ecosystem-certificate"},
			},
			want: controllerruntime.Result{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to set ecosystem certificate in global config object", i) &&
					assert.ErrorContains(t, err, "failed to update ecosystem certificate in global config", i)
			},
		},
		{
			name: "should fail to write global config object",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(createCertificateSecret(), nil)
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					m.EXPECT().Get(testCtx).Return(config.CreateGlobalConfig(config.Entries{}), nil)
					expectedGlobalConfig := config.CreateGlobalConfig(config.Entries{})
					var err error
					expectedGlobalConfig.Config, err = expectedGlobalConfig.Set("certificate/server.crt", "mycert")
					assert.NoError(t, err)
					m.EXPECT().Update(testCtx, expectedGlobalConfig).Return(config.GlobalConfig{}, assert.AnError)
					return m
				},
			},
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: "ecosystem-certificate"},
			},
			want: controllerruntime.Result{},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError) &&
					assert.ErrorContains(t, err, "failed write global config object", i) &&
					assert.ErrorContains(t, err, "failed to update ecosystem certificate in global config", i)
			},
		},
		{
			name: "should succeed with retry on conflict",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(createCertificateSecret(), nil)
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					m.EXPECT().Get(testCtx).Return(config.CreateGlobalConfig(config.Entries{}), nil)
					expectedGlobalConfig := config.CreateGlobalConfig(config.Entries{})
					var err error
					expectedGlobalConfig.Config, err = expectedGlobalConfig.Set("certificate/server.crt", "mycert")
					assert.NoError(t, err)
					firstCall := m.On("Update", testCtx, expectedGlobalConfig).Return(config.GlobalConfig{}, regErrs.NewConflictError(assert.AnError)).Once()
					m.EXPECT().Update(testCtx, expectedGlobalConfig).Return(config.GlobalConfig{}, nil).NotBefore(firstCall).Once()
					return m
				},
			},
			req: controllerruntime.Request{
				NamespacedName: types.NamespacedName{Name: "ecosystem-certificate"},
			},
			want:    controllerruntime.Result{},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &ecosystemCertificateReconciler{
				secretInterface:  tt.fields.secretInterfaceFn(t),
				globalConfigRepo: tt.fields.globalConfigRepoFn(t),
			}
			got, err := r.Reconcile(testCtx, tt.req)
			if !tt.wantErr(t, err, fmt.Sprintf("Reconcile(%v, %v)", testCtx, tt.req)) {
				return
			}
			assert.Equalf(t, tt.want, got, "Reconcile(%v, %v)", testCtx, tt.req)
		})
	}
}

func createCertificateSecret() *v1.Secret {
	return &v1.Secret{Data: map[string][]byte{
		v1.TLSCertKey: []byte("mycert"),
	}}
}

func TestNewEcosystemCertificateReconciler(t *testing.T) {
	secretClient := NewMockSecretClient(t)
	globalConfigRepo := NewMockGlobalConfigRepository(t)
	reconciler := NewEcosystemCertificateReconciler(secretClient, globalConfigRepo)
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
