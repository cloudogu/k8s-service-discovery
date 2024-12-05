// Code generated by mockery v2.44.1. DO NOT EDIT.

package expose

import mock "github.com/stretchr/testify/mock"

// mockIngressController is an autogenerated mock type for the ingressController type
type mockIngressController struct {
	mock.Mock
}

type mockIngressController_Expecter struct {
	mock *mock.Mock
}

func (_m *mockIngressController) EXPECT() *mockIngressController_Expecter {
	return &mockIngressController_Expecter{mock: &_m.Mock}
}

// GetAdditionalConfigurationKey provides a mock function with given fields:
func (_m *mockIngressController) GetAdditionalConfigurationKey() string {
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

// mockIngressController_GetAdditionalConfigurationKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetAdditionalConfigurationKey'
type mockIngressController_GetAdditionalConfigurationKey_Call struct {
	*mock.Call
}

// GetAdditionalConfigurationKey is a helper method to define mock.On call
func (_e *mockIngressController_Expecter) GetAdditionalConfigurationKey() *mockIngressController_GetAdditionalConfigurationKey_Call {
	return &mockIngressController_GetAdditionalConfigurationKey_Call{Call: _e.mock.On("GetAdditionalConfigurationKey")}
}

func (_c *mockIngressController_GetAdditionalConfigurationKey_Call) Run(run func()) *mockIngressController_GetAdditionalConfigurationKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockIngressController_GetAdditionalConfigurationKey_Call) Return(_a0 string) *mockIngressController_GetAdditionalConfigurationKey_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockIngressController_GetAdditionalConfigurationKey_Call) RunAndReturn(run func() string) *mockIngressController_GetAdditionalConfigurationKey_Call {
	_c.Call.Return(run)
	return _c
}

// GetControllerSpec provides a mock function with given fields:
func (_m *mockIngressController) GetControllerSpec() string {
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

// mockIngressController_GetControllerSpec_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetControllerSpec'
type mockIngressController_GetControllerSpec_Call struct {
	*mock.Call
}

// GetControllerSpec is a helper method to define mock.On call
func (_e *mockIngressController_Expecter) GetControllerSpec() *mockIngressController_GetControllerSpec_Call {
	return &mockIngressController_GetControllerSpec_Call{Call: _e.mock.On("GetControllerSpec")}
}

func (_c *mockIngressController_GetControllerSpec_Call) Run(run func()) *mockIngressController_GetControllerSpec_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockIngressController_GetControllerSpec_Call) Return(_a0 string) *mockIngressController_GetControllerSpec_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockIngressController_GetControllerSpec_Call) RunAndReturn(run func() string) *mockIngressController_GetControllerSpec_Call {
	_c.Call.Return(run)
	return _c
}

// GetRewriteAnnotationKey provides a mock function with given fields:
func (_m *mockIngressController) GetRewriteAnnotationKey() string {
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

// mockIngressController_GetRewriteAnnotationKey_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRewriteAnnotationKey'
type mockIngressController_GetRewriteAnnotationKey_Call struct {
	*mock.Call
}

// GetRewriteAnnotationKey is a helper method to define mock.On call
func (_e *mockIngressController_Expecter) GetRewriteAnnotationKey() *mockIngressController_GetRewriteAnnotationKey_Call {
	return &mockIngressController_GetRewriteAnnotationKey_Call{Call: _e.mock.On("GetRewriteAnnotationKey")}
}

func (_c *mockIngressController_GetRewriteAnnotationKey_Call) Run(run func()) *mockIngressController_GetRewriteAnnotationKey_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockIngressController_GetRewriteAnnotationKey_Call) Return(_a0 string) *mockIngressController_GetRewriteAnnotationKey_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockIngressController_GetRewriteAnnotationKey_Call) RunAndReturn(run func() string) *mockIngressController_GetRewriteAnnotationKey_Call {
	_c.Call.Return(run)
	return _c
}

// newMockIngressController creates a new instance of mockIngressController. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockIngressController(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockIngressController {
	mock := &mockIngressController{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
