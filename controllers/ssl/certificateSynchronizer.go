package ssl

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/errors"
	"github.com/cloudogu/retry-lib/retry"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	certificateSecretName  = "ecosystem-certificate"
	serverCertificateID    = "certificate/server.crt"
	serverCertificateKeyID = "certificate/server.key"
)

func NewCertificateSynchronizer(secretInterface secretClient, globalConfigRepo GlobalConfigRepository) *certificateSynchronizer {
	return &certificateSynchronizer{secretInterface: secretInterface, globalConfigRepo: globalConfigRepo}
}

type certificateSynchronizer struct {
	secretInterface  secretClient
	globalConfigRepo GlobalConfigRepository
}

func (s *certificateSynchronizer) Start(ctx context.Context) error {
	return s.Synchronize(ctx)
}

func (s *certificateSynchronizer) Synchronize(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)

	secret, err := s.secretInterface.Get(ctx, certificateSecretName, metav1.GetOptions{})
	if err != nil {
		return client.IgnoreNotFound(fmt.Errorf("failed to get ecosystem certificate secret: %w", err))
	}

	certificateBytes, exists := secret.Data[v1.TLSCertKey]
	if !exists {
		return fmt.Errorf("could not find certificate in ecosystem certificate secret")
	}

	logger.Info("Updating ecosystem certificate in global config...")
	err = s.updateInGlobalConfig(ctx, certificateBytes)
	if err != nil {
		return fmt.Errorf("failed to update ecosystem certificate in global config: %w", err)
	}

	logger.Info("Updated ecosystem certificate in global config")
	return nil
}

func (s *certificateSynchronizer) updateInGlobalConfig(ctx context.Context, certificateBytes []byte) error {
	return retry.OnError(1000, errors.IsConflictError, func() error {
		globalConfig, err := s.globalConfigRepo.Get(ctx)
		if err != nil {
			return fmt.Errorf("failed to get global config object: %w", err)
		}

		globalConfig.Config, err = globalConfig.Set(serverCertificateID, config.Value(certificateBytes))
		if err != nil {
			return fmt.Errorf("failed to set ecosystem certificate in global config object: %w", err)
		}

		// delete private key since it is a security risk
		globalConfig.Config = globalConfig.Delete(serverCertificateKeyID)

		_, err = s.globalConfigRepo.Update(ctx, globalConfig)
		if err != nil {
			return fmt.Errorf("failed write global config object: %w", err)
		}

		return nil
	})
}
