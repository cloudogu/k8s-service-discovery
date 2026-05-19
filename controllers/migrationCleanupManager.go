package controllers

import (
	"context"
	"errors"
	"fmt"

	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var oldServiceDiscoveryLabels = labels.Set{
	"app":                    "ces",
	"app.kubernetes.io/name": "k8s-service-discovery",
}

type MigrationCleanupManager struct {
	Client    client.Client
	Namespace string
}

// Start cleans up old objects when migrating to a new version.
func (m *MigrationCleanupManager) Start(ctx context.Context) error {
	err := m.migrateFrom6_0_2(ctx)
	if err != nil {
		return fmt.Errorf("failed to cleanup old objects from version 6.0.2 and earlier: %w", err)
	}

	return nil
}

// migrate from 6.0.2 and earlier versions
func (m *MigrationCleanupManager) migrateFrom6_0_2(ctx context.Context) error {
	var errs []error
	selector := labels.SelectorFromSet(oldServiceDiscoveryLabels)
	err := m.Client.DeleteAllOf(ctx, &networkingv1.Ingress{}, &client.DeleteAllOfOptions{ListOptions: client.ListOptions{LabelSelector: selector, Namespace: m.Namespace}})
	errs = append(errs, err)
	err = m.Client.DeleteAllOf(ctx, &traefikv1alpha1.Middleware{}, &client.DeleteAllOfOptions{ListOptions: client.ListOptions{LabelSelector: selector, Namespace: m.Namespace}})
	errs = append(errs, err)
	err = m.Client.DeleteAllOf(ctx, &networkingv1.NetworkPolicy{}, &client.DeleteAllOfOptions{ListOptions: client.ListOptions{LabelSelector: selector, Namespace: m.Namespace}})
	errs = append(errs, err)

	err = errors.Join(errs...)
	return err
}
