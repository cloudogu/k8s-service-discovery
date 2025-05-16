package ssl

import (
	"context"
	"fmt"
	"github.com/cloudogu/cesapp-lib/ssl"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
)

type cesSelfSignedSSLGenerator interface {
	// GenerateSelfSignedCert generates a self-signed certificate for the ces and returns the certificate chain and the
	// private key as string.
	GenerateSelfSignedCert(fqdn string, domain string, certExpireDays int, country string,
		province string, locality string, altDNSNames []string) (string, string, error)
}

type cesSSLWriter interface {
	// WriteCertificate writes the type, cert and key to the ecosystem-certificate secret
	WriteCertificate(ctx context.Context, cert string, key string) error
}

type creator struct {
	globalConfigRepo GlobalConfigRepository
	sslGenerator     cesSelfSignedSSLGenerator
	sslWriter        cesSSLWriter
}

// NewCreator generates and writes selfsigned certificates to the ecosystem-certificate secret.
func NewCreator(globalConfigRepo GlobalConfigRepository, secretClient SecretClient) *creator {
	return &creator{
		globalConfigRepo: globalConfigRepo,
		sslGenerator:     ssl.NewSSLGenerator(),
		sslWriter:        NewSSLWriter(secretClient),
	}
}

// CreateAndSafeCertificate generates and writes the cert and key to the ecosystem-certificate secret.
func (c *creator) CreateAndSafeCertificate(ctx context.Context, certExpireDays int, country string,
	province string, locality string, altDNSNames []string) error {
	globalConfig, err := c.globalConfigRepo.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get global config for ssl creation: %w", err)
	}

	fqdn, exists := globalConfig.Get("fqdn")
	if !exists || !util.ContainsChars(fqdn.String()) {
		return fmt.Errorf("fqdn is empty or doesn't exists")
	}

	domain, exists := globalConfig.Get("domain")
	if !exists || !util.ContainsChars(domain.String()) {
		return fmt.Errorf("domain is empty or doesn't exists: %w", err)
	}

	cert, key, err := c.sslGenerator.GenerateSelfSignedCert(fqdn.String(), domain.String(), certExpireDays, country, province, locality, altDNSNames)
	if err != nil {
		return fmt.Errorf("failed to generate self-signed certificate and key: %w", err)
	}

	err = c.sslWriter.WriteCertificate(ctx, cert, key)
	if err != nil {
		return fmt.Errorf("failed to write certificate to the ecosystem-certificate secret: %w", err)
	}

	return nil
}
