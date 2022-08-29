package controllers

import (
	"context"
	cesmocks "github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/cloudogu/k8s-service-discovery/controllers/mocks"
	coreosclient "github.com/coreos/etcd/client"
	etcdclient "github.com/coreos/etcd/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

func TestNewMaintenanceModeUpdater(t *testing.T) {
	t.Run("failed to create registry", func(t *testing.T) {
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, err := NewMaintenanceModeUpdater(clientMock, "%!%*Ä'%'!%'", &mocks.IngressUpdater{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create etcd client")
		require.Nil(t, creator)
	})

	t.Run("successfully create updater", func(t *testing.T) {
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		creator, err := NewMaintenanceModeUpdater(clientMock, "test", &mocks.IngressUpdater{})

		require.NoError(t, err)
		require.NotNil(t, creator)
	})
}

func Test_maintenanceModeUpdater_Start(t *testing.T) {
	t.Run("error on maintenance update", func(t *testing.T) {
		// given
		regMock := &cesmocks.Registry{}

		watchContextMock := &cesmocks.WatchConfigurationContext{}
		watchContextMock.On("Watch", mock.Anything, "/config/_global/maintenance", true, mock.Anything).Run(func(args mock.Arguments) {
			channelobject := args.Get(3)
			sendChannel, ok := channelobject.(chan *coreosclient.Response)

			if ok {
				testResponse := &coreosclient.Response{}
				sendChannel <- testResponse
			}
		}).Return()

		globalConfigMock := &cesmocks.ConfigurationContext{}

		globalConfigMock.On("Get", "maintenance").Return("", assert.AnError)
		regMock.On("RootConfig").Return(watchContextMock, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := &mocks.IngressUpdater{}

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		maintenanceUpdater := &maintenanceModeUpdater{
			client:         clientMock,
			namespace:      namespace,
			registry:       regMock,
			ingressUpdater: ingressUpdater,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*100)

		// when
		err := maintenanceUpdater.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		mock.AssertExpectationsForObjects(t, regMock, watchContextMock, globalConfigMock, regMock)
	})

	t.Run("run without and terminate without any problems", func(t *testing.T) {
		// given
		regMock := &cesmocks.Registry{}

		watchContextMock := &cesmocks.WatchConfigurationContext{}
		watchContextMock.On("Watch", mock.Anything, "/config/_global/maintenance", true, mock.Anything).Run(func(args mock.Arguments) {
			channelobject := args.Get(3)
			sendChannel, ok := channelobject.(chan *coreosclient.Response)

			if ok {
				testResponse := &coreosclient.Response{}
				sendChannel <- testResponse
			}
		}).Return()

		globalConfigMock := &cesmocks.ConfigurationContext{}

		globalConfigMock.On("Get", "maintenance").Return("false", nil)
		regMock.On("RootConfig").Return(watchContextMock, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := &mocks.IngressUpdater{}

		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).Build()
		namespace := "myTestNamespace"
		maintenanceUpdater := &maintenanceModeUpdater{
			client:         clientMock,
			namespace:      namespace,
			registry:       regMock,
			ingressUpdater: ingressUpdater,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond*100)

		// when
		err := maintenanceUpdater.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
		mock.AssertExpectationsForObjects(t, regMock, watchContextMock, globalConfigMock, regMock)
	})
}

func Test_maintenanceModeUpdater_activateMaintenanceMode(t *testing.T) {

}

func Test_maintenanceModeUpdater_deactivateMaintenanceMode(t *testing.T) {

}

func Test_maintenanceModeUpdater_getAllServices(t *testing.T) {

}

func Test_maintenanceModeUpdater_handleMaintenanceModeUpdate(t *testing.T) {
	t.Run("activate maintenance mode with error", func(t *testing.T) {
		// given
		regMock := &cesmocks.Registry{}
		globalConfigMock := &cesmocks.ConfigurationContext{}
		globalConfigMock.On("Get", "maintenance").Return("true", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := &mocks.IngressUpdater{}
		ingressUpdater.On("UpdateIngressOfService", mock.Anything, mock.Anything, true).Return(assert.AnError)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		maintenanceUpdater := &maintenanceModeUpdater{
			client:         clientMock,
			namespace:      namespace,
			registry:       regMock,
			ingressUpdater: ingressUpdater,
		}

		// when
		err := maintenanceUpdater.handleMaintenanceModeUpdate(context.Background())

		// then
		require.ErrorIs(t, err, assert.AnError)
		mock.AssertExpectationsForObjects(t, regMock, globalConfigMock, regMock, ingressUpdater)
	})

	t.Run("deactivate maintenance mode with error", func(t *testing.T) {
		// given
		regMock := &cesmocks.Registry{}
		globalConfigMock := &cesmocks.ConfigurationContext{}

		keyNotFoundErr := etcdclient.Error{Code: etcdclient.ErrorCodeKeyNotFound}
		globalConfigMock.On("Get", "maintenance").Return("", keyNotFoundErr)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		ingressUpdater := &mocks.IngressUpdater{}
		ingressUpdater.On("UpdateIngressOfService", mock.Anything, mock.Anything, false).Return(assert.AnError)

		namespace := "myTestNamespace"
		testService := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "testService", Namespace: namespace}}
		serviceList := &corev1.ServiceList{Items: []corev1.Service{*testService}}
		clientMock := testclient.NewClientBuilder().WithScheme(getScheme()).WithLists(serviceList).Build()

		maintenanceUpdater := &maintenanceModeUpdater{
			client:         clientMock,
			namespace:      namespace,
			registry:       regMock,
			ingressUpdater: ingressUpdater,
		}

		// when
		err := maintenanceUpdater.handleMaintenanceModeUpdate(context.Background())

		// then
		require.ErrorIs(t, err, assert.AnError)
		mock.AssertExpectationsForObjects(t, regMock, globalConfigMock, regMock, ingressUpdater)
	})
}

func Test_maintenanceModeUpdater_isMaintenanceModeActive(t *testing.T) {

}

func Test_maintenanceModeUpdater_startEtcdWatch(t *testing.T) {

}
