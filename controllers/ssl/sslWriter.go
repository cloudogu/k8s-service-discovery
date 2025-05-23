package ssl

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type sslWriter struct {
	secretClient secretClient
}

// NewSSLWriter creates a new sslWriter instance to write certificate information to the ecosystem-certificate secret.
func NewSSLWriter(secretClient secretClient) *sslWriter {
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

	certificateSecret.Data[v1.TLSCertKey] = []byte(cert)
	certificateSecret.Data[v1.TLSPrivateKeyKey] = []byte(key)

	_, err = sw.secretClient.Update(ctx, certificateSecret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update secret writing ssl: %w", err)
	}

	return nil
}
