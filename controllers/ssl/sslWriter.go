package ssl

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const certificateSecretName = "ecosystem-certificate"
const certificateSecretPublicKey = "tls.crt"
const certificateSecretPrivateKey = "tls.key"

type sslWriter struct {
	secretClient SecretClient
}

// NewSSLWriter creates a new sslWriter instance to write certificate information in the global config
func NewSSLWriter(secretClient SecretClient) *sslWriter {
	return &sslWriter{
		secretClient: secretClient,
	}
}

// WriteCertificate writes the cert and key to the ecosystem-certificate secret
func (sw *sslWriter) WriteCertificate(ctx context.Context, cert string, key string) error {
	certificateSecret, err := sw.secretClient.Get(ctx, certificateSecretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret for ssl creation: %w", err)
	}

	certificateSecret.Data[certificateSecretPublicKey] = []byte(cert)
	certificateSecret.Data[certificateSecretPrivateKey] = []byte(key)

	_, err = sw.secretClient.Update(ctx, certificateSecret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret writing ssl: %w", err)
	}

	return nil
}
