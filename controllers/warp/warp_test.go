package warp

import (
	"context"
	_ "embed"
	cesmocks "github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/mocks"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	coreosclient "github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"os"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
	"testing"
	"time"
)

//go:embed testdata/k8s_config.yaml
var configBytes []byte
var k8sConfig corev1.ConfigMap

//go:embed testdata/k8s_menu_cm.yaml
var menuConfigMapBytes []byte
var menuConfigMap corev1.ConfigMap

func init() {
	err := yaml.Unmarshal(configBytes, &k8sConfig)
	if err != nil {
		panic(err)
	}

	err = yaml.Unmarshal(menuConfigMapBytes, &menuConfigMap)
	if err != nil {
		panic(err)
	}
}

func TestNewWatcher(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// given
		ctx := context.TODO()
		client := fake.NewClientBuilder().Build()
		err := client.Create(ctx, &k8sConfig)
		require.NoError(t, err)
		namespace := "test"
		mockRegistry := &cesmocks.Registry{}
		watchRegistry := &cesmocks.WatchConfigurationContext{}
		mockRegistry.On("RootConfig").Return(watchRegistry)
		err = os.Unsetenv("STAGE")
		require.NoError(t, err)

		// when
		watcher, err := NewWatcher(ctx, client, mockRegistry, namespace)

		// then
		require.NoError(t, err)
		assert.NotNil(t, watcher)
	})

	t.Run("fail to create configuration", func(t *testing.T) {
		// given
		err := os.Setenv("STAGE", "development")
		require.NoError(t, err)
		defer func() {
			err := os.Unsetenv("STAGE")
			if err != nil {
				panic(err)
			}
		}()
		client := fake.NewClientBuilder().Build()

		// when
		_, err = NewWatcher(context.TODO(), client, nil, "test")

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to Read configuration")
	})
}

func TestWatcher_Run(t *testing.T) {
	t.Run("success with 3 sources and one refresh", func(t *testing.T) {
		// given
		// prepare deadline for watch
		timeout := time.Second * 2
		ctx, cancel := context.WithTimeout(context.TODO(), timeout)
		timer := time.NewTimer(timeout)
		go func() {
			if <-timer.C; true {
				cancel()
			}
		}()

		// create the config with 3 sources and an empty menu json configmap
		client := fake.NewClientBuilder().Build()
		k8sConfig.ResourceVersion = ""
		err := client.Create(ctx, &k8sConfig)
		require.NoError(t, err)
		err = client.Create(ctx, &menuConfigMap)
		require.NoError(t, err)
		err = os.Unsetenv("STAGE")
		require.NoError(t, err)

		// prepare mocks
		namespace := "test"
		mockRegistry := &cesmocks.Registry{}
		watchRegistry := &cesmocks.WatchConfigurationContext{}
		watchEvent := &coreosclient.Response{}
		mockRegistry.On("RootConfig").Return(watchRegistry)
		watchRegistry.On("Watch", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
			warpChannel := args.Get(3).(chan *coreosclient.Response)
			warpChannel <- watchEvent
		}).Times(3)

		watcher, err := NewWatcher(ctx, client, mockRegistry, namespace)
		require.NoError(t, err)

		// prepare result categories
		expectedEntry := types.Entry{
			DisplayName: "Redmine",
			Href:        "/redmine",
			Title:       "Redmine",
			Target:      types.TARGET_SELF,
		}
		expectedEntries := types.Entries{expectedEntry}
		expectedCategory := &types.Category{
			Title:   "Development Apps",
			Order:   100,
			Entries: expectedEntries,
		}
		expectedCategories := types.Categories{expectedCategory}
		expectedMenuJSON := "[{\"Title\":\"Development Apps\",\"Order\":100,\"Entries\":[{\"DisplayName\":\"Redmine\",\"Href\":\"/redmine\",\"Title\":\"Redmine\",\"Target\":\"self\"}]}]"

		readerMock := &mocks.Reader{}
		readerMock.On("Read", mock.Anything).Return(expectedCategories, nil)
		watcher.ConfigReader = readerMock

		// when
		watcher.Run(ctx)

		// then
		mock.AssertExpectationsForObjects(t, mockRegistry, watchRegistry, readerMock)
		menuCm := &corev1.ConfigMap{}
		err = client.Get(ctx, client2.ObjectKey{Name: "k8s-ces-menu-json", Namespace: "test"}, menuCm)
		require.NoError(t, err)
		assert.Equal(t, expectedMenuJSON, menuCm.Data["menu.json"])
	})
}
