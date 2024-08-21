package controllers

import (
	"context"
	doguv1 "github.com/cloudogu/k8s-dogu-operator/api/v1"
	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

func getScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "k8s.cloudogu.com",
		Version: "v1",
		Kind:    "Dogu",
	}, &doguv1.Dogu{})
	return scheme
}

func Test_sslCertificateUpdater_Start(t *testing.T) {
	t.Run("should return error on error creating watch", func(t *testing.T) {
		// given
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		globalConfigRepoMock.EXPECT().Watch(testCtx, mock.Anything).Return(nil, assert.AnError)

		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
		}

		// when
		err := sslUpdater.Start(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create ssl watch")
	})

	t.Run("run start without change and send done to context", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*50)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		globalConfigRepoMock.EXPECT().Watch(ctx, mock.Anything).Return(resultChannel, nil)

		recorderMock := newMockEventRecorder(t)

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
			eventRecorder:    recorderMock,
		}

		// when
		err := sslUpdater.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
	})

	t.Run("run start with change error and send done to context", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		globalConfigRepoMock.EXPECT().Watch(ctx, mock.Anything).Return(resultChannel, nil)

		recorderMock := newMockEventRecorder(t)

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
			eventRecorder:    recorderMock,
		}

		// when
		err := sslUpdater.Start(ctx)

		resultChannel <- repository.GlobalConfigWatchResult{
			Err: assert.AnError,
		}
		cancelFunc()

		// then
		require.NoError(t, err)
	})

	t.Run("run start and close channel", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		globalConfigRepoMock.EXPECT().Watch(ctx, mock.Anything).Return(resultChannel, nil)

		recorderMock := newMockEventRecorder(t)

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
			eventRecorder:    recorderMock,
		}

		// when
		err := sslUpdater.Start(ctx)

		close(resultChannel)
		cancelFunc()

		// then
		require.NoError(t, err)
	})

	t.Run("run start and send change event", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		globalConfigRepoMock.EXPECT().Watch(ctx, mock.Anything).Return(resultChannel, nil)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/server.crt": "mycert",
			"certificate/server.key": "mykey",
		})
		globalConfigRepoMock.EXPECT().Get(ctx).Return(globalConfig, nil)

		recorderMock := newMockEventRecorder(t)
		recorderMock.On("Event", mock.IsType(&appsv1.Deployment{}), "Normal", "Certificate", "SSL secret created.")

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
			eventRecorder:    recorderMock,
		}

		// when
		err := sslUpdater.Start(ctx)
		resultChannel <- repository.GlobalConfigWatchResult{}
		timer := time.NewTimer(time.Second * 2)
		<-timer.C

		// then
		require.NoError(t, err)

		sslSecret := &v1.Secret{}
		objectKey := types.NamespacedName{Namespace: namespace, Name: certificateSecretName}
		err = clientMock.Get(ctx, objectKey, sslSecret)
		require.NoError(t, err)

		assert.Equal(t, "mycert", sslSecret.StringData[v1.TLSCertKey])
		assert.Equal(t, "mykey", sslSecret.StringData[v1.TLSPrivateKeyKey])
		cancelFunc()
	})

	t.Run("run start and get error on ssl change method", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*5)
		globalConfigRepoMock := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		globalConfigRepoMock.EXPECT().Watch(ctx, mock.Anything).Return(resultChannel, nil)
		globalConfig := config.CreateGlobalConfig(config.Entries{})
		globalConfigRepoMock.EXPECT().Get(ctx).Return(globalConfig, assert.AnError)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: globalConfigRepoMock,
		}

		// when
		err := sslUpdater.Start(ctx)
		resultChannel <- repository.GlobalConfigWatchResult{}
		timer := time.NewTimer(time.Second * 2)
		<-timer.C

		// then
		require.NoError(t, err)
		cancelFunc()
	})
}

func Test_sslCertificateUpdater_handleSslChange(t *testing.T) {
	t.Run("error on getting global config", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(config.GlobalConfig{}, assert.AnError)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get global config for ssl read")
	})

	t.Run("error on retrieving server cert (key not found)", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(config.GlobalConfig{}, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err, assert.AnError)
		assert.ErrorContains(t, err, "\"certificate/server.crt\" is empty or doesn't exists")
	})

	t.Run("error on retrieving server cert (value is empty)", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/server.crt": "",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err, assert.AnError)
		assert.ErrorContains(t, err, "\"certificate/server.crt\" is empty or doesn't exists")
	})

	t.Run("error on retrieving server key (key not found)", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/server.crt": "cert",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err, assert.AnError)
		assert.ErrorContains(t, err, "\"certificate/server.key\" is empty or doesn't exists")
	})

	t.Run("error on retrieving server key (value is empty)", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/server.crt": "cert",
			"certificate/server.key": "",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err, assert.AnError)
		assert.ErrorContains(t, err, "\"certificate/server.key\" is empty or doesn't exists")
	})

	t.Run("should return error if the deployment does not exist", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/server.crt": "cert",
			"certificate/server.key": "key",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "ssl handling: failed to get deployment")
	})

	t.Run("successfully handle ssl change with existing ssl secret", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/server.crt": "cert",
			"certificate/server.key": "key",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		recorderMock := newMockEventRecorder(t)
		recorderMock.On("Event", mock.IsType(&appsv1.Deployment{}), "Normal", "Certificate", "SSL secret changed.")

		namespace := "myTestNamespace"
		initialSslSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      certificateSecretName,
				Namespace: namespace,
			},
			StringData: map[string]string{
				v1.TLSCertKey:       "asd",
				v1.TLSPrivateKeyKey: "asdasd",
			},
			Type: v1.SecretTypeTLS,
		}

		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment, initialSslSecret).Build()
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
			eventRecorder:    recorderMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.NoError(t, err)

		sslSecret := &v1.Secret{}
		objectKey := types.NamespacedName{Namespace: namespace, Name: certificateSecretName}
		err = clientMock.Get(context.Background(), objectKey, sslSecret)
		require.NoError(t, err)

		assert.Equal(t, "cert", sslSecret.StringData[v1.TLSCertKey])
		assert.Equal(t, "key", sslSecret.StringData[v1.TLSPrivateKeyKey])
	})

	t.Run("successfully handle ssl change", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/server.crt": "cert",
			"certificate/server.key": "key",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		recorderMock := newMockEventRecorder(t)
		recorderMock.On("Event", mock.IsType(&appsv1.Deployment{}), "Normal", "Certificate", "SSL secret created.")

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:           clientMock,
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
			eventRecorder:    recorderMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.NoError(t, err)

		sslSecret := &v1.Secret{}
		objectKey := types.NamespacedName{Namespace: namespace, Name: certificateSecretName}
		err = clientMock.Get(context.Background(), objectKey, sslSecret)
		require.NoError(t, err)

		assert.Equal(t, "cert", sslSecret.StringData[v1.TLSCertKey])
		assert.Equal(t, "key", sslSecret.StringData[v1.TLSPrivateKeyKey])
	})
}

func TestNewSslCertificateUpdater(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// when
		result := NewSslCertificateUpdater(nil, "nil", nil, nil)

		// then
		require.NotNil(t, result)
	})
}
