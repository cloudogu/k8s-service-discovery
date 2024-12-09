// Code generated by mockery v2.44.1. DO NOT EDIT.

package controllers

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
)

// MockExposedPortUpdater is an autogenerated mock type for the ExposedPortUpdater type
type MockExposedPortUpdater struct {
	mock.Mock
}

type MockExposedPortUpdater_Expecter struct {
	mock *mock.Mock
}

func (_m *MockExposedPortUpdater) EXPECT() *MockExposedPortUpdater_Expecter {
	return &MockExposedPortUpdater_Expecter{mock: &_m.Mock}
}

// RemoveExposedPorts provides a mock function with given fields: ctx, serviceName
func (_m *MockExposedPortUpdater) RemoveExposedPorts(ctx context.Context, serviceName string) error {
	ret := _m.Called(ctx, serviceName)

	if len(ret) == 0 {
		panic("no return value specified for RemoveExposedPorts")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, serviceName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockExposedPortUpdater_RemoveExposedPorts_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RemoveExposedPorts'
type MockExposedPortUpdater_RemoveExposedPorts_Call struct {
	*mock.Call
}

// RemoveExposedPorts is a helper method to define mock.On call
//   - ctx context.Context
//   - serviceName string
func (_e *MockExposedPortUpdater_Expecter) RemoveExposedPorts(ctx interface{}, serviceName interface{}) *MockExposedPortUpdater_RemoveExposedPorts_Call {
	return &MockExposedPortUpdater_RemoveExposedPorts_Call{Call: _e.mock.On("RemoveExposedPorts", ctx, serviceName)}
}

func (_c *MockExposedPortUpdater_RemoveExposedPorts_Call) Run(run func(ctx context.Context, serviceName string)) *MockExposedPortUpdater_RemoveExposedPorts_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockExposedPortUpdater_RemoveExposedPorts_Call) Return(_a0 error) *MockExposedPortUpdater_RemoveExposedPorts_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockExposedPortUpdater_RemoveExposedPorts_Call) RunAndReturn(run func(context.Context, string) error) *MockExposedPortUpdater_RemoveExposedPorts_Call {
	_c.Call.Return(run)
	return _c
}

// UpsertCesLoadbalancerService provides a mock function with given fields: ctx, service
func (_m *MockExposedPortUpdater) UpsertCesLoadbalancerService(ctx context.Context, service *v1.Service) error {
	ret := _m.Called(ctx, service)

	if len(ret) == 0 {
		panic("no return value specified for UpsertCesLoadbalancerService")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.Service) error); ok {
		r0 = rf(ctx, service)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockExposedPortUpdater_UpsertCesLoadbalancerService_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpsertCesLoadbalancerService'
type MockExposedPortUpdater_UpsertCesLoadbalancerService_Call struct {
	*mock.Call
}

// UpsertCesLoadbalancerService is a helper method to define mock.On call
//   - ctx context.Context
//   - service *v1.Service
func (_e *MockExposedPortUpdater_Expecter) UpsertCesLoadbalancerService(ctx interface{}, service interface{}) *MockExposedPortUpdater_UpsertCesLoadbalancerService_Call {
	return &MockExposedPortUpdater_UpsertCesLoadbalancerService_Call{Call: _e.mock.On("UpsertCesLoadbalancerService", ctx, service)}
}

func (_c *MockExposedPortUpdater_UpsertCesLoadbalancerService_Call) Run(run func(ctx context.Context, service *v1.Service)) *MockExposedPortUpdater_UpsertCesLoadbalancerService_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*v1.Service))
	})
	return _c
}

func (_c *MockExposedPortUpdater_UpsertCesLoadbalancerService_Call) Return(_a0 error) *MockExposedPortUpdater_UpsertCesLoadbalancerService_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockExposedPortUpdater_UpsertCesLoadbalancerService_Call) RunAndReturn(run func(context.Context, *v1.Service) error) *MockExposedPortUpdater_UpsertCesLoadbalancerService_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockExposedPortUpdater creates a new instance of MockExposedPortUpdater. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockExposedPortUpdater(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockExposedPortUpdater {
	mock := &MockExposedPortUpdater{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
