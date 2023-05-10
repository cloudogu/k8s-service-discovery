package controllers

import (
	"context"
	"github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	etcdclient "go.etcd.io/etcd/client/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

func Test_selfsignedCertificateUpdater_Start(t *testing.T) {
	t.Run("run start without change and send done to context", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		watchContextMock := newMockWatchConfigurationContext(t)
		regMock.EXPECT().RootConfig().Return(watchContextMock)
		watchContextMock.EXPECT().Watch(mock.Anything, "/config/_global/fqdn", false, mock.Anything).Return()

		recorderMock := newMockEventRecorder(t)

		namespace := "myTestNamespace"
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithObjects(deployment).Build()
		sut := &selfsignedCertificateUpdater{
			client:        clientMock,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: recorderMock,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*50)

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
	})

	t.Run("should fail to get certificate type", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		watchContextMock := newMockWatchConfigurationContext(t)
		watchContextMock.EXPECT().Watch(mock.Anything, "/config/_global/fqdn", false, mock.Anything).Run(func(_ context.Context, _ string, _ bool, eventChannel chan *etcdclient.Response) {
			testResponse := &etcdclient.Response{}
			eventChannel <- testResponse
		}).Return()
		regMock.EXPECT().RootConfig().Return(watchContextMock)

		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/type").Return("", assert.AnError)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		namespace := "myTestNamespace"
		sut := &selfsignedCertificateUpdater{
			client:        nil,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: nil,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "could get certificate type from registry")
	})

	t.Run("should succeed for not selfsigned certificate", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		watchContextMock := newMockWatchConfigurationContext(t)
		watchContextMock.EXPECT().Watch(mock.Anything, "/config/_global/fqdn", false, mock.Anything).Run(func(_ context.Context, _ string, _ bool, eventChannel chan *etcdclient.Response) {
			testResponse := &etcdclient.Response{}
			eventChannel <- testResponse
		}).Return()
		regMock.EXPECT().RootConfig().Return(watchContextMock)

		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/type").Return("notselfsigned", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		namespace := "myTestNamespace"
		sut := &selfsignedCertificateUpdater{
			client:        nil,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: nil,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
	})

	t.Run("should fail to get server certificate", func(t *testing.T) {
		// given
		regMock := newMockCesRegistry(t)
		watchContextMock := newMockWatchConfigurationContext(t)
		watchContextMock.EXPECT().Watch(mock.Anything, "/config/_global/fqdn", false, mock.Anything).Run(func(_ context.Context, _ string, _ bool, eventChannel chan *etcdclient.Response) {
			testResponse := &etcdclient.Response{}
			eventChannel <- testResponse
		}).Return()
		regMock.EXPECT().RootConfig().Return(watchContextMock)

		globalConfigMock := mocks.NewConfigurationContext(t)
		globalConfigMock.On("Get", "certificate/type").Return("selfsigned", nil)
		globalConfigMock.On("Get", "certificate/server.crt").Return("", assert.AnError)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		namespace := "myTestNamespace"
		sut := &selfsignedCertificateUpdater{
			client:        nil,
			namespace:     namespace,
			registry:      regMock,
			eventRecorder: nil,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get previous certificate from global config")
	})
}
