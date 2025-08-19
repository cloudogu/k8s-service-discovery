package controllers

import (
	"context"
	"crypto/x509"
	_ "embed"
	"testing"
	"time"

	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
	certProvidedGlobalConfig := config.CreateGlobalConfig(config.Entries{
		"certificate/type": "provided",
	})

	t.Run("should return error on error creating watch", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(certProvidedGlobalConfig, nil)
		mockGlobalConfigRepo.EXPECT().Watch(testCtx, mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sut.Start(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create fqdn watch")
	})

	t.Run("should return and log message if channel is closed", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(certProvidedGlobalConfig, nil)
		mockGlobalConfigRepo.EXPECT().Watch(testCtx, mock.Anything, mock.Anything, mock.Anything).Return(resultChannel, nil)
		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
		}
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
		mockLogSink.EXPECT().Info(0, "Starting selfsigned certificate updater...")
		mockLogSink.EXPECT().Info(0, "start global config watcher for ssl certificates")
		mockLogSink.EXPECT().Info(0, "fqdn watch channel was closed - stop watch")

		// when
		err := sut.Start(testCtx)
		timer := time.NewTimer(time.Second)
		<-timer.C
		close(resultChannel)
		timer = time.NewTimer(time.Second)
		<-timer.C

		// then
		require.NoError(t, err)
	})

	t.Run("should log error on error in result channel", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		mockGlobalConfigRepo.EXPECT().Get(ctx).Return(certProvidedGlobalConfig, nil)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		mockGlobalConfigRepo.EXPECT().Watch(ctx, mock.Anything, mock.Anything, mock.Anything).Return(resultChannel, nil)
		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
		}
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
		mockLogSink.EXPECT().Info(0, "Starting selfsigned certificate updater...")
		mockLogSink.EXPECT().Info(0, "start global config watcher for ssl certificates")
		mockLogSink.EXPECT().Info(0, "context done - stop global config watcher for fqdn changes")
		mockLogSink.EXPECT().Error(assert.AnError, "fqdn watch channel error").Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		// when
		err := sut.Start(ctx)

		resultChannel <- repository.GlobalConfigWatchResult{
			Err: assert.AnError,
		}

		// then
		require.NoError(t, err)
		<-ctx.Done()
		timer := time.NewTimer(time.Millisecond + 250)
		<-timer.C
	})

	t.Run("run start without change and send done to context", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second)
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		mockGlobalConfigRepo.EXPECT().Get(ctx).Return(certProvidedGlobalConfig, nil)
		mockGlobalConfigRepo.EXPECT().Watch(ctx, mock.Anything, mock.Anything, mock.Anything).Return(resultChannel, nil)

		namespace := "myTestNamespace"

		sut := &selfsignedCertificateUpdater{
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
		}

		// when
		err := sut.Start(ctx)
		cancelFunc()

		// then
		require.NoError(t, err)
	})

	t.Run("should fail to get certificate type", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithCancel(context.Background())
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
		mockLogSink.EXPECT().Info(0, "Starting selfsigned certificate updater...")
		mockLogSink.EXPECT().Info(0, "start global config watcher for ssl certificates")
		mockLogSink.EXPECT().Info(0, "context done - stop global config watcher for fqdn changes")
		mockLogSink.EXPECT().Info(0, "FQDN, alternativeFQDNs or domain changed in registry. Checking for selfsigned certificate...")
		mockLogSink.EXPECT().Error(mock.Anything, "failed to handle fqdn update", mock.Anything).Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		mockGlobalConfigRepo.EXPECT().Get(ctx).Return(certProvidedGlobalConfig, nil).Once()
		mockGlobalConfigRepo.EXPECT().Watch(ctx, mock.Anything, mock.Anything, mock.Anything).Return(resultChannel, nil)

		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "",
		})
		mockGlobalConfigRepo.EXPECT().Get(ctx).Return(globalConfig, nil).Once()

		mockSecretClient := newMockSecretClient(t)

		namespace := "myTestNamespace"
		sut := &selfsignedCertificateUpdater{
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mockSecretClient,
		}

		// when
		err := sut.Start(ctx)
		time.Sleep(time.Second)
		resultChannel <- repository.GlobalConfigWatchResult{}

		// then
		require.NoError(t, err)
		<-ctx.Done()
		// Wait for last log
		timer := time.NewTimer(time.Second)
		<-timer.C
	})

	t.Run("should fail to getting global config", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithCancel(context.Background())
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
		mockLogSink.EXPECT().Info(0, "Starting selfsigned certificate updater...")
		mockLogSink.EXPECT().Info(0, "start global config watcher for ssl certificates")
		mockLogSink.EXPECT().Info(0, "context done - stop global config watcher for fqdn changes")
		mockLogSink.EXPECT().Info(0, "FQDN, alternativeFQDNs or domain changed in registry. Checking for selfsigned certificate...")
		mockLogSink.EXPECT().Error(mock.Anything, "failed to handle fqdn update", mock.Anything).Run(func(err error, msg string, keysAndValues ...interface{}) {
			cancelFunc()
		})

		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		mockGlobalConfigRepo.EXPECT().Get(ctx).Return(certProvidedGlobalConfig, nil).Once()
		mockGlobalConfigRepo.EXPECT().Watch(ctx, mock.Anything, mock.Anything, mock.Anything).Return(resultChannel, nil)
		mockGlobalConfigRepo.EXPECT().Get(ctx).Return(config.GlobalConfig{}, assert.AnError).Once()

		mockSecretClient := newMockSecretClient(t)

		namespace := "myTestNamespace"
		sut := &selfsignedCertificateUpdater{
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mockSecretClient,
		}

		// when
		err := sut.Start(ctx)
		resultChannel <- repository.GlobalConfigWatchResult{}

		// then
		require.NoError(t, err)
		<-ctx.Done()
	})

	t.Run("should succeed for not selfsigned certificate", func(t *testing.T) {
		// given
		ctx, cancelFunc := context.WithCancel(context.Background())
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
		mockLogSink.EXPECT().Info(0, "Starting selfsigned certificate updater...")
		mockLogSink.EXPECT().Info(0, "start global config watcher for ssl certificates")
		mockLogSink.EXPECT().Info(0, "FQDN, alternativeFQDNs or domain changed in registry. Checking for selfsigned certificate...")

		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		resultChannel := make(chan repository.GlobalConfigWatchResult)
		mockGlobalConfigRepo.EXPECT().Watch(ctx, mock.Anything, mock.Anything, mock.Anything).Return(resultChannel, nil)

		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "external",
		})
		mockGlobalConfigRepo.EXPECT().Get(ctx).Return(globalConfig, nil)

		mockSecretClient := newMockSecretClient(t)

		namespace := "myTestNamespace"
		sut := &selfsignedCertificateUpdater{
			namespace:        namespace,
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mockSecretClient,
		}

		// when
		err := sut.Start(ctx)
		resultChannel <- repository.GlobalConfigWatchResult{}
		timer := time.NewTimer(2 * time.Second)
		<-timer.C

		// then
		require.NoError(t, err)
		cancelFunc()
	})
}

