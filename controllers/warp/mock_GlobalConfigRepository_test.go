// Code generated by mockery v2.44.1. DO NOT EDIT.

package warp

import (
	context "context"

	config "github.com/cloudogu/k8s-registry-lib/config"

	mock "github.com/stretchr/testify/mock"

	repository "github.com/cloudogu/k8s-registry-lib/repository"
)

// MockGlobalConfigRepository is an autogenerated mock type for the GlobalConfigRepository type
type MockGlobalConfigRepository struct {
	mock.Mock
}

type MockGlobalConfigRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *MockGlobalConfigRepository) EXPECT() *MockGlobalConfigRepository_Expecter {
	return &MockGlobalConfigRepository_Expecter{mock: &_m.Mock}
}

// Get provides a mock function with given fields: _a0
func (_m *MockGlobalConfigRepository) Get(_a0 context.Context) (config.GlobalConfig, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 config.GlobalConfig
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (config.GlobalConfig, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(context.Context) config.GlobalConfig); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(config.GlobalConfig)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockGlobalConfigRepository_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type MockGlobalConfigRepository_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
//   - _a0 context.Context
func (_e *MockGlobalConfigRepository_Expecter) Get(_a0 interface{}) *MockGlobalConfigRepository_Get_Call {
	return &MockGlobalConfigRepository_Get_Call{Call: _e.mock.On("Get", _a0)}
}

func (_c *MockGlobalConfigRepository_Get_Call) Run(run func(_a0 context.Context)) *MockGlobalConfigRepository_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockGlobalConfigRepository_Get_Call) Return(_a0 config.GlobalConfig, _a1 error) *MockGlobalConfigRepository_Get_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockGlobalConfigRepository_Get_Call) RunAndReturn(run func(context.Context) (config.GlobalConfig, error)) *MockGlobalConfigRepository_Get_Call {
	_c.Call.Return(run)
	return _c
}

// Watch provides a mock function with given fields: _a0, _a1
func (_m *MockGlobalConfigRepository) Watch(_a0 context.Context, _a1 ...config.WatchFilter) (<-chan repository.GlobalConfigWatchResult, error) {
	_va := make([]interface{}, len(_a1))
	for _i := range _a1 {
		_va[_i] = _a1[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Watch")
	}

	var r0 <-chan repository.GlobalConfigWatchResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, ...config.WatchFilter) (<-chan repository.GlobalConfigWatchResult, error)); ok {
		return rf(_a0, _a1...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, ...config.WatchFilter) <-chan repository.GlobalConfigWatchResult); ok {
		r0 = rf(_a0, _a1...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan repository.GlobalConfigWatchResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, ...config.WatchFilter) error); ok {
		r1 = rf(_a0, _a1...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockGlobalConfigRepository_Watch_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Watch'
type MockGlobalConfigRepository_Watch_Call struct {
	*mock.Call
}

// Watch is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 ...config.WatchFilter
func (_e *MockGlobalConfigRepository_Expecter) Watch(_a0 interface{}, _a1 ...interface{}) *MockGlobalConfigRepository_Watch_Call {
	return &MockGlobalConfigRepository_Watch_Call{Call: _e.mock.On("Watch",
		append([]interface{}{_a0}, _a1...)...)}
}

func (_c *MockGlobalConfigRepository_Watch_Call) Run(run func(_a0 context.Context, _a1 ...config.WatchFilter)) *MockGlobalConfigRepository_Watch_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]config.WatchFilter, len(args)-1)
		for i, a := range args[1:] {
			if a != nil {
				variadicArgs[i] = a.(config.WatchFilter)
			}
		}
		run(args[0].(context.Context), variadicArgs...)
	})
	return _c
}

func (_c *MockGlobalConfigRepository_Watch_Call) Return(_a0 <-chan repository.GlobalConfigWatchResult, _a1 error) *MockGlobalConfigRepository_Watch_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockGlobalConfigRepository_Watch_Call) RunAndReturn(run func(context.Context, ...config.WatchFilter) (<-chan repository.GlobalConfigWatchResult, error)) *MockGlobalConfigRepository_Watch_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockGlobalConfigRepository creates a new instance of MockGlobalConfigRepository. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockGlobalConfigRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockGlobalConfigRepository {
	mock := &MockGlobalConfigRepository{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
