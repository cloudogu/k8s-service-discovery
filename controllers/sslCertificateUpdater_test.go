package controllers

import (
	"context"
	"fmt"
	doguv1 "github.com/cloudogu/k8s-dogu-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	etcdclient "go.etcd.io/etcd/client/v2"

	"github.com/stretchr/testify/mock"

	"github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	t.Run("run start without change and send done to context", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		watchContextMock := newMockWatchConfigurationContext(t)
		regMock.EXPECT().RootConfig().Return(watchContextMock)
		watchContextMock.EXPECT().Watch(mock.Anything, "/config/_global/certificate", true, mock.Anything).Return()

		recorderMock := newMockEventRecorder(t)

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:        clientMock,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: recorderMock,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*50)

		// when
		err := sslUpdater.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
	})

	t.Run("run start and send change event", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		watchContextMock := newMockWatchConfigurationContext(t)
		watchContextMock.EXPECT().Watch(mock.Anything, "/config/_global/certificate", true, mock.Anything).Run(func(_ context.Context, _ string, _ bool, eventChannel chan *etcdclient.Response) {
			testResponse := &etcdclient.Response{}
			eventChannel <- testResponse
		}).Return()
		regMock.EXPECT().RootConfig().Return(watchContextMock)

		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("mykey", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		recorderMock := newMockEventRecorder(t)
		recorderMock.On("Event", mock.IsType(&appsv1.Deployment{}), "Normal", "Certificate", "SSL secret created.")

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:        clientMock,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: recorderMock,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sslUpdater.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)

		sslSecret := &v1.Secret{}
		objectKey := types.NamespacedName{Namespace: namespace, Name: certificateSecretName}
		err = clientMock.Get(ctx, objectKey, sslSecret)
		require.NoError(t, err)

		assert.Equal(t, "mycert", sslSecret.StringData[v1.TLSCertKey])
		assert.Equal(t, "mykey", sslSecret.StringData[v1.TLSPrivateKeyKey])
	})

	t.Run("run start and get error on ssl change method", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		watchContextMock := newMockWatchConfigurationContext(t)
		watchContextMock.EXPECT().Watch(mock.Anything, "/config/_global/certificate", true, mock.Anything).Run(func(_ context.Context, _ string, _ bool, eventChannel chan *etcdclient.Response) {
			testResponse := &etcdclient.Response{}
			eventChannel <- testResponse
		}).Return()
		regMock.EXPECT().RootConfig().Return(watchContextMock)

		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("", assert.AnError)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:    clientMock,
			namespace: namespace,
			registry:  regMock,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sslUpdater.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err, assert.AnError)
	})
}

func Test_sslCertificateUpdater_handleSslChange(t *testing.T) {
	t.Run("error on retrieving server cert", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("", assert.AnError)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:    clientMock,
			namespace: namespace,
			registry:  regMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err, assert.AnError)
	})

	t.Run("error on retrieving server key", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("", assert.AnError)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:    clientMock,
			namespace: namespace,
			registry:  regMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err, assert.AnError)
	})

	t.Run("key not found on retrieving server key result in no error", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("", fmt.Errorf("error: Key not found"))
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:    clientMock,
			namespace: namespace,
			registry:  regMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.NoError(t, err)
	})

	t.Run("should return error if the deployment does not exist", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("mykey", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:    clientMock,
			namespace: namespace,
			registry:  regMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "ssl handling: failed to get deployment")
	})

	t.Run("successfully handle ssl change with existing ssl secret", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("mykey", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

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
			client:        clientMock,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: recorderMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.NoError(t, err)

		sslSecret := &v1.Secret{}
		objectKey := types.NamespacedName{Namespace: namespace, Name: certificateSecretName}
		err = clientMock.Get(context.Background(), objectKey, sslSecret)
		require.NoError(t, err)

		assert.Equal(t, "mycert", sslSecret.StringData[v1.TLSCertKey])
		assert.Equal(t, "mykey", sslSecret.StringData[v1.TLSPrivateKeyKey])
	})

	t.Run("successfully handle ssl change", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("mykey", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		recorderMock := newMockEventRecorder(t)
		recorderMock.On("Event", mock.IsType(&appsv1.Deployment{}), "Normal", "Certificate", "SSL secret created.")

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sslUpdater := &sslCertificateUpdater{
			client:        clientMock,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: recorderMock,
		}

		// when
		err := sslUpdater.handleSslChange(context.Background())

		// then
		require.NoError(t, err)

		sslSecret := &v1.Secret{}
		objectKey := types.NamespacedName{Namespace: namespace, Name: certificateSecretName}
		err = clientMock.Get(context.Background(), objectKey, sslSecret)
		require.NoError(t, err)

		assert.Equal(t, "mycert", sslSecret.StringData[v1.TLSCertKey])
		assert.Equal(t, "mykey", sslSecret.StringData[v1.TLSPrivateKeyKey])
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
