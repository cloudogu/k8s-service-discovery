package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// sslCertificateUpdater is responsible to update the ssl certificate of the ecosystem.
type sslCertificateUpdater struct {
	Client        client.Client  `json:"client"`
	Namespace     string         `json:"namespace"`
	Endpoint      string         `json:"endpoint"`
	Configuration *Configuration `json:"configuration"`
}

// NewSslCertificateUpdater creates a new updater.
func NewSslCertificateUpdater(client client.Client, namespace string) *sslCertificateUpdater {
	endpoint := fmt.Sprintf("http://etcd.%s.svc.cluster.local:4001", namespace)
	return &sslCertificateUpdater{
		Client:    client,
		Namespace: namespace,
		Endpoint:  endpoint,
	}
}

// Start starts the update process.
func (scu sslCertificateUpdater) Start(_ context.Context) error {
	return nil
}
