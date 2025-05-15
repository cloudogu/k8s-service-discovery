package ssl

import (
	"fmt"
	"testing"

	"github.com/cloudogu/k8s-registry-lib/config"
	regErrs "github.com/cloudogu/k8s-registry-lib/errors"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_certificateSynchronizer_Synchronize(t *testing.T) {
	type fields struct {
		secretInterfaceFn  func(t *testing.T) SecretClient
		globalConfigRepoFn func(t *testing.T) GlobalConfigRepository
	}
	tests := []struct {
		name    string
		fields  fields
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
			wantErr: assert.NoError,
		},
		{
			name: "should succeed and delete private key",
			fields: fields{
				secretInterfaceFn: func(t *testing.T) SecretClient {
					m := NewMockSecretClient(t)
					m.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(createCertificateSecret(), nil)
					return m
				},
				globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
					m := NewMockGlobalConfigRepository(t)
					var err error
					initialGlobalConfig := config.CreateGlobalConfig(config.Entries{})
					initialGlobalConfig.Config, err = initialGlobalConfig.Set("certificate/server.key", "mykey")
					assert.NoError(t, err)
					m.EXPECT().Get(testCtx).Return(initialGlobalConfig, nil)
					expectedGlobalConfig := config.CreateGlobalConfig(config.Entries{})
					expectedGlobalConfig.Config, err = expectedGlobalConfig.Set("certificate/server.key", "mykey")
					assert.NoError(t, err)
					expectedGlobalConfig.Config, err = expectedGlobalConfig.Set("certificate/server.crt", "mycert")
					assert.NoError(t, err)
					expectedGlobalConfig.Config = expectedGlobalConfig.Delete("certificate/server.key")
					m.EXPECT().Update(testCtx, expectedGlobalConfig).Return(config.GlobalConfig{}, nil).Once()
					return m
				},
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &certificateSynchronizer{
				secretInterface:  tt.fields.secretInterfaceFn(t),
				globalConfigRepo: tt.fields.globalConfigRepoFn(t),
			}
			err := r.Synchronize(testCtx)
			tt.wantErr(t, err, fmt.Sprintf("Synchronize(%v)", testCtx))
		})
	}
}

func createCertificateSecret() *v1.Secret {
	return &v1.Secret{Data: map[string][]byte{
		v1.TLSCertKey: []byte("mycert"),
	}}
}

func TestNewCertificateSynchronizer(t *testing.T) {
	secretClient := NewMockSecretClient(t)
	globalConfigRepo := NewMockGlobalConfigRepository(t)
	synchronizer := NewCertificateSynchronizer(secretClient, globalConfigRepo)
	assert.NotEmpty(t, synchronizer)
}
