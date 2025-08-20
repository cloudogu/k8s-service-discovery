package controllers

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sslLib "github.com/cloudogu/cesapp-lib/ssl"
	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/ssl"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	globalFqdnPath            = "fqdn"
	globalDomainPath          = "domain"
	alternativeFQDNsPath      = "alternativeFQDNs"
	serverCertificateTypePath = "certificate/type"
	selfsignedCertificateType = "selfsigned"
	ecosystemCertificateName  = "ecosystem-certificate"
)

// selfsignedCertificateUpdater is responsible to update the sslLib certificate of the ecosystem.
type selfsignedCertificateUpdater struct {
	namespace          string
	globalConfigRepo   GlobalConfigRepository
	certificateCreator selfSignedCertificateCreator
	secretClient       secretClient
}

type selfSignedCertificateCreator interface {
	CreateAndSafeCertificate(ctx context.Context, certExpireDays int, country string,
		province string, locality string, altDNSNames []string) error
}

// NewSelfsignedCertificateUpdater creates a new updater.
func NewSelfsignedCertificateUpdater(namespace string, globalConfigRepo GlobalConfigRepository, secretClient secretClient) *selfsignedCertificateUpdater {
	return &selfsignedCertificateUpdater{
		namespace:          namespace,
		globalConfigRepo:   globalConfigRepo,
		certificateCreator: ssl.NewCreator(globalConfigRepo, secretClient),
		secretClient:       secretClient,
	}
}

// Start starts the update process. This update process runs indefinitely and is designed to be started as goroutine.
func (scu *selfsignedCertificateUpdater) Start(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Starting selfsigned certificate updater...")

	shouldUpdate, err := scu.shouldUpdateCurrentCertificate(ctx)
	if err != nil {
		logger.Error(err, "failed to check if certificate should be updated")
	}

	if shouldUpdate {
		logger.Info("Certificate should be updated. Updating now...")
		if err := scu.handleFqdnChange(ctx); err != nil {
			logger.Error(err, "failed to update certificate")
		}
	}

	return scu.startGlobalConfigWatch(ctx)
}

func (scu *selfsignedCertificateUpdater) startGlobalConfigWatch(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("start global config watcher for ssl certificates")
	fqdnChannel, err := scu.globalConfigRepo.Watch(ctx, config.KeyFilter(globalFqdnPath), config.KeyFilter(alternativeFQDNsPath), config.KeyFilter(globalDomainPath))
	if err != nil {
		return fmt.Errorf("failed to create fqdn watch: %w", err)
	}

	go func() {
		scu.startFQDNWatch(ctx, fqdnChannel)
	}()

	return nil
}

func (scu *selfsignedCertificateUpdater) startFQDNWatch(ctx context.Context, fqdnWatchChannel <-chan repository.GlobalConfigWatchResult) {
	for {
		select {
		case <-ctx.Done():
			ctrl.LoggerFrom(ctx).Info("context done - stop global config watcher for fqdn changes")
			return
		case result, open := <-fqdnWatchChannel:
			if !open {
				ctrl.LoggerFrom(ctx).Info("fqdn watch channel was closed - stop watch")
				return
			}
			if result.Err != nil {
				ctrl.LoggerFrom(ctx).Error(result.Err, "fqdn watch channel error")
				continue
			}

			err := scu.handleFqdnChange(ctx)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to handle fqdn update")
			}
		}
	}
}

func (scu *selfsignedCertificateUpdater) handleFqdnChange(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("FQDN, alternativeFQDNs or domain changed in registry. Checking for selfsigned certificate...")

	globalConfig, err := scu.globalConfigRepo.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get global config for ssl read: %w", err)
	}

	isSelfSignedCertificate, err := scu.isSelfSignedCertificate(globalConfig)
	if err != nil {
		return fmt.Errorf("failed to check certificate-type: %w", err)
	}

	if isSelfSignedCertificate {
		ctrl.LoggerFrom(ctx).Info("Certificate is selfsigned. Regenerating certificate...")

		previousCert, err := scu.getCurrentCertificate(ctx)
		if err != nil {
			return fmt.Errorf("failed to get current certificate: %w", err)
		}

		expireDays := previousCert.NotAfter.Sub(previousCert.NotBefore).Hours() / 24
		country := getFirstOrDefault(previousCert.Subject.Country, sslLib.Country)
		province := getFirstOrDefault(previousCert.Subject.Province, sslLib.Province)
		locality := getFirstOrDefault(previousCert.Subject.Locality, sslLib.Locality)

		altDnsNames := getAlternativeFQDNs(globalConfig)

		err = scu.certificateCreator.CreateAndSafeCertificate(ctx, int(expireDays), country, province, locality, altDnsNames)
		if err != nil {
			return fmt.Errorf("failed to regenerate and safe selfsigned certificate: %w", err)
		}

		ctrl.LoggerFrom(ctx).Info("Selfsigned certificate regenerated.")
	}

	return nil
}

