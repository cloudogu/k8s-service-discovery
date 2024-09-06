package warp

import (
	"context"
	_ "embed"
	"github.com/cloudogu/k8s-registry-lib/dogu"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/controllers/config"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	types2 "k8s.io/apimachinery/pkg/types"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
		clientMock := fake.NewClientBuilder().Build()
		err := clientMock.Create(ctx, &k8sConfig)
		require.NoError(t, err)
		namespace := "test"
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		versionRegistryMock := NewMockDoguVersionRegistry(t)
		doguSpecRepoMock := NewMockLocalDoguRepo(t)
		err = os.Unsetenv("STAGE")
		require.NoError(t, err)

		// when
		watcher, err := NewWatcher(ctx, clientMock, versionRegistryMock, doguSpecRepoMock, namespace, newMockEventRecorder(t), mockGlobalConfigRepo)

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
		clientMock := fake.NewClientBuilder().Build()

		// when
		_, err = NewWatcher(context.TODO(), clientMock, nil, nil, "test", nil, nil)

		// then
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to Read configuration")
	})
}

var testNamespace = "test"

func TestWatcher_Run(t *testing.T) {
	t.Run("success of the initial run", func(t *testing.T) {
		// given
		testConfiguration := &config.Configuration{}
		k8sClientMock := newMockK8sClient(t)
		configReaderMock := NewMockReader(t)
		eventRecorderMock := newMockEventRecorder(t)
		k8sClientMock.EXPECT().Get(testCtx, types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(nil)
		categories := types.Categories{
			{
				Title: "Administration Apps",
				Order: 100,
				Entries: types.Entries{
					{
						DisplayName: "Admin",
						Href:        "/admin",
						Title:       "Admin",
						Target:      1,
					},
				},
			},
		}
		configReaderMock.EXPECT().Read(testCtx, testConfiguration).Return(categories, nil)
		k8sClientMock.EXPECT().Get(testCtx, client.ObjectKey{Name: config.MenuConfigMap, Namespace: testNamespace}, mock.Anything).RunAndReturn(func(ctx context.Context, name types2.NamespacedName, object client.Object, option ...client.GetOption) error {
			menuJsonCm := object.(*corev1.ConfigMap)
			menuJsonCm.Data = map[string]string{}
			return nil
		})
		expectedMenuJsonCm := &corev1.ConfigMap{Data: map[string]string{"menu.json": "[{\"Title\":\"Administration Apps\",\"Order\":100,\"Entries\":[{\"DisplayName\":\"Admin\",\"Href\":\"/admin\",\"Title\":\"Admin\",\"Target\":\"self\"}]}]"}}
		k8sClientMock.EXPECT().Update(testCtx, expectedMenuJsonCm).Return(nil)
		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, "WarpMenu", "Warp menu updated.")

		sut := Watcher{
			k8sClient:     k8sClientMock,
			ConfigReader:  configReaderMock,
			configuration: testConfiguration,
			namespace:     testNamespace,
			eventRecorder: eventRecorderMock,
		}

		// when
		err := sut.Run(testCtx)

		// then
		require.NoError(t, err)
	})

	t.Run("should log error in initial run", func(t *testing.T) {
		// given
		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(testCtx, types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(assert.AnError)

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Error(mock.Anything, "error creating warp-menu")

		sut := Watcher{
			k8sClient:     k8sClientMock,
			namespace:     testNamespace,
			configuration: &config.Configuration{},
		}

		// when
		err := sut.Run(testCtx)

		// then
		require.NoError(t, err)
	})

	t.Run("success with a dogu source", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		testConfiguration := &config.Configuration{
			Sources: []config.Source{
				{
					Path: "/dogu",
					Type: "dogus",
					Tag:  "warp",
				},
			},
		}
		k8sClientMock := newMockK8sClient(t)
		configReaderMock := NewMockReader(t)
		eventRecorderMock := newMockEventRecorder(t)
		versionRegistryMock := NewMockDoguVersionRegistry(t)
		k8sClientMock.EXPECT().Get(cancelCtx, types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(nil)
		categories := types.Categories{
			{
				Title: "Administration Apps",
				Order: 100,
				Entries: types.Entries{
					{
						DisplayName: "Admin",
						Href:        "/admin",
						Title:       "Admin",
						Target:      1,
					},
				},
			},
		}
		configReaderMock.EXPECT().Read(cancelCtx, testConfiguration).Return(categories, nil)
		k8sClientMock.EXPECT().Get(cancelCtx, client.ObjectKey{Name: config.MenuConfigMap, Namespace: testNamespace}, mock.Anything).RunAndReturn(func(ctx context.Context, name types2.NamespacedName, object client.Object, option ...client.GetOption) error {
			menuJsonCm := object.(*corev1.ConfigMap)
			menuJsonCm.Data = map[string]string{}
			return nil
		})
		expectedMenuJsonCm := &corev1.ConfigMap{Data: map[string]string{"menu.json": "[{\"Title\":\"Administration Apps\",\"Order\":100,\"Entries\":[{\"DisplayName\":\"Admin\",\"Href\":\"/admin\",\"Title\":\"Admin\",\"Target\":\"self\"}]}]"}}
		k8sClientMock.EXPECT().Update(cancelCtx, expectedMenuJsonCm).Return(nil)
		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, "WarpMenu", "Warp menu updated.").Times(1)
		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, "WarpMenu", "Warp menu updated.").Times(1).Run(func(args mock.Arguments) {
			cancelFunc()
		})

		resultChannel := make(chan dogu.CurrentVersionsWatchResult)
		versionRegistryMock.EXPECT().WatchAllCurrent(cancelCtx).Return(resultChannel, nil)

		sut := Watcher{
			k8sClient:       k8sClientMock,
			ConfigReader:    configReaderMock,
			configuration:   testConfiguration,
			namespace:       testNamespace,
			eventRecorder:   eventRecorderMock,
			registryToWatch: versionRegistryMock,
		}

		// when
		err := sut.Run(cancelCtx)
		resultChannel <- dogu.CurrentVersionsWatchResult{}

		// then
		require.NoError(t, err)
		<-cancelCtx.Done()
	})

	t.Run("success with a config source", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		testConfiguration := &config.Configuration{
			Sources: []config.Source{
				{
					Path: "externals",
					Type: "externals",
				},
			},
		}
		k8sClientMock := newMockK8sClient(t)
		configReaderMock := NewMockReader(t)
		eventRecorderMock := newMockEventRecorder(t)
		globalConfigMock := NewMockGlobalConfigRepository(t)
		k8sClientMock.EXPECT().Get(cancelCtx, types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(nil)
		categories := types.Categories{
			{
				Title: "Administration Apps",
				Order: 100,
				Entries: types.Entries{
					{
						DisplayName: "Admin",
						Href:        "/admin",
						Title:       "Admin",
						Target:      1,
					},
				},
			},
		}
		configReaderMock.EXPECT().Read(cancelCtx, testConfiguration).Return(categories, nil)
		k8sClientMock.EXPECT().Get(cancelCtx, client.ObjectKey{Name: config.MenuConfigMap, Namespace: testNamespace}, mock.Anything).RunAndReturn(func(ctx context.Context, name types2.NamespacedName, object client.Object, option ...client.GetOption) error {
			menuJsonCm := object.(*corev1.ConfigMap)
			menuJsonCm.Data = map[string]string{}
			return nil
		})
		expectedMenuJsonCm := &corev1.ConfigMap{Data: map[string]string{"menu.json": "[{\"Title\":\"Administration Apps\",\"Order\":100,\"Entries\":[{\"DisplayName\":\"Admin\",\"Href\":\"/admin\",\"Title\":\"Admin\",\"Target\":\"self\"}]}]"}}
		k8sClientMock.EXPECT().Update(cancelCtx, expectedMenuJsonCm).Return(nil)
		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, "WarpMenu", "Warp menu updated.").Times(1)
		eventRecorderMock.EXPECT().Event(mock.Anything, corev1.EventTypeNormal, "WarpMenu", "Warp menu updated.").Times(1).Run(func(args mock.Arguments) {
			cancelFunc()
		})

		resultChannel := make(chan repository.GlobalConfigWatchResult)
		globalConfigMock.EXPECT().Watch(cancelCtx, mock.Anything).Return(resultChannel, nil)

		sut := Watcher{
			k8sClient:        k8sClientMock,
			ConfigReader:     configReaderMock,
			configuration:    testConfiguration,
			namespace:        testNamespace,
			eventRecorder:    eventRecorderMock,
			globalConfigRepo: globalConfigMock,
		}

		// when
		err := sut.Run(cancelCtx)
		resultChannel <- repository.GlobalConfigWatchResult{}

		// then
		require.NoError(t, err)
		<-cancelCtx.Done()
	})
}

