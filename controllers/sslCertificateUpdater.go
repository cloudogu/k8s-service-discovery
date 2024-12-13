package controllers

import (
	"context"
	"fmt"
	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	globalServerCertificatePath = "certificate"
	serverCertificateID         = "certificate/server.crt"
	serverCertificateKeyID      = "certificate/server.key"
	certificateSecretName       = "ecosystem-certificate"
)

const (
	certificateChangeEventReason = "Certificate"
)

// sslCertificateUpdater is responsible to update the ssl certificate of the ecosystem.
type sslCertificateUpdater struct {
	client           client.Client
	namespace        string
	globalConfigRepo GlobalConfigRepository
	eventRecorder    eventRecorder
}

// NewSslCertificateUpdater creates a new updater.
func NewSslCertificateUpdater(client client.Client, namespace string, globalConfigRepo GlobalConfigRepository, recorder eventRecorder) *sslCertificateUpdater {
	return &sslCertificateUpdater{
		client:           client,
		namespace:        namespace,
		globalConfigRepo: globalConfigRepo,
		eventRecorder:    recorder,
	}
}

// Start starts the update process. This update process runs indefinitely and is designed to be started as goroutine.
func (scu *sslCertificateUpdater) Start(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Starting ssl updater...")
	return scu.startGlobalConfigWatch(ctx)
}

func (scu *sslCertificateUpdater) startGlobalConfigWatch(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Start global config watcher on certificate keys")

	sslWatchChannel, err := scu.globalConfigRepo.Watch(ctx, config.DirectoryFilter(globalServerCertificatePath))
	if err != nil {
		return fmt.Errorf("failed to create ssl watch: %w", err)
	}

	go func() {
		ctrl.LoggerFrom(ctx).Info("start global config watcher for ssl certificates")
		scu.startSSLWatch(ctx, sslWatchChannel)
		ctrl.LoggerFrom(ctx).Info("stop global config watcher for ssl certificates")
	}()

	return nil
}

func (scu *sslCertificateUpdater) startSSLWatch(ctx context.Context, sslWatchChannel <-chan repository.GlobalConfigWatchResult) {
	for {
		select {
		case <-ctx.Done():
			ctrl.LoggerFrom(ctx).Info("context done - stop global config watcher for ssl certificate changes")
			return
		case result, open := <-sslWatchChannel:
			if !open {
				ctrl.LoggerFrom(ctx).Info("ssl watch channel canceled - stop watch")
				return
			}
			if result.Err != nil {
				ctrl.LoggerFrom(ctx).Error(result.Err, "ssl watch channel error")
				continue
			}

			err := scu.handleSslChange(ctx)
			if err != nil {
				ctrl.LoggerFrom(ctx).Error(err, "failed to handle ssl update")
			}
		}
	}
}

func (scu *sslCertificateUpdater) handleSslChange(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Certificate key changed in registry. Refresh ssl certificate secret...")

	cert, key, err := scu.readCertificateFromRegistry(ctx)
	if err != nil {
		return fmt.Errorf("failed to read certificate: %w", err)
	}

	deployment := &appsv1.Deployment{}
	err = scu.client.Get(ctx, types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: scu.namespace}, deployment)
	if err != nil {
		return fmt.Errorf("ssl handling: failed to get deployment [%s]: %w", "k8s-service-discovery-controller-manager", err)
	}

	err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		sslSecret, ok, err := scu.getSslSecret(ctx)
		if err != nil {
			return err
		}

		if !ok {
			ctrl.LoggerFrom(ctx).Info("Creating new ssl secret...")
			err = scu.createSslSecret(ctx, cert, key)
			if err != nil {
				return fmt.Errorf("failed to create ssl secret: %w", err)
			}
			scu.eventRecorder.Event(deployment, v1.EventTypeNormal, certificateChangeEventReason, "SSL secret created.")
			return nil
		}

		sslSecret.StringData = map[string]string{
			v1.TLSCertKey:       cert,
			v1.TLSPrivateKeyKey: key,
		}

		ctrl.LoggerFrom(ctx).Info("Update ssl secret...")
		err = scu.client.Update(ctx, sslSecret)
		if err != nil {
			return fmt.Errorf("failed to update ssl secret: %w", err)
		}
		scu.eventRecorder.Event(deployment, v1.EventTypeNormal, certificateChangeEventReason, "SSL secret changed.")

		return nil
	})

	if err != nil {
		return fmt.Errorf("timout during ssl secret update: %w", err)
	}

	return nil
}

func (scu *sslCertificateUpdater) readCertificateFromRegistry(ctx context.Context) (string, string, error) {
	globalConfig, err := scu.globalConfigRepo.Get(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get global config for ssl read: %w", err)
	}

	cert, exists := globalConfig.Get(serverCertificateID)
	if !exists || !util.ContainsChars(cert.String()) {
		return "", "", fmt.Errorf("%q is empty or doesn't exists", serverCertificateID)
	}

	key, exists := globalConfig.Get(serverCertificateKeyID)
	if !exists || !util.ContainsChars(key.String()) {
		return "", "", fmt.Errorf("%q is empty or doesn't exists", serverCertificateKeyID)
	}

	return cert.String(), key.String(), nil
}

func (scu *sslCertificateUpdater) getSslSecret(ctx context.Context) (*v1.Secret, bool, error) {
	var sslSecret v1.Secret
	sslSecretID := types.NamespacedName{
		Namespace: scu.namespace,
		Name:      certificateSecretName,
	}

	err := scu.client.Get(ctx, sslSecretID, &sslSecret)
	if errors.IsNotFound(err) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, fmt.Errorf("failed to retrieve secret [%v] from cluster: %w", sslSecretID, err)
	}

	return &sslSecret, true, nil
}

func (scu *sslCertificateUpdater) createSslSecret(ctx context.Context, cert string, key string) error {
	sslSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificateSecretName,
			Namespace: scu.namespace,
			Labels:    util.K8sCesServiceDiscoveryLabels,
		},
		StringData: map[string]string{
			v1.TLSCertKey:       cert,
			v1.TLSPrivateKeyKey: key,
		},
		Type: v1.SecretTypeTLS,
	}

	err := scu.client.Create(ctx, sslSecret)
	if err != nil {
		return fmt.Errorf("failed to create secret [%s/%s]: %w", scu.namespace, certificateSecretName, err)
	}

	return nil
}
