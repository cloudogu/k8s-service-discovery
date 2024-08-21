package warp

import (
	"context"
	_ "embed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"
	"testing"
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
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		versionRegistryMock := NewMockDoguVersionRegistry(t)
		doguSpecRepoMock := NewMockDoguSpecRepo(t)
		err = os.Unsetenv("STAGE")
		require.NoError(t, err)

		// when
		watcher, err := NewWatcher(ctx, client, versionRegistryMock, doguSpecRepoMock, namespace, newMockEventRecorder(t), mockGlobalConfigRepo)

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
		_, err = NewWatcher(context.TODO(), client, nil, nil, "test", nil, nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to Read configuration")
	})
}

// TODO Fix this test if the watch for the config and global config is implemented.
// func TestWatcher_Run(t *testing.T) {
// 	t.Run("success with 3 sources and one refresh", func(t *testing.T) {
// 		// given
// 		// prepare deadline for watch
// 		timeout := time.Second * 2
// 		ctx, cancel := context.WithTimeout(context.TODO(), timeout)
// 		timer := time.NewTimer(timeout)
// 		go func() {
// 			if <-timer.C; true {
// 				cancel()
// 			}
// 		}()
//
// 		// create the config with 3 sources and an empty menu json configmap
// 		k8sConfig.ResourceVersion = ""
// 		namespace := "test"
// 		deployment := &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
// 		client := fake.NewClientBuilder().WithObjects(&k8sConfig, &menuConfigMap, deployment).Build()
// 		err := os.Unsetenv("STAGE")
// 		require.NoError(t, err)
//
// 		// prepare mocks
// 		watchRegistry := newMockWatchConfigurationContext(t)
// 		watchEvent := &etcdclient.Response{}
// 		watchRegistry.EXPECT().Watch(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(_ context.Context, _ string, _ bool, eventChannel chan *etcdclient.Response) {
// 			eventChannel <- watchEvent
// 		}).Times(5)
// 		versionRegistryMock := NewMockDoguVersionRegistry(t)
// 		doguSpecRepoMock := NewMockDoguSpecRepo(t)
//
// 		recorderMock := newMockEventRecorder(t)
// 		recorderMock.EXPECT().Event(mock.IsType(&v1.Deployment{}), "Normal", "WarpMenu", "Warp menu updated.")
//
// 		watcher, err := NewWatcher(ctx, client, versionRegistryMock, doguSpecRepoMock, namespace, recorderMock, watchRegistry)
// 		require.NoError(t, err)
//
// 		// prepare result categories
// 		expectedEntry := types.Entry{
// 			DisplayName: "Redmine",
// 			Href:        "/redmine",
// 			Title:       "Redmine",
// 			Target:      types.TARGET_SELF,
// 		}
// 		expectedEntries := types.Entries{expectedEntry}
// 		expectedCategory := &types.Category{
// 			Title:   "Development Apps",
// 			Order:   100,
// 			Entries: expectedEntries,
// 		}
// 		expectedCategories := types.Categories{expectedCategory}
// 		expectedMenuJSON := "[{\"Title\":\"Development Apps\",\"Order\":100,\"Entries\":[{\"DisplayName\":\"Redmine\",\"Href\":\"/redmine\",\"Title\":\"Redmine\",\"Target\":\"self\"}]}]"
//
// 		readerMock := NewMockReader(t)
// 		readerMock.EXPECT().Read(testCtx, mock.Anything).Return(expectedCategories, nil)
// 		watcher.ConfigReader = readerMock
//
// 		// when
// 		watcher.Run(ctx)
//
// 		// then
// 		menuCm := &corev1.ConfigMap{}
// 		err = client.Get(ctx, client2.ObjectKey{Name: "k8s-ces-menu-json", Namespace: "test"}, menuCm)
// 		require.NoError(t, err)
// 		assert.Equal(t, expectedMenuJSON, menuCm.Data["menu.json"])
// 	})
// }
