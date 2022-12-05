// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	cache "sigs.k8s.io/controller-runtime/pkg/cache"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	context "context"

	healthz "sigs.k8s.io/controller-runtime/pkg/healthz"

	http "net/http"

	logr "github.com/go-logr/logr"

	manager "sigs.k8s.io/controller-runtime/pkg/manager"

	meta "k8s.io/apimachinery/pkg/api/meta"

	mock "github.com/stretchr/testify/mock"

	record "k8s.io/client-go/tools/record"

	rest "k8s.io/client-go/rest"

	runtime "k8s.io/apimachinery/pkg/runtime"

	v1alpha1 "sigs.k8s.io/controller-runtime/pkg/config/v1alpha1"

	webhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

// Add provides a mock function with given fields: _a0
func (_m *Manager) Add(_a0 manager.Runnable) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(manager.Runnable) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddHealthzCheck provides a mock function with given fields: name, check
func (_m *Manager) AddHealthzCheck(name string, check healthz.Checker) error {
	ret := _m.Called(name, check)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, healthz.Checker) error); ok {
		r0 = rf(name, check)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddMetricsExtraHandler provides a mock function with given fields: path, handler
func (_m *Manager) AddMetricsExtraHandler(path string, handler http.Handler) error {
	ret := _m.Called(path, handler)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, http.Handler) error); ok {
		r0 = rf(path, handler)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddReadyzCheck provides a mock function with given fields: name, check
func (_m *Manager) AddReadyzCheck(name string, check healthz.Checker) error {
	ret := _m.Called(name, check)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, healthz.Checker) error); ok {
		r0 = rf(name, check)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Elected provides a mock function with given fields:
func (_m *Manager) Elected() <-chan struct{} {
	ret := _m.Called()

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func() <-chan struct{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// GetAPIReader provides a mock function with given fields:
func (_m *Manager) GetAPIReader() client.Reader {
	ret := _m.Called()

	var r0 client.Reader
	if rf, ok := ret.Get(0).(func() client.Reader); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(client.Reader)
		}
	}

	return r0
}

// GetCache provides a mock function with given fields:
func (_m *Manager) GetCache() cache.Cache {
	ret := _m.Called()

	var r0 cache.Cache
	if rf, ok := ret.Get(0).(func() cache.Cache); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(cache.Cache)
		}
	}

	return r0
}

// GetClient provides a mock function with given fields:
func (_m *Manager) GetClient() client.Client {
	ret := _m.Called()

	var r0 client.Client
	if rf, ok := ret.Get(0).(func() client.Client); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(client.Client)
		}
	}

	return r0
}

// GetConfig provides a mock function with given fields:
func (_m *Manager) GetConfig() *rest.Config {
	ret := _m.Called()

	var r0 *rest.Config
	if rf, ok := ret.Get(0).(func() *rest.Config); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*rest.Config)
		}
	}

	return r0
}

// GetControllerOptions provides a mock function with given fields:
func (_m *Manager) GetControllerOptions() v1alpha1.ControllerConfigurationSpec {
	ret := _m.Called()

	var r0 v1alpha1.ControllerConfigurationSpec
	if rf, ok := ret.Get(0).(func() v1alpha1.ControllerConfigurationSpec); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(v1alpha1.ControllerConfigurationSpec)
	}

	return r0
}

// GetEventRecorderFor provides a mock function with given fields: name
func (_m *Manager) GetEventRecorderFor(name string) record.EventRecorder {
	ret := _m.Called(name)

	var r0 record.EventRecorder
	if rf, ok := ret.Get(0).(func(string) record.EventRecorder); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(record.EventRecorder)
		}
	}

	return r0
}

// GetFieldIndexer provides a mock function with given fields:
func (_m *Manager) GetFieldIndexer() client.FieldIndexer {
	ret := _m.Called()

	var r0 client.FieldIndexer
	if rf, ok := ret.Get(0).(func() client.FieldIndexer); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(client.FieldIndexer)
		}
	}

	return r0
}

// GetLogger provides a mock function with given fields:
func (_m *Manager) GetLogger() logr.Logger {
	ret := _m.Called()

	var r0 logr.Logger
	if rf, ok := ret.Get(0).(func() logr.Logger); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(logr.Logger)
	}

	return r0
}

// GetRESTMapper provides a mock function with given fields:
func (_m *Manager) GetRESTMapper() meta.RESTMapper {
	ret := _m.Called()

	var r0 meta.RESTMapper
	if rf, ok := ret.Get(0).(func() meta.RESTMapper); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(meta.RESTMapper)
		}
	}

	return r0
}

// GetScheme provides a mock function with given fields:
func (_m *Manager) GetScheme() *runtime.Scheme {
	ret := _m.Called()

	var r0 *runtime.Scheme
	if rf, ok := ret.Get(0).(func() *runtime.Scheme); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*runtime.Scheme)
		}
	}

	return r0
}

// GetWebhookServer provides a mock function with given fields:
func (_m *Manager) GetWebhookServer() *webhook.Server {
	ret := _m.Called()

	var r0 *webhook.Server
	if rf, ok := ret.Get(0).(func() *webhook.Server); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*webhook.Server)
		}
	}

	return r0
}

// SetFields provides a mock function with given fields: _a0
func (_m *Manager) SetFields(_a0 interface{}) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Start provides a mock function with given fields: ctx
func (_m *Manager) Start(ctx context.Context) error {
	ret := _m.Called(ctx)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewManager interface {
	mock.TestingT
	Cleanup(func())
}

// NewManager creates a new instance of Manager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewManager(t mockConstructorTestingTNewManager) *Manager {
	mock := &Manager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