func TestWatcher_startGlobalConfigWatch(t *testing.T) {
	t.Run("should log error and return on get watch error", func(t *testing.T) {
		// given
		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Info(0, "start global config watcher for source [/config/externals]")
		mockLogSink.EXPECT().Error(mock.Anything, "failed to create global config watch for path %q", "/config/externals")

		globalConfigMock := NewMockGlobalConfigRepository(t)
		globalConfigMock.EXPECT().Watch(mock.Anything, mock.Anything).Return(nil, assert.AnError)

		sut := Watcher{
			globalConfigRepo: globalConfigMock,
		}

		// when
		sut.startGlobalConfigDirectoryWatch(testCtx, "/config/externals")
	})
}

func TestWatcher_startVersionRegistryWatch(t *testing.T) {
	t.Run("should log error and return on get watch error", func(t *testing.T) {
		// given
		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Info(0, "start version registry watcher for source type dogu")
		mockLogSink.EXPECT().Error(mock.Anything, "failed to create dogu version registry watch")

		versionRegistryMock := NewMockDoguVersionRegistry(t)
		resultChannel := make(chan dogu.CurrentVersionsWatchResult)
		versionRegistryMock.EXPECT().WatchAllCurrent(mock.Anything).Return(resultChannel, assert.AnError)

		sut := Watcher{
			registryToWatch: versionRegistryMock,
		}

		// when
		sut.startVersionRegistryWatch(testCtx)
	})
}

