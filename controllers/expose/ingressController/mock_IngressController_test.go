// Code generated by mockery v2.42.1. DO NOT EDIT.

package ingressController

import (
	context "context"

	util "github.com/cloudogu/k8s-service-discovery/controllers/util"
	mock "github.com/stretchr/testify/mock"
)

// MockIngressController is an autogenerated mock type for the IngressController type
type MockIngressController struct {
	mock.Mock
}

type MockIngressController_Expecter struct {
	mock *mock.Mock
}

func (_m *MockIngressController) EXPECT() *MockIngressController_Expecter {
	return &MockIngressController_Expecter{mock: &_m.Mock}
}

// DeleteExposedPorts provides a mock function with given fields: ctx, namespace, targetServiceName
func (_m *MockIngressController) DeleteExposedPorts(ctx context.Context, namespace string, targetServiceName string) error {
	ret := _m.Called(ctx, namespace, targetServiceName)

	if len(ret) == 0 {
		panic("no return value specified for DeleteExposedPorts")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, namespace, targetServiceName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockIngressController_DeleteExposedPorts_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteExposedPorts'
type MockIngressController_DeleteExposedPorts_Call struct {
	*mock.Call
}

// DeleteExposedPorts is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - targetServiceName string
func (_e *MockIngressController_Expecter) DeleteExposedPorts(ctx interface{}, namespace interface{}, targetServiceName interface{}) *MockIngressController_DeleteExposedPorts_Call {
	return &MockIngressController_DeleteExposedPorts_Call{Call: _e.mock.On("DeleteExposedPorts", ctx, namespace, targetServiceName)}
}

func (_c *MockIngressController_DeleteExposedPorts_Call) Run(run func(ctx context.Context, namespace string, targetServiceName string)) *MockIngressController_DeleteExposedPorts_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string))
	})
	return _c
}

func (_c *MockIngressController_DeleteExposedPorts_Call) Return(_a0 error) *MockIngressController_DeleteExposedPorts_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_DeleteExposedPorts_Call) RunAndReturn(run func(context.Context, string, string) error) *MockIngressController_DeleteExposedPorts_Call {
	_c.Call.Return(run)
	return _c
}

// ExposeOrUpdateExposedPorts provides a mock function with given fields: ctx, namespace, targetServiceName, exposedPorts
func (_m *MockIngressController) ExposeOrUpdateExposedPorts(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts) error {
	ret := _m.Called(ctx, namespace, targetServiceName, exposedPorts)

	if len(ret) == 0 {
		panic("no return value specified for ExposeOrUpdateExposedPorts")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, util.ExposedPorts) error); ok {
		r0 = rf(ctx, namespace, targetServiceName, exposedPorts)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockIngressController_ExposeOrUpdateExposedPorts_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ExposeOrUpdateExposedPorts'
type MockIngressController_ExposeOrUpdateExposedPorts_Call struct {
	*mock.Call
}

// ExposeOrUpdateExposedPorts is a helper method to define mock.On call
//   - ctx context.Context
//   - namespace string
//   - targetServiceName string
//   - exposedPorts util.ExposedPorts
func (_e *MockIngressController_Expecter) ExposeOrUpdateExposedPorts(ctx interface{}, namespace interface{}, targetServiceName interface{}, exposedPorts interface{}) *MockIngressController_ExposeOrUpdateExposedPorts_Call {
	return &MockIngressController_ExposeOrUpdateExposedPorts_Call{Call: _e.mock.On("ExposeOrUpdateExposedPorts", ctx, namespace, targetServiceName, exposedPorts)}
}

func (_c *MockIngressController_ExposeOrUpdateExposedPorts_Call) Run(run func(ctx context.Context, namespace string, targetServiceName string, exposedPorts util.ExposedPorts)) *MockIngressController_ExposeOrUpdateExposedPorts_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(string), args[3].(util.ExposedPorts))
	})
	return _c
}

func (_c *MockIngressController_ExposeOrUpdateExposedPorts_Call) Return(_a0 error) *MockIngressController_ExposeOrUpdateExposedPorts_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_ExposeOrUpdateExposedPorts_Call) RunAndReturn(run func(context.Context, string, string, util.ExposedPorts) error) *MockIngressController_ExposeOrUpdateExposedPorts_Call {
	_c.Call.Return(run)
	return _c
}

// GetAdditionalConfigurationKey provides a mock function with given fields:
func (_m *MockIngressController) GetAdditionalConfigurationKey() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetAdditionalConfigurationKey")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockIngressController_GetAdditionalConfigurationKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetAdditionalConfigurationKey'
type MockIngressController_GetAdditionalConfigurationKey_Call struct {
	*mock.Call
}

// GetAdditionalConfigurationKey is a helper method to define mock.On call
func (_e *MockIngressController_Expecter) GetAdditionalConfigurationKey() *MockIngressController_GetAdditionalConfigurationKey_Call {
	return &MockIngressController_GetAdditionalConfigurationKey_Call{Call: _e.mock.On("GetAdditionalConfigurationKey")}
}

func (_c *MockIngressController_GetAdditionalConfigurationKey_Call) Run(run func()) *MockIngressController_GetAdditionalConfigurationKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockIngressController_GetAdditionalConfigurationKey_Call) Return(_a0 string) *MockIngressController_GetAdditionalConfigurationKey_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_GetAdditionalConfigurationKey_Call) RunAndReturn(run func() string) *MockIngressController_GetAdditionalConfigurationKey_Call {
	_c.Call.Return(run)
	return _c
}

