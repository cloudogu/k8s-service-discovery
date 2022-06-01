package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudogu/cesapp-lib/core"
	"github.com/cloudogu/cesapp-lib/registry"
	coreosclient "github.com/coreos/etcd/client"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	serverCertificatePath  = "/config/_global/certificate"
	serverCertificateID    = "certificate/server.crt"
	serverCertificateKeyID = "certificate/server.key"
	certificateSecretName  = "ecosystem-certificate"
)

// sslCertificateUpdater is responsible to update the ssl certificate of the ecosystem.
type sslCertificateUpdater struct {
	client    client.Client
	namespace string
	registry  registry.Registry
}

// NewSslCertificateUpdater creates a new updater.
func NewSslCertificateUpdater(client client.Client, namespace string) (*sslCertificateUpdater, error) {
	endpoint := fmt.Sprintf("http://etcd.%s.svc.cluster.local:4001", namespace)
	reg, err := registry.New(core.Registry{
		Type:      "etcd",
		Endpoints: []string{endpoint},
	})
	if err != nil {
		return nil, err
	}

	return &sslCertificateUpdater{
		client:    client,
		namespace: namespace,
		registry:  reg,
	}, nil
}

// Start starts the update process. This update process runs indefinitely and is designed to be started as goroutine.
func (scu sslCertificateUpdater) Start(ctx context.Context) error {
	logger := ctrl.LoggerFrom(ctx)
	logger.Info("Starting ssl updater...")
	return scu.startEtcdWatch(ctx, scu.registry.RootConfig())
}

func (scu *sslCertificateUpdater) startEtcdWatch(ctx context.Context, reg registry.WatchConfigurationContext) error {
	ctrl.LoggerFrom(ctx).Info("Start etcd watcher on certificate keys")

	warpChannel := make(chan *coreosclient.Response)
	go reg.Watch(serverCertificatePath, true, warpChannel)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-warpChannel:
			err := scu.handleSslChange(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (scu *sslCertificateUpdater) handleSslChange(ctx context.Context) error {
	ctrl.LoggerFrom(ctx).Info("Certificate key changed in registry. Refresh ssl certificate secret...")

	cert, key, err := scu.readCertificateFromRegistry()
	if err != nil && isEtcdKeyNotFoundError(err) {
		message := fmt.Sprintf("The etcd keys [%s/server.crt] and [%s/server.key] are required but not set in the etcd.", serverCertificatePath, serverCertificatePath)
		ctrl.LoggerFrom(ctx).Error(fmt.Errorf("%w", err), fmt.Sprintf("%s %s", message, "Writing an event..."))
		return nil
	} else if err != nil {
		return err
	}

	sslSecret, ok, err := scu.getSslSecret(ctx)
	if err != nil {
		return err
	}

	if ok {
		ctrl.LoggerFrom(ctx).Info("Found old ssl secret. Deleting it before recreation...")
		err = scu.deleteSslSecret(ctx, sslSecret)
		if err != nil {
			return fmt.Errorf("failed to create ssl secret: %w", err)
		}
	}

	ctrl.LoggerFrom(ctx).Info("Creating new ssl secret...")
	err = scu.createSslSecret(ctx, cert, key)
	if err != nil {
		return fmt.Errorf("failed to create ssl secret: %w", err)
	}

	return nil
}

func (scu *sslCertificateUpdater) readCertificateFromRegistry() (string, string, error) {
	cert, err := scu.registry.GlobalConfig().Get(serverCertificateID)
	if err != nil {
		return "", "", fmt.Errorf("failed to read the ssl certificate from the registry: %w", err)
	}

	key, err := scu.registry.GlobalConfig().Get(serverCertificateKeyID)
	if err != nil {
		return "", "", fmt.Errorf("failed to read the ssl certificate key from the registry: %w", err)
	}

	return cert, key, nil
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
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      certificateSecretName,
			Namespace: scu.namespace,
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

func (scu *sslCertificateUpdater) deleteSslSecret(ctx context.Context, secret *v1.Secret) error {
	err := scu.client.Delete(ctx, secret)
	if err != nil {
		return fmt.Errorf("failed to delete secret [%s/%s]: %w", scu.namespace, certificateSecretName, err)
	}

	return nil
}

func isEtcdKeyNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "Key not found")
}
