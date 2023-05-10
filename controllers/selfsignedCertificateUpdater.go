package controllers

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	sslLib "github.com/cloudogu/cesapp-lib/ssl"
	"github.com/cloudogu/k8s-service-discovery/controllers/ssl"
	etcdclient "go.etcd.io/etcd/client/v2"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	fqdnPath                  = "/config/_global/fqdn"
	serverCertificateTypePath = "certificate/type"
	selfsignedCertificateType = "selfsigned"
)

const (
	fqdnChangeEventReason = "FQDNChange"
)

// selfsignedCertificateUpdater is responsible to update the sslLib certificate of the ecosystem.
type selfsignedCertificateUpdater struct {
	client             client.Client
	namespace          string
	registry           cesRegistry
	eventRecorder      eventRecorder
	certificateCreator selfSignedCertificateCreator
}

type selfSignedCertificateCreator interface {
	CreateAndSafeCertificate(certExpireDays int, country string,
		province string, locality string, altDNSNames []string) error
}

// NewSelfsignedCertificateUpdater creates a new updater.
func NewSelfsignedCertificateUpdater(client client.Client, namespace string, cesRegistry cesRegistry, recorder eventRecorder) *selfsignedCertificateUpdater {
	return &selfsignedCertificateUpdater{
		client:             client,
		namespace:          namespace,
		registry:           cesRegistry,
		eventRecorder:      recorder,
		certificateCreator: ssl.NewCreator(cesRegistry.GlobalConfig()),
	}
}

// Start starts the update process. This update process runs indefinitely and is designed to be started as goroutine.
func (scu *selfsignedCertificateUpdater) Start(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Starting selfsigned certificate updater...")
	return scu.startEtcdWatch(ctx, scu.registry.RootConfig())
}

func (scu *selfsignedCertificateUpdater) startEtcdWatch(ctx context.Context, reg watchConfigurationContext) error {
	ctrl.LoggerFrom(ctx).Info("Start etcd watcher on fqdn")

	fqdnChannel := make(chan *etcdclient.Response)
	go func() {
		ctrl.LoggerFrom(ctx).Info("start etcd watcher for fqdn changes")
		reg.Watch(ctx, fqdnPath, false, fqdnChannel)
		ctrl.LoggerFrom(ctx).Info("stop etcd watcher for fqdn changes")
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-fqdnChannel:
			ctrl.Log.Info(fmt.Sprintf("Context: [%+v]", ctx))
			err := scu.handleFqdnChange(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (scu *selfsignedCertificateUpdater) handleFqdnChange(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("FQDN or domain changed in registry. Checking for selfsigned certificate...")
	certificateType, err := scu.registry.GlobalConfig().Get(serverCertificateTypePath)
	if err != nil {
		return fmt.Errorf("could get certificate type from registry: %w", err)
	}

	if certificateType == selfsignedCertificateType {
		ctrl.LoggerFrom(ctx).Info("Certificate is selfsigned. Regenerating certificate...")

		deployment := &appsv1.Deployment{}
		err = scu.client.Get(ctx, types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: scu.namespace}, deployment)
		if err != nil {
			return fmt.Errorf("selfsigned certificate handling: failed to get deployment [%s]: %w", "k8s-service-discovery-controller-manager", err)
		}

		previousCertRaw, err := scu.registry.GlobalConfig().Get(serverCertificateID)
		if err != nil {
			return fmt.Errorf("failed to get previous certificate from global config: %w", err)
		}

		block, _ := pem.Decode([]byte(previousCertRaw))
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

		err = scu.certificateCreator.CreateAndSafeCertificate(int(expireDays), country, province, locality, altDnsNames)
		if err != nil {
			return fmt.Errorf("failed to regenerate and safe selfsigned certificate: %w", err)
		}

		scu.eventRecorder.Event(deployment, v1.EventTypeNormal, fqdnChangeEventReason, "Selfsigned certificate regenerated.")
	}

	return nil
}

func getFirstOrDefault(items []string, defaultValue string) string {
	if len(items) > 0 {
		return items[0]
	}

	return defaultValue
}
