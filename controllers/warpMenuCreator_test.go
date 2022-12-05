package controllers

import (
	"github.com/cloudogu/k8s-service-discovery/controllers/mocks"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

func TestNewWarpMenuCreator(t *testing.T) {
	// given
	client := fake.NewClientBuilder().Build()

	// when
	underTest := NewWarpMenuCreator(client, nil, "test", mocks.NewEventRecorder(t))

	// then
	require.NotNil(t, underTest)
}
