package ssl

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_creator_CreateAndSafeCertificate(t *testing.T) {
	t.Run("should return an error if fqdn can not be queried", func(t *testing.T) {
		// given
		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("", assert.AnError)

		sut := &creator{
			globalConfig: globalConfigMock,
			sslGenerator: nil,
			sslWriter:    nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get FQDN from global config")
	})
	t.Run("should return an error if domain can not be queried", func(t *testing.T) {
		// given
		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("", nil)
		getExpect("domain").Return("", assert.AnError)

		sut := &creator{
			globalConfig: globalConfigMock,
			sslGenerator: nil,
			sslWriter:    nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get DOMAIN from global config")
	})
	t.Run("should return an error if certificate can not be generated", func(t *testing.T) {
		// given
		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("1.2.3.4", nil)
		getExpect("domain").Return("local.cloudogu.com", nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("1.2.3.4", "local.cloudogu.com", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", assert.AnError)

		sut := &creator{
			globalConfig: globalConfigMock,
			sslGenerator: sslGeneratorMock,
			sslWriter:    nil,
		}

		// when
		err := sut.CreateAndSafeCertificate(1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to generate self-signed certificate and key")
	})
	t.Run("should return an error if certificate can not be written", func(t *testing.T) {
		// given
		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("1.2.3.4", nil)
		getExpect("domain").Return("local.cloudogu.com", nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("1.2.3.4", "local.cloudogu.com", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", nil)

		sslWriterMock := newMockCesSSLWriter(t)
		sslWriterMock.EXPECT().WriteCertificate("selfsigned", "mycert", "mykey").Return(assert.AnError)

		sut := &creator{
			globalConfig: globalConfigMock,
			sslGenerator: sslGeneratorMock,
			sslWriter:    sslWriterMock,
		}

		// when
		err := sut.CreateAndSafeCertificate(1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to write certificate to global config")
	})
	t.Run("should succeed", func(t *testing.T) {
		// given
		globalConfigMock := newMockGlobalConfig(t)
		getExpect := globalConfigMock.EXPECT().Get
		getExpect("fqdn").Return("1.2.3.4", nil)
		getExpect("domain").Return("local.cloudogu.com", nil)

		sslGeneratorMock := newMockCesSelfSignedSSLGenerator(t)
		sslGeneratorMock.EXPECT().GenerateSelfSignedCert("1.2.3.4", "local.cloudogu.com", 1,
			"DE", "Lower Saxony", "Brunswick", []string{}).Return("mycert", "mykey", nil)

		sslWriterMock := newMockCesSSLWriter(t)
		sslWriterMock.EXPECT().WriteCertificate("selfsigned", "mycert", "mykey").Return(nil)

		sut := &creator{
			globalConfig: globalConfigMock,
			sslGenerator: sslGeneratorMock,
			sslWriter:    sslWriterMock,
		}

		// when
		err := sut.CreateAndSafeCertificate(1, "DE", "Lower Saxony", "Brunswick", []string{})

		// then
		require.NoError(t, err)
	})
}
