package ssl

import (
	"fmt"
)

type sslWriter struct {
	globalConfig setConfigurationContext
}

type setConfigurationContext interface {
	// Set sets a configuration value in current context
	Set(key, value string) error
}

// NewSSLWriter creates a new sslWriter instance to write certificate information in the global config
func NewSSLWriter(globalConfig setConfigurationContext) *sslWriter {
	return &sslWriter{globalConfig: globalConfig}
}

// WriteCertificate writes the type, cert and key to the global config
func (sw *sslWriter) WriteCertificate(certType string, cert string, key string) error {
	err := sw.globalConfig.Set("certificate/type", certType)
	if err != nil {
		return fmt.Errorf("failed to set certificate type: %w", err)
	}

	err = sw.globalConfig.Set("certificate/server.crt", cert)
	if err != nil {
		return fmt.Errorf("failed to set certificate: %w", err)
	}

	err = sw.globalConfig.Set("certificate/server.key", key)
	if err != nil {
		return fmt.Errorf("failed to set certificate key: %w", err)
	}

	return nil
}
