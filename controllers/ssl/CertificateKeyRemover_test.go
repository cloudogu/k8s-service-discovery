package ssl

import (
	"fmt"
	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewCertificateKeyRemover(t *testing.T) {
	globalConfigRepositoryMock := NewMockGlobalConfigRepository(t)

	remover := NewCertificateKeyRemover(globalConfigRepositoryMock)

	require.NotNil(t, remover)
}

func Test_certificateKeyRemover_RemoveCertificateKeyOutOfGlobalConfig(t *testing.T) {
	tests := []struct {
		name               string
		globalConfigRepoFn func(t *testing.T) GlobalConfigRepository
		wantErr            assert.ErrorAssertionFunc
	}{
		{
			name: "should fail to get global config",
			globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
				m := NewMockGlobalConfigRepository(t)
				m.EXPECT().Get(testCtx).Return(config.GlobalConfig{}, assert.AnError)
				return m
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to get global config object", i)
			},
		},
		{
			name: "should fail to write global config",
			globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
				m := NewMockGlobalConfigRepository(t)
				m.EXPECT().Get(testCtx).Return(config.CreateGlobalConfig(config.Entries{}), nil)
				m.EXPECT().Update(testCtx, mock.Anything).Return(config.GlobalConfig{}, assert.AnError)
				return m
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to write global config object", i)
			},
		},
		{
			name: "should fail to write global config",
			globalConfigRepoFn: func(t *testing.T) GlobalConfigRepository {
				m := NewMockGlobalConfigRepository(t)
				m.EXPECT().Get(testCtx).Return(config.CreateGlobalConfig(config.Entries{}), nil)
				m.EXPECT().Update(testCtx, mock.Anything).Return(config.GlobalConfig{}, nil)
				return m
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ckr := &certificateKeyRemover{
				globalConfigRepo: tt.globalConfigRepoFn(t),
			}
			tt.wantErr(t, ckr.RemoveCertificateKeyOutOfGlobalConfig(testCtx), fmt.Sprintf("RemoveCertificateKeyOutOfGlobalConfig(%v)", testCtx))
		})
	}
}