func TestWatcher_handleGlobalConfigUpdates(t *testing.T) {
	t.Run("should return and log if the channel will be closed", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Info(0, "global config watch channel canceled - stop watch for warp generation").Run(func(level int, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		sut := Watcher{}
		channel := make(chan repository.GlobalConfigWatchResult)

		// when
		go func() {
			sut.handleGlobalConfigUpdates(cancelCtx, channel)
		}()
		close(channel)
		<-cancelCtx.Done()
	})

	t.Run("should continue and log error on watch result error", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Error(assert.AnError, "global config watch channel error for warp generation").Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})
		mockLogSink.EXPECT().Info(0, "context done - stop global config watch for warp generation")

		sut := Watcher{}
		channel := make(chan repository.GlobalConfigWatchResult)

		// when
		go func() {
			sut.handleGlobalConfigUpdates(cancelCtx, channel)
		}()
		channel <- repository.GlobalConfigWatchResult{Err: assert.AnError}
		<-cancelCtx.Done()
		// Wait for last log
		timer := time.NewTimer(time.Millisecond * 500)
		<-timer.C
	})

	t.Run("should return error on error executing global config update on watch event", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(cancelCtx, types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(nil)

		configReaderMock := NewMockReader(t)
		configReaderMock.EXPECT().Read(cancelCtx, mock.Anything).Return(nil, assert.AnError)

		eventRecoderMock := newMockEventRecorder(t)
		eventRecoderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeWarning, "ErrUpdateWarpMenu", "Updating warp menu failed: %w", assert.AnError)

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Error(mock.Anything, "failed to update entries from global config in warp menu").Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})
		mockLogSink.EXPECT().Info(0, "context done - stop global config watch for warp generation")

		sut := Watcher{
			namespace:     testNamespace,
			k8sClient:     k8sClientMock,
			ConfigReader:  configReaderMock,
			eventRecorder: eventRecoderMock,
		}
		channel := make(chan repository.GlobalConfigWatchResult)

		// when
		go func() {
			sut.handleGlobalConfigUpdates(cancelCtx, channel)
		}()
		channel <- repository.GlobalConfigWatchResult{}
		<-cancelCtx.Done()
	})
}

