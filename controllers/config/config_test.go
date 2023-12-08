package config

import (
	"context"
	_ "embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
)

//go:embed testdata/k8s_config.yaml
var configBytes []byte
var k8sConfig corev1.ConfigMap

//go:embed testdata/invalid_k8s_config.yaml
var invalidConfigBytes []byte
var invalidK8sConfig corev1.ConfigMap

func init() {
	err := yaml.Unmarshal(configBytes, &k8sConfig)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(invalidConfigBytes, &invalidK8sConfig)
	if err != nil {
		panic(err)
	}
}

func TestReadConfiguration(t *testing.T) {
	t.Run("read from file", func(t *testing.T) {
		// given
		client := fake.NewClientBuilder().Build()
		err := os.Setenv("STAGE", "local")
		require.NoError(t, err)
		defer func() {
			err := os.Unsetenv("STAGE")
			if err != nil {
				panic(err)
			}
		}()

		// when
		_, err = ReadConfiguration(context.TODO(), client, "test")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find configuration at k8s/dev-resources/k8s-ces-warp-config.yaml")
	})

	t.Run("read from cluster", func(t *testing.T) {
		// given
		client := fake.NewClientBuilder().Build()
		ctx := context.TODO()
		err := client.Create(ctx, &k8sConfig)
		require.NoError(t, err)

		// when
		config, err := ReadConfiguration(ctx, client, "test")

		// then
		require.NoError(t, err)
		assert.NotNil(t, config)
	})
}

func Test_readWarpConfigFromCluster(t *testing.T) {
	namespace := "test"
	ctx := context.TODO()
	t.Run("success", func(t *testing.T) {
		// given
		client := fake.NewClientBuilder().Build()
		k8sConfig.ResourceVersion = ""
		err := client.Create(ctx, &k8sConfig)
		require.NoError(t, err)

		// when
		config, err := readWarpConfigFromCluster(ctx, client, namespace)

		// then
		require.NoError(t, err)
		assert.NotNil(t, config)
	})
	t.Run("failed to get configmap", func(t *testing.T) {
		// given
		client := fake.NewClientBuilder().Build()

		// when
		_, err := readWarpConfigFromCluster(ctx, client, namespace)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get warp menu configmap")
	})

	t.Run("failed to unmarshal yaml from configmap", func(t *testing.T) {
		// given
		client := fake.NewClientBuilder().Build()
		err := client.Create(ctx, &invalidK8sConfig)
		require.NoError(t, err)

		// when
		_, err = readWarpConfigFromCluster(ctx, client, namespace)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to unmarshal yaml from warp config")
	})
}

func Test_readWarpConfigFromFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// when
		config, err := readWarpConfigFromFile("testdata/config.yaml")

		// then
		require.NoError(t, err)
		assert.NotNil(t, config)
	})

	t.Run("config does not exists", func(t *testing.T) {
		// when
		_, err := readWarpConfigFromFile("testdata/doesnotexists.yaml")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "could not find configuration at")
	})

	t.Run("fail because of invalid yaml", func(t *testing.T) {
		// when
		_, err := readWarpConfigFromFile("testdata/invalid_config.yaml")

		// then
		require.Error(t, err)
	})
}
