package controllers

import (
	"context"
	"testing"
	"time"

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
	// given
	regMock := &mocks.Registry{}
	watchContextMock := &mocks.WatchConfigurationContext{}
	regMock.On("RootConfig").Return(watchContextMock, nil)
	watchContextMock.On("Watch", "/config/_global/certificate", true, mock.Anything).Return()

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
}
