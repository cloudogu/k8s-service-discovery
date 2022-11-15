package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/cloudogu/k8s-service-discovery/controllers/mocks"

	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type mockDefinition struct {
	Arguments   []interface{}
	ReturnValue interface{}
}

func getCopyMap(definitions map[string]mockDefinition) map[string]mockDefinition {
	newCopyMap := map[string]mockDefinition{}
	for k, v := range definitions {
		newCopyMap[k] = v
	}
	return newCopyMap
}

func getNewMockManager(expectedErrorOnNewManager error, definitions map[string]mockDefinition) manager.Manager {
	k8sManager := &mocks.Manager{}
	ctrl.NewManager = func(config *rest.Config, options manager.Options) (manager.Manager, error) {
		for key, value := range definitions {
			k8sManager.Mock.On(key, value.Arguments...).Return(value.ReturnValue)
		}
		return k8sManager, expectedErrorOnNewManager
	}
	k8sManager.Mock.On("GetLogger").Return(ctrl.Log)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

	return k8sManager
}

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

	defaultMockDefinitions := map[string]mockDefinition{
		"GetScheme":            {ReturnValue: scheme},
		"GetClient":            {ReturnValue: client},
		"Add":                  {Arguments: []interface{}{mock.Anything}, ReturnValue: nil},
		"AddHealthzCheck":      {Arguments: []interface{}{mock.Anything, mock.Anything}, ReturnValue: nil},
		"AddReadyzCheck":       {Arguments: []interface{}{mock.Anything, mock.Anything}, ReturnValue: nil},
		"Start":                {Arguments: []interface{}{mock.Anything}, ReturnValue: nil},
		"GetControllerOptions": {ReturnValue: v1alpha1.ControllerConfigurationSpec{}},
		"SetFields":            {Arguments: []interface{}{mock.Anything}, ReturnValue: nil},
	}

	t.Run("Error on missing namespace environment variable", func(t *testing.T) {
		// given
		err := os.Unsetenv("WATCH_NAMESPACE")
		require.NoError(t, err)
		getNewMockManager(nil, defaultMockDefinitions)

		// when
		err = startManager()

		// then
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read namespace to watch from environment variable")
	})

	t.Setenv(namespaceEnvVar, "mynamespace")

	expectedError := fmt.Errorf("this is my expected error")

	t.Run("Test with error on manager creation", func(t *testing.T) {
		// given
		getNewMockManager(expectedError, defaultMockDefinitions)

		// when
		err := startManager()

		// then
		require.ErrorIs(t, err, expectedError)
	})

	mockDefinitionsThatCanFail := []string{
		"Add",
		"AddHealthzCheck",
		"AddReadyzCheck",
		"Start",
		"SetFields",
	}

	for _, mockDefinitionName := range mockDefinitionsThatCanFail {
		t.Run(fmt.Sprintf("fail setup when error on %s", mockDefinitionName), func(t *testing.T) {
			// given
			adaptedMockDefinitions := getCopyMap(defaultMockDefinitions)
			adaptedMockDefinitions[mockDefinitionName] = mockDefinition{
				Arguments:   adaptedMockDefinitions[mockDefinitionName].Arguments,
				ReturnValue: expectedError,
			}
			getNewMockManager(nil, adaptedMockDefinitions)

			// when
			err := startManager()

			// then
			require.ErrorIs(t, err, expectedError)
		})
	}
}
