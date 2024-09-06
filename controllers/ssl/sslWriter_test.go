package ssl

import (
	"context"
	registryconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_sslWriter_WriteCertificate(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{}),
		}

		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)
		globalConfigRepoMock.EXPECT().Update(testCtx, mock.Anything).RunAndReturn(func(ctx context.Context, config registryconfig.GlobalConfig) (registryconfig.GlobalConfig, error) {
			assert.Equal(t, 3, len(config.GetChangeHistory()))
			certType, _ := config.Get("certificate/type")
			assert.Equal(t, "self-signed", certType.String())
			cert, _ := config.Get("certificate/server.crt")
			assert.Equal(t, "cert", cert.String())
			key, _ := config.Get("certificate/server.key")
			assert.Equal(t, "key", key.String())

			return registryconfig.GlobalConfig{}, nil
		})
		writer := NewSSLWriter(globalConfigRepoMock)

		// when
		err := writer.WriteCertificate(testCtx, "self-signed", "cert", "key")

		// then
		require.NoError(t, err)
	})

	t.Run("failed to write type", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"certificate/type/key": "already a dictionary",
			}),
		}

		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)
		writer := NewSSLWriter(globalConfigRepoMock)

		// when
		err := writer.WriteCertificate(testCtx, "self-signed", "cert", "key")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set certificate type")
	})

	t.Run("failed to write certificate", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"certificate/server.crt/key": "already a dictionary",
			}),
		}

		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)
		writer := NewSSLWriter(globalConfigRepoMock)

		// when
		err := writer.WriteCertificate(testCtx, "self-signed", "cert", "key")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set certificate")
	})

	t.Run("failed to write certificate key", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"certificate/server.key/key": "already a dictionary",
			}),
		}

		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)
		writer := NewSSLWriter(globalConfigRepoMock)

		// when
		err := writer.WriteCertificate(testCtx, "self-signed", "cert", "key")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to set certificate key")
	})
}
