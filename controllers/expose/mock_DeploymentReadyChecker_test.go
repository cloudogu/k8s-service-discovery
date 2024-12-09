// Code generated by mockery v2.44.1. DO NOT EDIT.

package expose

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockDeploymentReadyChecker is an autogenerated mock type for the DeploymentReadyChecker type
type MockDeploymentReadyChecker struct {
	mock.Mock
}

type MockDeploymentReadyChecker_Expecter struct {
	mock *mock.Mock
}

func (_m *MockDeploymentReadyChecker) EXPECT() *MockDeploymentReadyChecker_Expecter {
	return &MockDeploymentReadyChecker_Expecter{mock: &_m.Mock}
}

// IsReady provides a mock function with given fields: ctx, deploymentName
func (_m *MockDeploymentReadyChecker) IsReady(ctx context.Context, deploymentName string) (bool, error) {
	ret := _m.Called(ctx, deploymentName)

	if len(ret) == 0 {
		panic("no return value specified for IsReady")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (bool, error)); ok {
		return rf(ctx, deploymentName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(ctx, deploymentName)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, deploymentName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockDeploymentReadyChecker_IsReady_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'IsReady'
type MockDeploymentReadyChecker_IsReady_Call struct {
	*mock.Call
}

// IsReady is a helper method to define mock.On call
//   - ctx context.Context
//   - deploymentName string
func (_e *MockDeploymentReadyChecker_Expecter) IsReady(ctx interface{}, deploymentName interface{}) *MockDeploymentReadyChecker_IsReady_Call {
	return &MockDeploymentReadyChecker_IsReady_Call{Call: _e.mock.On("IsReady", ctx, deploymentName)}
}

func (_c *MockDeploymentReadyChecker_IsReady_Call) Run(run func(ctx context.Context, deploymentName string)) *MockDeploymentReadyChecker_IsReady_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockDeploymentReadyChecker_IsReady_Call) Return(_a0 bool, _a1 error) *MockDeploymentReadyChecker_IsReady_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockDeploymentReadyChecker_IsReady_Call) RunAndReturn(run func(context.Context, string) (bool, error)) *MockDeploymentReadyChecker_IsReady_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockDeploymentReadyChecker creates a new instance of MockDeploymentReadyChecker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockDeploymentReadyChecker(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockDeploymentReadyChecker {
	mock := &MockDeploymentReadyChecker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}