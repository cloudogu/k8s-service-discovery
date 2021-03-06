package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	coreosclient "github.com/coreos/etcd/client"

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
	return scheme
}

func Test_sslCertificateUpdater_Start(t *testing.T) {
	t.Run("run start and send done to context", func(t *testing.T) {
		// given
		regMock := &mocks.Registry{}
		watchContextMock := &mocks.WatchConfigurationContext{}
		globalConfigMock := &mocks.ConfigurationContext{}
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("mykey", nil)
		regMock.On("RootConfig").Return(watchContextMock, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)
		watchContextMock.On("Watch", mock.Anything, "/config/_global/certificate", true, mock.Anything).Return()

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		sslUpdater := &sslCertificateUpdater{
			client:    clientMock,
			namespace: namespace,
			registry:  regMock,
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
		regMock := &mocks.Registry{}

		watchContextMock := &mocks.WatchConfigurationContext{}
		watchContextMock.On("Watch", mock.Anything, "/config/_global/certificate", true, mock.Anything).Run(func(args mock.Arguments) {
			channelobject := args.Get(3)
			sendChannel, ok := channelobject.(chan *coreosclient.Response)

			if ok {
				testResponse := &coreosclient.Response{}
				sendChannel <- testResponse
			}
		}).Return()
		regMock.On("RootConfig").Return(watchContextMock, nil)

		globalConfigMock := &mocks.ConfigurationContext{}
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
		regMock := &mocks.Registry{}

		watchContextMock := &mocks.WatchConfigurationContext{}
		watchContextMock.On("Watch", mock.Anything, "/config/_global/certificate", true, mock.Anything).Run(func(args mock.Arguments) {
			channelobject := args.Get(3)
			sendChannel, ok := channelobject.(chan *coreosclient.Response)

			if ok {
				testResponse := &coreosclient.Response{}
				sendChannel <- testResponse
			}
		}).Return()
		regMock.On("RootConfig").Return(watchContextMock, nil)

		globalConfigMock := &mocks.ConfigurationContext{}
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
		regMock := &mocks.Registry{}

		globalConfigMock := &mocks.ConfigurationContext{}
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
		regMock := &mocks.Registry{}

		globalConfigMock := &mocks.ConfigurationContext{}
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
		regMock := &mocks.Registry{}

		globalConfigMock := &mocks.ConfigurationContext{}
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

	t.Run("successfully handle ssl change with existing ssl secret", func(t *testing.T) {
		// given
		regMock := &mocks.Registry{}

		globalConfigMock := &mocks.ConfigurationContext{}
		globalConfigMock.On("Get", "certificate/server.crt").Return("mycert", nil)
		globalConfigMock.On("Get", "certificate/server.key").Return("mykey", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

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
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(initialSslSecret).Build()
		sslUpdater := &sslCertificateUpdater{
			client:    clientMock,
			namespace: namespace,
			registry:  regMock,
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
		regMock := &mocks.Registry{}

		globalConfigMock := &mocks.ConfigurationContext{}
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
		require.NoError(t, err)

		sslSecret := &v1.Secret{}
		objectKey := types.NamespacedName{Namespace: namespace, Name: certificateSecretName}
		err = clientMock.Get(context.Background(), objectKey, sslSecret)
		require.NoError(t, err)

		assert.Equal(t, "mycert", sslSecret.StringData[v1.TLSCertKey])
		assert.Equal(t, "mykey", sslSecret.StringData[v1.TLSPrivateKeyKey])
	})
}
