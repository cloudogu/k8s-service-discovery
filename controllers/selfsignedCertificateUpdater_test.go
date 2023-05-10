package controllers

import (
	"context"
	_ "embed"
	"github.com/cloudogu/cesapp-lib/registry/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	etcdclient "go.etcd.io/etcd/client/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"testing"
	"time"
)

//go:embed testdata/server.crt
var serverCert string

var pubPEMData = `
-----BEGIN PUBLIC KEY-----
MIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKCAgEAlRuRnThUjU8/prwYxbty
WPT9pURI3lbsKMiB6Fn/VHOKE13p4D8xgOCADpdRagdT6n4etr9atzDKUSvpMtR3
CP5noNc97WiNCggBjVWhs7szEe8ugyqF23XwpHQ6uV1LKH50m92MbOWfCtjU9p/x
qhNpQQ1AZhqNy5Gevap5k8XzRmjSldNAFZMY7Yv3Gi+nyCwGwpVtBUwhuLzgNFK/
yDtw2WcWmUU7NuC8Q6MWvPebxVtCfVp/iQU6q60yyt6aGOBkhAX0LpKAEhKidixY
nP9PNVBvxgu3XZ4P36gZV6+ummKdBVnc3NqwBLu5+CcdRdusmHPHd5pHf4/38Z3/
6qU2a/fPvWzceVTEgZ47QjFMTCTmCwNt29cvi7zZeQzjtwQgn4ipN9NibRH/Ax/q
TbIzHfrJ1xa2RteWSdFjwtxi9C20HUkjXSeI4YlzQMH0fPX6KCE7aVePTOnB69I/
a9/q96DiXZajwlpq3wFctrs1oXqBp5DVrCIj8hU2wNgB7LtQ1mCtsYz//heai0K9
PhE4X6hiE0YmeAZjR0uHl8M/5aW9xCoJ72+12kKpWAa0SFRWLy6FejNYCYpkupVJ
yecLk/4L1W0l6jQQZnWErXZYe0PNFcmwGXy1Rep83kfBRNKRy5tvocalLlwXLdUk
AIU+2GKjyT3iMuzZxxFxPFMCAwEAAQ==
-----END PUBLIC KEY-----
and some more`

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

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(mocks.Anything,
			types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}, mocks.Anything).
			Return(nil)

		sut := &selfsignedCertificateUpdater{
			client:        k8sClientMock,
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

	t.Run("should fail to parse certificate block", func(t *testing.T) {
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
		globalConfigMock.On("Get", "certificate/server.crt").Return("unparsableCert", nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		namespace := "myTestNamespace"

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(mocks.Anything,
			types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}, mocks.Anything).
			Return(nil)

		sut := &selfsignedCertificateUpdater{
			client:        k8sClientMock,
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
		assert.ErrorContains(t, err, "failed to parse certificate PEM of previous certificate")
	})

	t.Run("should fail to parse certificate", func(t *testing.T) {
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
		globalConfigMock.On("Get", "certificate/server.crt").Return(pubPEMData, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		namespace := "myTestNamespace"

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(mocks.Anything,
			types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}, mocks.Anything).
			Return(nil)

		sut := &selfsignedCertificateUpdater{
			client:        k8sClientMock,
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
		assert.ErrorContains(t, err, "failed to parse previous certificate: x509: malformed serial number")
	})

	t.Run("should fail to create and save certificate", func(t *testing.T) {
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
		globalConfigMock.On("Get", "certificate/server.crt").Return(serverCert, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		creatorMock := newMockSelfSignedCertificateCreator(t)
		creatorMock.EXPECT().CreateAndSafeCertificate(365, "DE", "Lower Saxony", "Brunswick", []string{"192.168.56.2", "local.cloudogu.com"}).Return(assert.AnError)

		namespace := "myTestNamespace"

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(mocks.Anything,
			types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}, mocks.Anything).
			Return(nil)

		sut := &selfsignedCertificateUpdater{
			client:             k8sClientMock,
			namespace:          namespace,
			registry:           regMock,
			eventRecorder:      nil,
			certificateCreator: creatorMock,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to regenerate and safe selfsigned certificate")
	})

	t.Run("should fail to get deployment", func(t *testing.T) {
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
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		namespace := "myTestNamespace"

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(mocks.Anything,
			types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}, mocks.Anything).
			Return(assert.AnError)

		sut := &selfsignedCertificateUpdater{
			client:             k8sClientMock,
			namespace:          namespace,
			registry:           regMock,
			eventRecorder:      nil,
			certificateCreator: nil,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "selfsigned certificate handling: failed to get deployment [k8s-service-discovery-controller-manager]")
	})

	t.Run("should succeed", func(t *testing.T) {
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
		globalConfigMock.On("Get", "certificate/server.crt").Return(serverCert, nil)
		regMock.On("GlobalConfig").Return(globalConfigMock, nil)

		creatorMock := newMockSelfSignedCertificateCreator(t)
		creatorMock.EXPECT().CreateAndSafeCertificate(365, "DE", "Lower Saxony", "Brunswick", []string{"192.168.56.2", "local.cloudogu.com"}).Return(nil)

		namespace := "myTestNamespace"

		k8sClientMock := newMockK8sClient(t)
		k8sClientMock.EXPECT().Get(mocks.Anything,
			types.NamespacedName{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}, mocks.Anything).
			Return(nil)

		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Event(mock.IsType(&appsv1.Deployment{}), "Normal", "FQDNChange", "Selfsigned certificate regenerated.")

		sut := &selfsignedCertificateUpdater{
			client:             k8sClientMock,
			namespace:          namespace,
			registry:           regMock,
			eventRecorder:      recorderMock,
			certificateCreator: creatorMock,
		}

		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*2)

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
	})
}
