// Code generated by mockery v2.42.1. DO NOT EDIT.

package warp

import (
	types "github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	mock "github.com/stretchr/testify/mock"
)

// MockExternalConverter is an autogenerated mock type for the ExternalConverter type
type MockExternalConverter struct {
	mock.Mock
}

type MockExternalConverter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockExternalConverter) EXPECT() *MockExternalConverter_Expecter {
	return &MockExternalConverter_Expecter{mock: &_m.Mock}
}

// ReadAndUnmarshalExternal provides a mock function with given fields: link
func (_m *MockExternalConverter) ReadAndUnmarshalExternal(link string) (types.EntryWithCategory, error) {
	ret := _m.Called(link)

	if len(ret) == 0 {
		panic("no return value specified for ReadAndUnmarshalExternal")
	}

	var r0 types.EntryWithCategory
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (types.EntryWithCategory, error)); ok {
		return rf(link)
	}
	if rf, ok := ret.Get(0).(func(string) types.EntryWithCategory); ok {
		r0 = rf(link)
	} else {
		r0 = ret.Get(0).(types.EntryWithCategory)
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(link)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockExternalConverter_ReadAndUnmarshalExternal_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ReadAndUnmarshalExternal'
type MockExternalConverter_ReadAndUnmarshalExternal_Call struct {
	*mock.Call
}

// ReadAndUnmarshalExternal is a helper method to define mock.On call
//   - link string
func (_e *MockExternalConverter_Expecter) ReadAndUnmarshalExternal(link interface{}) *MockExternalConverter_ReadAndUnmarshalExternal_Call {
	return &MockExternalConverter_ReadAndUnmarshalExternal_Call{Call: _e.mock.On("ReadAndUnmarshalExternal", link)}
}

func (_c *MockExternalConverter_ReadAndUnmarshalExternal_Call) Run(run func(link string)) *MockExternalConverter_ReadAndUnmarshalExternal_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockExternalConverter_ReadAndUnmarshalExternal_Call) Return(_a0 types.EntryWithCategory, _a1 error) *MockExternalConverter_ReadAndUnmarshalExternal_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockExternalConverter_ReadAndUnmarshalExternal_Call) RunAndReturn(run func(string) (types.EntryWithCategory, error)) *MockExternalConverter_ReadAndUnmarshalExternal_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockExternalConverter creates a new instance of MockExternalConverter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockExternalConverter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockExternalConverter {
	mock := &MockExternalConverter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