func Test_selfsignedCertificateUpdater_handleFqdnChange(t *testing.T) {
	t.Run("should fail parsing the cert", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type":       "selfsigned",
			"certificate/server.crt": "unparsableCert",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockSecretClient := newMockSecretClient(t)
		mockSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte("unparsableCert"),
			"tls.key": []byte("key"),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			namespace:        testNamespace,
			secretClient:     mockSecretClient,
		}

		// when
		err := sut.handleFqdnChange(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse certificate PEM of previous certificate")
	})

	t.Run("should fail parsing the cert block", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type":       "selfsigned",
			"certificate/server.crt": config.Value(pubPEMData),
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockSecretClient := newMockSecretClient(t)
		mockSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(pubPEMData),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			namespace:        testNamespace,
			secretClient:     mockSecretClient,
		}

		// when
		err := sut.handleFqdnChange(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to parse previous certificate")
	})

	t.Run("should fail to create and save certificate", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "selfsigned",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		creatorMock := newMockSelfSignedCertificateCreator(t)
		creatorMock.EXPECT().CreateAndSafeCertificate(testCtx, 365, "DE", "Lower Saxony", "Brunswick", []string{"192.168.56.2", "local.cloudogu.com"}).Return(assert.AnError)

		mockSecretClient := newMockSecretClient(t)
		mockSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo:   mockGlobalConfigRepo,
			namespace:          testNamespace,
			certificateCreator: creatorMock,
			secretClient:       mockSecretClient,
		}

		// when
		err := sut.handleFqdnChange(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to regenerate and safe selfsigned certificate")
	})

	t.Run("should fail because a non existent certificate", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "selfsigned",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockSecretClient := newMockSecretClient(t)
		mockSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			namespace:        testNamespace,
			secretClient:     mockSecretClient,
		}

		// when
		err := sut.handleFqdnChange(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "could not find certificate in ecosystem certificate secret")
	})

	t.Run("should fail because an empty certificate", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "selfsigned",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mockSecretClient := newMockSecretClient(t)
		mockSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(""),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			namespace:        testNamespace,
			secretClient:     mockSecretClient,
		}

		// when
		err := sut.handleFqdnChange(testCtx)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "could not find certificate in ecosystem certificate secret")
	})

	t.Run("successfully regenerate the certificate", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "selfsigned",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		creatorMock := newMockSelfSignedCertificateCreator(t)
		creatorMock.EXPECT().CreateAndSafeCertificate(testCtx, 365, "DE", "Lower Saxony", "Brunswick", []string{"192.168.56.2", "local.cloudogu.com"}).Return(nil)

		mockSecretClient := newMockSecretClient(t)
		mockSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo:   mockGlobalConfigRepo,
			namespace:          testNamespace,
			certificateCreator: creatorMock,
			secretClient:       mockSecretClient,
		}

		// when
		err := sut.handleFqdnChange(testCtx)

		// then
		require.NoError(t, err)
	})

	t.Run("successfully regenerate the certificate with alternative FQDNs", func(t *testing.T) {
		// given
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "selfsigned",
			"alternativeFQDNs": "fqdn1.example.com, fqdn2.example.com, fqdn3.example.com:certName3",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		creatorMock := newMockSelfSignedCertificateCreator(t)
		creatorMock.EXPECT().CreateAndSafeCertificate(testCtx, 365, "DE", "Lower Saxony", "Brunswick", []string{"192.168.56.2", "local.cloudogu.com", "fqdn1.example.com", "fqdn2.example.com"}).Return(nil)

		mockSecretClient := newMockSecretClient(t)
		mockSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo:   mockGlobalConfigRepo,
			namespace:          testNamespace,
			certificateCreator: creatorMock,
			secretClient:       mockSecretClient,
		}

		// when
		err := sut.handleFqdnChange(testCtx)

		// then
		require.NoError(t, err)
	})
}

