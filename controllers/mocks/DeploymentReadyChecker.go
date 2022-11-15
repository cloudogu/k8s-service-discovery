// Code generated by mockery v2.14.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// DeploymentReadyChecker is an autogenerated mock type for the DeploymentReadyChecker type
type DeploymentReadyChecker struct {
	mock.Mock
}

// IsReady provides a mock function with given fields: ctx, deploymentName
func (_m *DeploymentReadyChecker) IsReady(ctx context.Context, deploymentName string) (bool, error) {
	ret := _m.Called(ctx, deploymentName)

	var r0 bool
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(ctx, deploymentName)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, deploymentName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewDeploymentReadyChecker interface {
	mock.TestingT
	Cleanup(func())
}

// NewDeploymentReadyChecker creates a new instance of DeploymentReadyChecker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewDeploymentReadyChecker(t mockConstructorTestingTNewDeploymentReadyChecker) *DeploymentReadyChecker {
	mock := &DeploymentReadyChecker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
