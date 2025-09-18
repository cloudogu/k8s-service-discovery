package ssl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	sslWriterNamespace = "default"
	cert               = "cert"
	key                = "key"
)

func Test_sslWriter_WriteCertificate(t *testing.T) {
	t.Parallel()

	t.Run("success- create new certificate", func(t *testing.T) {
		// given

		secretClientMock := newMockSecretClient(t)
		secretClientMock.EXPECT().Update(testCtx, mock.Anything, v1.UpdateOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, assert.AnError.Error()))

		writer := NewSSLWriter(secretClientMock, sslWriterNamespace)

		secretClientMock.EXPECT().Create(testCtx, writer.createCertificateSecret(cert, key), v1.CreateOptions{}).Return(nil, nil)

		// when
		err := writer.WriteCertificate(testCtx, cert, key)

		// then
		require.NoError(t, err)
	})

	t.Run("success- update certificate", func(t *testing.T) {
		// given

		secretClientMock := newMockSecretClient(t)
		writer := NewSSLWriter(secretClientMock, sslWriterNamespace)

		secretClientMock.EXPECT().Update(testCtx, writer.createCertificateSecret(cert, key), v1.UpdateOptions{}).Return(nil, nil)

		// when
		err := writer.WriteCertificate(testCtx, cert, key)

		// then
		require.NoError(t, err)
	})

	t.Run("failed to create new certificate", func(t *testing.T) {
		// given

		secretClientMock := newMockSecretClient(t)
		secretClientMock.EXPECT().Update(testCtx, mock.Anything, v1.UpdateOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, assert.AnError.Error()))

		writer := NewSSLWriter(secretClientMock, sslWriterNamespace)

		secretClientMock.EXPECT().Create(testCtx, writer.createCertificateSecret(cert, key), v1.CreateOptions{}).Return(nil, assert.AnError)

		// when
		err := writer.WriteCertificate(testCtx, cert, key)

		// then
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to create new secret for ecosystem certificate")
	})

	t.Run("failed to update certificate", func(t *testing.T) {
		// given

		secretClientMock := newMockSecretClient(t)
		writer := NewSSLWriter(secretClientMock, sslWriterNamespace)

		secretClientMock.EXPECT().Update(testCtx, writer.createCertificateSecret(cert, key), v1.UpdateOptions{}).Return(nil, assert.AnError)

		// when
		err := writer.WriteCertificate(testCtx, cert, key)

		// then
		require.Error(t, err)
		require.ErrorContains(t, err, "failed to update secret for ecosystem certificate")
	})
}