// GetControllerSpec provides a mock function with given fields:
func (_m *MockIngressController) GetControllerSpec() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetControllerSpec")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockIngressController_GetControllerSpec_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetControllerSpec'
type MockIngressController_GetControllerSpec_Call struct {
	*mock.Call
}

// GetControllerSpec is a helper method to define mock.On call
func (_e *MockIngressController_Expecter) GetControllerSpec() *MockIngressController_GetControllerSpec_Call {
	return &MockIngressController_GetControllerSpec_Call{Call: _e.mock.On("GetControllerSpec")}
}

func (_c *MockIngressController_GetControllerSpec_Call) Run(run func()) *MockIngressController_GetControllerSpec_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockIngressController_GetControllerSpec_Call) Return(_a0 string) *MockIngressController_GetControllerSpec_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_GetControllerSpec_Call) RunAndReturn(run func() string) *MockIngressController_GetControllerSpec_Call {
	_c.Call.Return(run)
	return _c
}

// GetName provides a mock function with given fields:
func (_m *MockIngressController) GetName() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetName")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockIngressController_GetName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetName'
type MockIngressController_GetName_Call struct {
	*mock.Call
}

// GetName is a helper method to define mock.On call
func (_e *MockIngressController_Expecter) GetName() *MockIngressController_GetName_Call {
	return &MockIngressController_GetName_Call{Call: _e.mock.On("GetName")}
}

func (_c *MockIngressController_GetName_Call) Run(run func()) *MockIngressController_GetName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockIngressController_GetName_Call) Return(_a0 string) *MockIngressController_GetName_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_GetName_Call) RunAndReturn(run func() string) *MockIngressController_GetName_Call {
	_c.Call.Return(run)
	return _c
}

// GetProxyBodySizeKey provides a mock function with given fields:
func (_m *MockIngressController) GetProxyBodySizeKey() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetProxyBodySizeKey")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockIngressController_GetProxyBodySizeKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetProxyBodySizeKey'
type MockIngressController_GetProxyBodySizeKey_Call struct {
	*mock.Call
}

// GetProxyBodySizeKey is a helper method to define mock.On call
func (_e *MockIngressController_Expecter) GetProxyBodySizeKey() *MockIngressController_GetProxyBodySizeKey_Call {
	return &MockIngressController_GetProxyBodySizeKey_Call{Call: _e.mock.On("GetProxyBodySizeKey")}
}

func (_c *MockIngressController_GetProxyBodySizeKey_Call) Run(run func()) *MockIngressController_GetProxyBodySizeKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockIngressController_GetProxyBodySizeKey_Call) Return(_a0 string) *MockIngressController_GetProxyBodySizeKey_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_GetProxyBodySizeKey_Call) RunAndReturn(run func() string) *MockIngressController_GetProxyBodySizeKey_Call {
	_c.Call.Return(run)
	return _c
}

// GetRewriteAnnotationKey provides a mock function with given fields:
func (_m *MockIngressController) GetRewriteAnnotationKey() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetRewriteAnnotationKey")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockIngressController_GetRewriteAnnotationKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRewriteAnnotationKey'
type MockIngressController_GetRewriteAnnotationKey_Call struct {
	*mock.Call
}

// GetRewriteAnnotationKey is a helper method to define mock.On call
func (_e *MockIngressController_Expecter) GetRewriteAnnotationKey() *MockIngressController_GetRewriteAnnotationKey_Call {
	return &MockIngressController_GetRewriteAnnotationKey_Call{Call: _e.mock.On("GetRewriteAnnotationKey")}
}

func (_c *MockIngressController_GetRewriteAnnotationKey_Call) Run(run func()) *MockIngressController_GetRewriteAnnotationKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockIngressController_GetRewriteAnnotationKey_Call) Return(_a0 string) *MockIngressController_GetRewriteAnnotationKey_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_GetRewriteAnnotationKey_Call) RunAndReturn(run func() string) *MockIngressController_GetRewriteAnnotationKey_Call {
	_c.Call.Return(run)
	return _c
}

// GetUseRegexKey provides a mock function with given fields:
func (_m *MockIngressController) GetUseRegexKey() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetUseRegexKey")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockIngressController_GetUseRegexKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetUseRegexKey'
type MockIngressController_GetUseRegexKey_Call struct {
	*mock.Call
}

// GetUseRegexKey is a helper method to define mock.On call
func (_e *MockIngressController_Expecter) GetUseRegexKey() *MockIngressController_GetUseRegexKey_Call {
	return &MockIngressController_GetUseRegexKey_Call{Call: _e.mock.On("GetUseRegexKey")}
}

func (_c *MockIngressController_GetUseRegexKey_Call) Run(run func()) *MockIngressController_GetUseRegexKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockIngressController_GetUseRegexKey_Call) Return(_a0 string) *MockIngressController_GetUseRegexKey_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockIngressController_GetUseRegexKey_Call) RunAndReturn(run func() string) *MockIngressController_GetUseRegexKey_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockIngressController creates a new instance of MockIngressController. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockIngressController(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockIngressController {
	mock := &MockIngressController{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
