package ssl

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

const (
	cert = "cert"
	key  = "key"
)

func Test_sslWriter_WriteCertificate(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		// given

		secretClientMock := newMockSecretClient(t)
		secretClientMock.EXPECT().Get(testCtx, "ecosystem-certificate", v1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{}}, nil)
		secretClientMock.EXPECT().Update(testCtx, &corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(cert),
			"tls.key": []byte(key),
		}}, v1.UpdateOptions{}).Return(nil, nil)

		writer := NewSSLWriter(secretClientMock)

		// when
		err := writer.WriteCertificate(testCtx, cert, key)

		// then
		require.NoError(t, err)
	})

	t.Run("failed to get ecosystem-certificate secret", func(t *testing.T) {
		// given
		secretClientMock := newMockSecretClient(t)
		secretClientMock.EXPECT().Get(testCtx, "ecosystem-certificate", v1.GetOptions{}).Return(nil, assert.AnError)

		writer := NewSSLWriter(secretClientMock)

		// when
		err := writer.WriteCertificate(testCtx, cert, key)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get secret for ssl creation: ")
	})

	t.Run("failed to update certificate secret", func(t *testing.T) {
		// given
		secretClientMock := newMockSecretClient(t)
		secretClientMock.EXPECT().Get(testCtx, "ecosystem-certificate", v1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{}}, nil)
		secretClientMock.EXPECT().Update(testCtx, &corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(cert),
			"tls.key": []byte(key),
		}}, v1.UpdateOptions{}).Return(nil, assert.AnError)

		writer := NewSSLWriter(secretClientMock)

		// when
		err := writer.WriteCertificate(testCtx, cert, key)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update secret writing ssl:")
	})
}
