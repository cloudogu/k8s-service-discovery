package ssl

import (
	"fmt"
	"github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/cesapp-lib/ssl"
)

type globalConfig interface {
	registry.ConfigurationContext
}

type cesSelfSignedSSLGenerator interface {
	// GenerateSelfSignedCert generates a self-signed certificate for the ces and returns the certificate chain and the
	// private key as string.
	GenerateSelfSignedCert(fqdn string, domain string, certExpireDays int, country string,
		province string, locality string, altDNSNames []string) (string, string, error)
}

type cesSSLWriter interface {
	// WriteCertificate writes the type, cert and key to the global config
	WriteCertificate(certType string, cert string, key string) error
}

type creator struct {
	globalConfig globalConfig
	sslGenerator cesSelfSignedSSLGenerator
	sslWriter    cesSSLWriter
}

// NewCreator generates and writes selfsigned certificates to the ces registry.
func NewCreator(globalConfig globalConfig) *creator {
	return &creator{
		globalConfig: globalConfig,
		sslGenerator: ssl.NewSSLGenerator(),
		sslWriter:    NewSSLWriter(globalConfig),
	}
}

// CreateAndSafeCertificate generates and writes the type, cert and key to the global config.
func (c *creator) CreateAndSafeCertificate(certExpireDays int, country string,
	province string, locality string, altDNSNames []string) error {

	fqdn, err := c.globalConfig.Get("fqdn")
	if err != nil {
		return fmt.Errorf("failed to get FQDN from global config: %w", err)
	}

	domain, err := c.globalConfig.Get("domain")
	if err != nil {
		return fmt.Errorf("failed to get DOMAIN from global config: %w", err)
	}

	cert, key, err := c.sslGenerator.GenerateSelfSignedCert(fqdn, domain, certExpireDays, country, province, locality, altDNSNames)
	if err != nil {
		return fmt.Errorf("failed to generate self-signed certificate and key: %w", err)
	}

	err = c.sslWriter.WriteCertificate("selfsigned", cert, key)
	if err != nil {
		return fmt.Errorf("failed to write certificate to global config: %w", err)
	}

	return nil
}
