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
	"github.com/cloudogu/k8s-service-discovery/controllers/ssl"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	globalFqdnPath            = "fqdn"
	serverCertificateTypePath = "certificate/type"
	selfsignedCertificateType = "selfsigned"
	ecosystemCertificateName  = "ecosystem-certificate"
)

// selfsignedCertificateUpdater is responsible to update the sslLib certificate of the ecosystem.
type selfsignedCertificateUpdater struct {
	namespace          string
	globalConfigRepo   GlobalConfigRepository
	certificateCreator selfSignedCertificateCreator
	secretClient       SecretClient
}

type selfSignedCertificateCreator interface {
	CreateAndSafeCertificate(ctx context.Context, certExpireDays int, country string,
		province string, locality string, altDNSNames []string) error
}

// NewSelfsignedCertificateUpdater creates a new updater.
func NewSelfsignedCertificateUpdater(namespace string, globalConfigRepo GlobalConfigRepository, secretClient SecretClient) *selfsignedCertificateUpdater {
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
	return scu.startGlobalConfigWatch(ctx)
}

func (scu *selfsignedCertificateUpdater) startGlobalConfigWatch(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("start global config watcher for ssl certificates")
	fqdnChannel, err := scu.globalConfigRepo.Watch(ctx, config.KeyFilter(globalFqdnPath))
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
	ctrl.LoggerFrom(ctx).Info("FQDN or domain changed in registry. Checking for selfsigned certificate...")
	secret, err := scu.secretClient.Get(ctx, ecosystemCertificateName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret for ssl read: %w", err)
	}

	globalConfig, err := scu.globalConfigRepo.Get(ctx)
	if err != nil {
		return fmt.Errorf("failed to get global config for ssl read: %w", err)
	}

	certType, typeExists := globalConfig.Get(serverCertificateTypePath)
	if !typeExists || !util.ContainsChars(certType.String()) {
		return fmt.Errorf("%q is empty or doesn't exists: %w", serverCertificateTypePath, err)
	}

	if certType == selfsignedCertificateType {
		ctrl.LoggerFrom(ctx).Info("Certificate is selfsigned. Regenerating certificate...")

		certificateBytes, exists := secret.Data[v1.TLSCertKey]
		if !exists || string(certificateBytes) == "" {
			return fmt.Errorf("could not find certificate in ecosystem certificate secret")
		}

		block, _ := pem.Decode(certificateBytes)
		if block == nil {
			return fmt.Errorf("failed to parse certificate PEM of previous certificate")
		}

		previousCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse previous certificate: %w", err)
		}

		expireDays := previousCert.NotAfter.Sub(previousCert.NotBefore).Hours() / 24
		country := getFirstOrDefault(previousCert.Subject.Country, sslLib.Country)
		province := getFirstOrDefault(previousCert.Subject.Province, sslLib.Province)
		locality := getFirstOrDefault(previousCert.Subject.Locality, sslLib.Locality)
		altDnsNames := previousCert.DNSNames

		err = scu.certificateCreator.CreateAndSafeCertificate(ctx, int(expireDays), country, province, locality, altDnsNames)
		if err != nil {
			return fmt.Errorf("failed to regenerate and safe selfsigned certificate: %w", err)
		}

		ctrl.LoggerFrom(ctx).Info("Selfsigned certificate regenerated.")
	}

	return nil
}

func getFirstOrDefault(items []string, defaultValue string) string {
	if len(items) > 0 {
		return items[0]
	}

	return defaultValue
}
