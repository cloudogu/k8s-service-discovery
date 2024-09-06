package ssl

import (
	registryconfig "github.com/cloudogu/k8s-registry-lib/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_creator_CreateAndSafeCertificate(t *testing.T) {
	t.Run("should return an error if fqdn is not set", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(registryconfig.GlobalConfig{}, nil)

		sut := &creator{
			globalConfigRepo: globalConfigRepoMock,
			sslGenerator:     nil,
			sslWriter:        nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(testCtx, 1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "fqdn is empty or doesn't exists")
	})
	t.Run("should return an error if fqdn is empty", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"fqdn": "",
			}),
		}
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		sut := &creator{
			globalConfigRepo: globalConfigRepoMock,
			sslGenerator:     nil,
			sslWriter:        nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(testCtx, 1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "fqdn is empty or doesn't exists")
	})
	t.Run("should return an error if domain is not set", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"fqdn": "192.168.56.2",
			}),
		}
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		sut := &creator{
			globalConfigRepo: globalConfigRepoMock,
			sslGenerator:     nil,
			sslWriter:        nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(testCtx, 1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "domain is empty or doesn't exists")
	})
	t.Run("should return an error if domain is empty", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"fqdn":   "192.168.56.2",
				"domain": "",
			}),
		}
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		sut := &creator{
			globalConfigRepo: globalConfigRepoMock,
			sslGenerator:     nil,
			sslWriter:        nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(testCtx, 1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "domain is empty or doesn't exists")
	})

	t.Run("should return an error if certificate can not be generated", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"fqdn":   "192.168.56.2",
				"domain": "ces.local",
			}),
		}
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("192.168.56.2", "ces.local", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", assert.AnError)

		sut := &creator{
			globalConfigRepo: globalConfigRepoMock,
			sslGenerator:     sslGeneratorMock,
			sslWriter:        nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(testCtx, 1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to generate self-signed certificate and key")
	})
	t.Run("should return an error if certificate can not be written", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"fqdn":   "192.168.56.2",
				"domain": "ces.local",
			}),
		}
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("192.168.56.2", "ces.local", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", nil)

		sslWriterMock := newMockCesSSLWriter(t)
		sslWriterMock.EXPECT().WriteCertificate(testCtx, "selfsigned", "mycert", "mykey").Return(assert.AnError)

		sut := &creator{
			globalConfigRepo: globalConfigRepoMock,
			sslGenerator:     sslGeneratorMock,
			sslWriter:        sslWriterMock,
		}

		// when
		err := sut.CreateAndSafeCertificate(testCtx, 1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to write certificate to global config")
	})
	t.Run("should succeed", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfig := registryconfig.GlobalConfig{
			Config: registryconfig.CreateConfig(registryconfig.Entries{
				"fqdn":   "192.168.56.2",
				"domain": "ces.local",
			}),
		}
		globalConfigRepoMock.EXPECT().Get(testCtx).Return(globalConfig, nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("192.168.56.2", "ces.local", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", nil)

		sslWriterMock := newMockCesSSLWriter(t)
		sslWriterMock.EXPECT().WriteCertificate(testCtx, "selfsigned", "mycert", "mykey").Return(nil)

		sut := &creator{
			globalConfigRepo: globalConfigRepoMock,
			sslGenerator:     sslGeneratorMock,
			sslWriter:        sslWriterMock,
		}

		// when
		err := sut.CreateAndSafeCertificate(testCtx, 1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.NoError(t, err)
	})
}