func TestNewSelfsignedCertificateUpdater(t *testing.T) {
	t.Run("should return not nil", func(t *testing.T) {
		// given
		globalConfigRepo := NewMockGlobalConfigRepository(t)
		secretClientMock := newMockSecretClient(t)

		// when
		sut := NewSelfsignedCertificateUpdater(testNamespace, globalConfigRepo, secretClientMock)

		// then
		require.NotNil(t, sut)
		assert.Equal(t, globalConfigRepo, sut.globalConfigRepo)
		assert.Equal(t, testNamespace, sut.namespace)
	})
}

func Test_getAlternativeFQDNs(t *testing.T) {
	t.Run("should correctly parse alternativeFQDNs", func(t *testing.T) {
		tests := []struct {
			name                string
			globalConfigEntries config.Entries
			expectedFQDNs       []string
		}{
			{
				name:                "no alternativeFQDNs key",
				globalConfigEntries: config.Entries{},
				expectedFQDNs:       []string{},
			},
			{
				name: "valid alternativeFQDNs",
				globalConfigEntries: config.Entries{
					"alternativeFQDNs": "fqdn1.example.com, fqdn2.example.com:certName2, fqdn3.example.com",
				},
				expectedFQDNs: []string{"fqdn1.example.com", "fqdn3.example.com"},
			},
			{
				name: "empty alternativeFQDNs",
				globalConfigEntries: config.Entries{
					"alternativeFQDNs": "",
				},
				expectedFQDNs: []string{},
			},
			{
				name: "malformed alternativeFQDNs",
				globalConfigEntries: config.Entries{
					"alternativeFQDNs": ",,fqdn1.example.com",
				},
				expectedFQDNs: []string{"fqdn1.example.com"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// given
				globalConfig := config.CreateGlobalConfig(tt.globalConfigEntries)

				// when
				result := getAlternativeFQDNs(globalConfig)

				// then
				assert.ElementsMatch(t, tt.expectedFQDNs, result)
			})
		}
	})
}

