package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	traefikv1alpha1 "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMigrationCleanupManager_Start(t *testing.T) {
	tests := []struct {
		name    string
		client  client.Client
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:   "should fail",
			client: fake.NewClientBuilder().Build(),
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to cleanup old objects from version 6.0.2 and earlier", i...)
			},
		},
		{
			name: "should succeed",
			client: fake.NewClientBuilder().WithScheme(getScheme(t)).WithObjects(
				&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}},
				&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}},
				&traefikv1alpha1.Middleware{ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: testNamespace}},
			).Build(),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MigrationCleanupManager{
				Client:    tt.client,
				Namespace: testNamespace,
			}
			tt.wantErr(t, m.Start(t.Context()))
		})
	}
}
