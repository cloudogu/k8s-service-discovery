package main

import (
	"context"
	"flag"
	"github.com/stretchr/testify/assert"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func Test_startManager(t *testing.T) {
	// override default controller method to create a new manager
	oldNewManagerDelegate := ctrl.NewManager
	defer func() { ctrl.NewManager = oldNewManagerDelegate }()

	// override default controller method to retrieve a kube config
	oldGetConfigOrDieDelegate := ctrl.GetConfigOrDie
	defer func() { ctrl.GetConfigOrDie = oldGetConfigOrDieDelegate }()
	ctrl.GetConfigOrDie = func() *rest.Config {
		return &rest.Config{}
	}

	// override default controller method to retrieve a kube config
	oldGetConfig := ctrl.GetConfig
	defer func() { ctrl.GetConfig = oldGetConfig }()
	ctrl.GetConfig = func() (*rest.Config, error) {
		return &rest.Config{}, nil
	}

	// override default controller method to signal the setup handler
	oldHandler := ctrl.SetupSignalHandler
	defer func() { ctrl.SetupSignalHandler = oldHandler }()
	ctrl.SetupSignalHandler = func() context.Context {
		return context.TODO()
	}

	// override default controller method to retrieve a kube config
	oldSetLoggerDelegate := ctrl.SetLogger
	defer func() { ctrl.SetLogger = oldSetLoggerDelegate }()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	t.Run("Error on missing namespace environment variable", func(t *testing.T) {
		// given
		err := os.Unsetenv("WATCH_NAMESPACE")
		require.NoError(t, err)
		k8sManager := NewMockManager(t)
		oldNewManger := ctrl.NewManager
		defer func() { ctrl.NewManager = oldNewManger }()
		ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
			return k8sManager, nil
		}
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

		// when
		err = startManager()

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read namespace to watch from environment variable")
	})

	t.Setenv(namespaceEnvVar, "mynamespace")

	t.Run("Test with error on manager creation", func(t *testing.T) {
		// given
		oldNewManger := ctrl.NewManager
		defer func() { ctrl.NewManager = oldNewManger }()
		ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
			return nil, assert.AnError
		}
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

		// when
		err := startManager()

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to create new manager")
	})

	t.Run("fail setup when error on Add", func(t *testing.T) {
		// given
		k8sManager := NewMockManager(t)
		k8sManager.EXPECT().GetClient().Return(client)
		k8sManager.EXPECT().Add(mock.Anything).Return(assert.AnError)
		k8sManager.EXPECT().GetEventRecorderFor("k8s-service-discovery-controller-manager").Return(nil)
		oldNewManger := ctrl.NewManager
		defer func() { ctrl.NewManager = oldNewManger }()
		ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
			return k8sManager, nil
		}
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

		// when
		err := startManager()

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to create ingress class creator: failed to add ingress class creator as runnable to the manager")
	})

	t.Run("fail setup when error on AddHealthzCheck", func(t *testing.T) {
		// given
		k8sManager := NewMockManager(t)
		k8sManager.EXPECT().GetClient().Return(client)
		k8sManager.EXPECT().Add(mock.Anything).Return(nil)
		k8sManager.EXPECT().GetEventRecorderFor("k8s-service-discovery-controller-manager").Return(nil)
		k8sManager.EXPECT().GetControllerOptions().Return(config.Controller{})
		k8sManager.EXPECT().GetScheme().Return(scheme)
		k8sManager.EXPECT().GetLogger().Return(logger)
		k8sManager.EXPECT().GetCache().Return(nil)
		k8sManager.EXPECT().AddHealthzCheck(mock.Anything, mock.Anything).Return(assert.AnError)
		k8sManager.EXPECT().GetConfig().Return(&rest.Config{})
		oldNewManger := ctrl.NewManager
		defer func() { ctrl.NewManager = oldNewManger }()
		ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
			return k8sManager, nil
		}
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

		// when
		err := startManager()

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to configure service discovery manager: failed to configure reconciler: failed to add healthz check")
	})

	t.Run("fail setup when error on AddReadyzCheck", func(t *testing.T) {
		// given
		k8sManager := NewMockManager(t)
		k8sManager.EXPECT().GetClient().Return(client)
		k8sManager.EXPECT().Add(mock.Anything).Return(nil)
		k8sManager.EXPECT().GetEventRecorderFor("k8s-service-discovery-controller-manager").Return(nil)
		k8sManager.EXPECT().GetControllerOptions().Return(config.Controller{})
		k8sManager.EXPECT().GetScheme().Return(scheme)
		k8sManager.EXPECT().GetLogger().Return(logger)
		k8sManager.EXPECT().GetCache().Return(nil)
		k8sManager.EXPECT().AddHealthzCheck(mock.Anything, mock.Anything).Return(nil)
		k8sManager.EXPECT().AddReadyzCheck(mock.Anything, mock.Anything).Return(assert.AnError)
		k8sManager.EXPECT().GetConfig().Return(&rest.Config{})
		oldNewManger := ctrl.NewManager
		defer func() { ctrl.NewManager = oldNewManger }()
		ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
			return k8sManager, nil
		}
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

		// when
		err := startManager()

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failed to configure service discovery manager: failed to configure reconciler: failed to add readyz check")
	})

	t.Run("fail setup when error on Start", func(t *testing.T) {
		// given
		k8sManager := NewMockManager(t)
		k8sManager.EXPECT().GetClient().Return(client)
		k8sManager.EXPECT().Add(mock.Anything).Return(nil)
		k8sManager.EXPECT().GetEventRecorderFor("k8s-service-discovery-controller-manager").Return(nil)
		k8sManager.EXPECT().GetControllerOptions().Return(config.Controller{})
		k8sManager.EXPECT().GetScheme().Return(scheme)
		k8sManager.EXPECT().GetLogger().Return(logger)
		k8sManager.EXPECT().GetCache().Return(nil)
		k8sManager.EXPECT().AddHealthzCheck(mock.Anything, mock.Anything).Return(nil)
		k8sManager.EXPECT().AddReadyzCheck(mock.Anything, mock.Anything).Return(nil)
		k8sManager.EXPECT().Start(mock.Anything).Return(assert.AnError)
		k8sManager.EXPECT().GetConfig().Return(&rest.Config{})
		oldNewManger := ctrl.NewManager
		defer func() { ctrl.NewManager = oldNewManger }()
		ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
			return k8sManager, nil
		}
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

		// when
		err := startManager()

		// then
		require.ErrorIs(t, err, assert.AnError)
		assert.ErrorContains(t, err, "failure at service discovery manager: failed to start service discovery manager")
	})

	t.Run("should setup successfully", func(t *testing.T) {
		// given
		k8sManager := NewMockManager(t)
		k8sManager.EXPECT().GetClient().Return(client)
		k8sManager.EXPECT().Add(mock.Anything).Return(nil)
		k8sManager.EXPECT().GetEventRecorderFor("k8s-service-discovery-controller-manager").Return(nil)
		k8sManager.EXPECT().GetControllerOptions().Return(config.Controller{})
		k8sManager.EXPECT().GetScheme().Return(scheme)
		k8sManager.EXPECT().GetLogger().Return(logger)
		k8sManager.EXPECT().GetCache().Return(nil)
		k8sManager.EXPECT().AddHealthzCheck(mock.Anything, mock.Anything).Return(nil)
		k8sManager.EXPECT().AddReadyzCheck(mock.Anything, mock.Anything).Return(nil)
		k8sManager.EXPECT().Start(mock.Anything).Return(nil)
		k8sManager.EXPECT().GetConfig().Return(&rest.Config{})
		oldNewManger := ctrl.NewManager
		defer func() { ctrl.NewManager = oldNewManger }()
		ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
			return k8sManager, nil
		}
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

		// when
		err := startManager()

		// then
		require.NoError(t, err)
	})
}