func (scu *selfsignedCertificateUpdater) isSelfSignedCertificate(globalConfig config.GlobalConfig) (bool, error) {
	certType, typeExists := globalConfig.Get(serverCertificateTypePath)
	if !typeExists || !util.ContainsChars(certType.String()) {
		return false, fmt.Errorf("%q is empty or doesn't exists", serverCertificateTypePath)
	}

	return certType == selfsignedCertificateType, nil
}

func (scu *selfsignedCertificateUpdater) shouldUpdateCurrentCertificate(ctx context.Context) (bool, error) {
	globalConfig, err := scu.globalConfigRepo.Get(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get global config: %w", err)
	}

	isSelfSignedCertificate, err := scu.isSelfSignedCertificate(globalConfig)
	if err != nil {
		return false, fmt.Errorf("failed to check certificate-type: %w", err)
	}

	if !isSelfSignedCertificate {
		return false, nil
	}

	certificate, err := scu.getCurrentCertificate(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get current certificate: %w", err)
	}

	fqdn, exists := globalConfig.Get(globalFqdnPath)
	if !exists || !util.ContainsChars(fqdn.String()) {
		return false, fmt.Errorf("fqdn is empty or doesn't exist")
	}

	// check if the current certificate has the configured fqdn or alternative fqdns
	fqdns := append(getAlternativeFQDNs(globalConfig), fqdn.String())
	if !certificateHasAllDNSNames(certificate, fqdns) {
		return true, nil
	}

	domain, exists := globalConfig.Get(globalDomainPath)
	if !exists || !util.ContainsChars(fqdn.String()) {
		return false, fmt.Errorf("domain is empty or doesn't exist")
	}

	// check if the current certificate has the configured domain
	if domain.String() != getFirstOrDefault(certificate.Subject.Organization, "") {
		return true, nil
	}

	return false, nil
}

func (scu *selfsignedCertificateUpdater) getCurrentCertificate(ctx context.Context) (*x509.Certificate, error) {
	secret, err := scu.secretClient.Get(ctx, ecosystemCertificateName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret for ssl read: %w", err)
	}

	certificateBytes, exists := secret.Data[v1.TLSCertKey]
	if !exists || string(certificateBytes) == "" {
		return nil, fmt.Errorf("could not find certificate in ecosystem certificate secret")
	}

	block, _ := pem.Decode(certificateBytes)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM of previous certificate")
	}

	previousCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse previous certificate: %w", err)
	}

	return previousCert, nil
}

func getAlternativeFQDNs(globalConfig config.GlobalConfig) []string {
	altFQDNsString, exists := globalConfig.Get(alternativeFQDNsPath)
	if !exists {
		return []string{}
	}

	altFQDNs := util.ParseAlternativeFQDNsFromConfigString(altFQDNsString.String())

	// Create a slice to hold just the names
	fqdns := make([]string, 0)
	for _, a := range altFQDNs {
		if !a.HasCertificate() {
			fqdns = append(fqdns, a.FQDN)
		}
	}

	return fqdns
}

func getFirstOrDefault(items []string, defaultValue string) string {
	if len(items) > 0 {
		return items[0]
	}

	return defaultValue
}

func certificateHasAllDNSNames(certificate *x509.Certificate, dnsNames []string) bool {
	for _, dnsName := range dnsNames {
		if !certificateHasDNSName(certificate, dnsName) {
			return false
		}
	}

	return true
}

func certificateHasDNSName(certificate *x509.Certificate, dnsName string) bool {
	for _, certificateDNSName := range certificate.DNSNames {
		if dnsName == certificateDNSName {
			return true
		}
	}

	return false
}
