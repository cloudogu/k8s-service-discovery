package ssl

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	controllerruntime "sigs.k8s.io/controller-runtime"
)

type sslWriter struct {
	secretClient secretClient
	namespace    string
}

// NewSSLWriter creates a new sslWriter instance to write certificate information to the ecosystem-certificate secret.
func NewSSLWriter(secretClient secretClient, namespace string) *sslWriter {
	return &sslWriter{
		secretClient: secretClient,
		namespace:    namespace,
	}
}

// WriteCertificate writes the cert and key to the ecosystem-certificate secret
func (sw *sslWriter) WriteCertificate(ctx context.Context, cert string, key string) error {
	certificateSecret := sw.createCertificateSecret(cert, key)

	_, uErr := sw.secretClient.Update(ctx, certificateSecret, metav1.UpdateOptions{})
	if uErr == nil {
		return nil
	}

	if !apierrors.IsNotFound(uErr) {
		return fmt.Errorf("failed to update secret for ecosystem certificate: %w", uErr)
	}

	if _, cErr := sw.secretClient.Create(ctx, certificateSecret, metav1.CreateOptions{}); cErr != nil {
		return fmt.Errorf("failed to create new secret for ecosystem certificate: %w", cErr)
	}

	return nil
}

func (sw *sslWriter) createCertificateSecret(cert string, key string) *v1.Secret {
	return &v1.Secret{
		ObjectMeta: controllerruntime.ObjectMeta{
			Name:      certificateSecretName,
			Namespace: sw.namespace,
			Labels:    util.GetAppLabel(),
		},
		Data: map[string][]byte{
			v1.TLSCertKey:       []byte(cert),
			v1.TLSPrivateKeyKey: []byte(key),
		},
	}
}
