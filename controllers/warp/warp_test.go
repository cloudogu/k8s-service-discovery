package warp

import (
	"context"
	_ "embed"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	etcdclient "go.etcd.io/etcd/client/v2"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		mockRegistry := newMockCesRegistry(t)
		watchRegistry := newMockWatchConfigurationContext(t)
		mockRegistry.EXPECT().RootConfig().Return(watchRegistry)
		err = os.Unsetenv("STAGE")
		require.NoError(t, err)

		// when
		watcher, err := NewWatcher(ctx, client, mockRegistry, namespace, newMockEventRecorder(t))

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
		_, err = NewWatcher(context.TODO(), client, nil, "test", newMockEventRecorder(t))

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
		k8sConfig.ResourceVersion = ""
		namespace := "test"
		deployment := &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		client := fake.NewClientBuilder().WithObjects(&k8sConfig, &menuConfigMap, deployment).Build()
		err := os.Unsetenv("STAGE")
		require.NoError(t, err)

		// prepare mocks
		mockRegistry := newMockCesRegistry(t)
		watchRegistry := newMockWatchConfigurationContext(t)
		watchEvent := &etcdclient.Response{}
		mockRegistry.EXPECT().RootConfig().Return(watchRegistry)
		watchRegistry.EXPECT().Watch(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(_ context.Context, _ string, _ bool, eventChannel chan *etcdclient.Response) {
			eventChannel <- watchEvent
		}).Times(5)

		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Event(mock.IsType(&v1.Deployment{}), "Normal", "WarpMenu", "Warp menu updated.")

		watcher, err := NewWatcher(ctx, client, mockRegistry, namespace, recorderMock)
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

		readerMock := NewMockReader(t)
		readerMock.EXPECT().Read(mock.Anything).Return(expectedCategories, nil)
		watcher.ConfigReader = readerMock

		// when
		watcher.Run(ctx)

		// then
		menuCm := &corev1.ConfigMap{}
		err = client.Get(ctx, client2.ObjectKey{Name: "k8s-ces-menu-json", Namespace: "test"}, menuCm)
		require.NoError(t, err)
		assert.Equal(t, expectedMenuJSON, menuCm.Data["menu.json"])
	})
}
