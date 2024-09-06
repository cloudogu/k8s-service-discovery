package ssl

import (
	"context"
	"fmt"
	libconfig "github.com/cloudogu/k8s-registry-lib/config"
)

type sslWriter struct {
	globalConfigRepo GlobalConfigRepository
}

// NewSSLWriter creates a new sslWriter instance to write certificate information in the global config
func NewSSLWriter(globalConfigRepo GlobalConfigRepository) *sslWriter {
	return &sslWriter{globalConfigRepo: globalConfigRepo}
}

// WriteCertificate writes the type, cert and key to the global config
func (sw *sslWriter) WriteCertificate(ctx context.Context, certType string, cert string, key string) error {
	globalConfig, err := sw.globalConfigRepo.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get global config for ssl creation: %w", err)
	}

	globalConfig.Config, err = globalConfig.Set("certificate/type", libconfig.Value(certType))
	if err != nil {
		return fmt.Errorf("failed to set certificate type: %w", err)
	}

	globalConfig.Config, err = globalConfig.Set("certificate/server.crt", libconfig.Value(cert))
	if err != nil {
		return fmt.Errorf("failed to set certificate: %w", err)
	}

	globalConfig.Config, err = globalConfig.Set("certificate/server.key", libconfig.Value(key))
	if err != nil {
		return fmt.Errorf("failed to set certificate key: %w", err)
	}

	_, err = sw.globalConfigRepo.Update(ctx, globalConfig)
	if err != nil {
		return fmt.Errorf("failed to update global config writing ssl: %w", err)
	}

	return nil
}