func TestWatcher_handleDoguVersionUpdates(t *testing.T) {
	t.Run("should return and log if the channel will be closed", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Info(0, "dogu version watch channel canceled - stop watch").Run(func(level int, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		sut := Watcher{}
		channel := make(chan dogu.CurrentVersionsWatchResult)

		// when
		go func() {
			sut.handleDoguVersionUpdates(cancelCtx, channel)
		}()
		close(channel)
		<-cancelCtx.Done()
		// Wait for last log
		timer := time.NewTimer(time.Millisecond * 500)
		<-timer.C
	})

	t.Run("should continue and log error on watch result error", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Error(assert.AnError, "dogu version watch channel error").Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})
		mockLogSink.EXPECT().Info(0, "context done - stop dogu version registry watch for warp generation")

		sut := Watcher{}
		channel := make(chan dogu.CurrentVersionsWatchResult)

		// when
		go func() {
			sut.handleDoguVersionUpdates(cancelCtx, channel)
		}()
		channel <- dogu.CurrentVersionsWatchResult{Err: assert.AnError}
		<-cancelCtx.Done()
		// Wait for last log
		timer := time.NewTimer(time.Millisecond * 500)
		<-timer.C
	})

	t.Run("should return error on error executing dogu version update on watch event", func(t *testing.T) {
		// given
		cancelCtx, cancelFunc := context.WithCancel(context.Background())

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(cancelCtx, types2.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: testNamespace}, mock.Anything).Return(nil)

		configReaderMock := NewMockReader(t)
		configReaderMock.EXPECT().Read(cancelCtx, mock.Anything).Return(nil, assert.AnError)

		eventRecoderMock := newMockEventRecorder(t)
		eventRecoderMock.EXPECT().Eventf(mock.Anything, corev1.EventTypeWarning, "ErrUpdateWarpMenu", "Updating warp menu failed: %w", assert.AnError)

		mockLogSink := NewMockLogSink(t)
		oldLogFn := log.FromContext
		ctrl.LoggerFrom = func(ctx context.Context, keysAndValues ...interface{}) logr.Logger {
			return logr.New(mockLogSink)
		}
		defer func() {
			ctrl.LoggerFrom = oldLogFn
		}()
		mockLogSink.EXPECT().Init(mock.Anything)
		mockLogSink.EXPECT().Enabled(mock.Anything).Return(true)
		mockLogSink.EXPECT().Error(mock.Anything, "failed to update dogus in warp menu").Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})
		mockLogSink.EXPECT().Info(0, "context done - stop dogu version registry watch for warp generation")

		sut := Watcher{
			namespace:     testNamespace,
			k8sClient:     k8sClientMock,
			ConfigReader:  configReaderMock,
			eventRecorder: eventRecoderMock,
		}
		channel := make(chan dogu.CurrentVersionsWatchResult)

		// when
		go func() {
			sut.handleDoguVersionUpdates(cancelCtx, channel)
		}()
		channel <- dogu.CurrentVersionsWatchResult{}
		<-cancelCtx.Done()
		// Wait for last log
		timer := time.NewTimer(time.Millisecond * 500)
		<-timer.C
	})
}