func Test_selfsignedCertificateUpdater_shouldUpdateCurrentCertificate(t *testing.T) {
	t.Run("should return true", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"fqdn":             "newFQDN",
			"certificate/type": "selfsigned",
			"alternativeFQDNs": "fqdn1.example.com, fqdn2.example.com:certName2",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mSecretClient := newMockSecretClient(t)
		mSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.NoError(t, err)
		assert.True(t, shouldUpdate)
	})

	t.Run("should fail if secret does not exist", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"fqdn":             "newFQDN",
			"certificate/type": "selfsigned",
			"alternativeFQDNs": "fqdn1.example.com, fqdn2.example.com:certName2",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mSecretClient := newMockSecretClient(t)
		mSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(nil, assert.AnError)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get current certificate: failed to get secret for ssl read: assert.AnError")
		assert.False(t, shouldUpdate)
	})

	t.Run("should fail on error getting globalConfig", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(config.GlobalConfig{}, assert.AnError)

		mSecretClient := newMockSecretClient(t)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to get global config: assert.AnError")
		assert.False(t, shouldUpdate)
	})

	t.Run("should fail on error checking cert-type", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"fqdn":             "newFQDN",
			"alternativeFQDNs": "fqdn1.example.com, fqdn2.example.com:certName2",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mSecretClient := newMockSecretClient(t)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to check certificate-type: \"certificate/type\" is empty or doesn't exists")
		assert.False(t, shouldUpdate)
	})

	t.Run("should fail if fqdn does not exist", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"certificate/type": "selfsigned",
			"alternativeFQDNs": "fqdn1.example.com, fqdn2.example.com:certName2",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mSecretClient := newMockSecretClient(t)
		mSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.Error(t, err)
		assert.ErrorContains(t, err, "fqdn is empty or doesn't exist")
		assert.False(t, shouldUpdate)
	})

	t.Run("should fail if domain does not exist", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"fqdn":             "local.cloudogu.com",
			"certificate/type": "selfsigned",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mSecretClient := newMockSecretClient(t)
		mSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.Error(t, err)
		assert.ErrorContains(t, err, "domain is empty or doesn't exist")
		assert.False(t, shouldUpdate)
	})

	t.Run("should return true if domain has changed", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"fqdn":             "local.cloudogu.com",
			"domain":           "other.cloudogu.com",
			"certificate/type": "selfsigned",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mSecretClient := newMockSecretClient(t)
		mSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.NoError(t, err)
		assert.True(t, shouldUpdate)
	})

	t.Run("should return false if nothing has changed", func(t *testing.T) {
		mockGlobalConfigRepo := NewMockGlobalConfigRepository(t)
		globalConfig := config.CreateGlobalConfig(config.Entries{
			"fqdn":             "local.cloudogu.com",
			"domain":           "k3ces.local",
			"certificate/type": "selfsigned",
		})
		mockGlobalConfigRepo.EXPECT().Get(testCtx).Return(globalConfig, nil)

		mSecretClient := newMockSecretClient(t)
		mSecretClient.EXPECT().Get(testCtx, "ecosystem-certificate", metav1.GetOptions{}).Return(&corev1.Secret{Data: map[string][]byte{
			"tls.crt": []byte(serverCert),
		}}, nil)

		sut := &selfsignedCertificateUpdater{
			globalConfigRepo: mockGlobalConfigRepo,
			secretClient:     mSecretClient,
		}

		shouldUpdate, err := sut.shouldUpdateCurrentCertificate(testCtx)

		require.NoError(t, err)
		assert.False(t, shouldUpdate)
	})
}

func Test_certificateHasAllDNSNames(t *testing.T) {
	t.Run("should return true if no dnsNames given", func(t *testing.T) {
		assert.True(t, certificateHasAllDNSNames(nil, []string{}))
	})

	t.Run("should return true", func(t *testing.T) {
		cert := &x509.Certificate{DNSNames: []string{"a", "b", "c"}}
		assert.True(t, certificateHasAllDNSNames(cert, []string{"a", "c"}))
	})

	t.Run("should return false", func(t *testing.T) {
		cert := &x509.Certificate{DNSNames: []string{"a", "b", "c"}}
		assert.False(t, certificateHasAllDNSNames(cert, []string{"a", "c", "d"}))
	})
}
