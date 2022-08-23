// Code generated by mockery v2.14.0. DO NOT EDIT.

package mocks

import (
	logr "github.com/go-logr/logr"
	mock "github.com/stretchr/testify/mock"
)

// LogSink is an autogenerated mock type for the LogSink type
type LogSink struct {
	mock.Mock
}

// Enabled provides a mock function with given fields: level
func (_m *LogSink) Enabled(level int) bool {
	ret := _m.Called(level)

	var r0 bool
	if rf, ok := ret.Get(0).(func(int) bool); ok {
		r0 = rf(level)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Error provides a mock function with given fields: err, msg, keysAndValues
func (_m *LogSink) Error(err error, msg string, keysAndValues ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, err, msg)
	_ca = append(_ca, keysAndValues...)
	_m.Called(_ca...)
}

// Info provides a mock function with given fields: level, msg, keysAndValues
func (_m *LogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, level, msg)
	_ca = append(_ca, keysAndValues...)
	_m.Called(_ca...)
}

// Init provides a mock function with given fields: info
func (_m *LogSink) Init(info logr.RuntimeInfo) {
	_m.Called(info)
}

// WithName provides a mock function with given fields: name
func (_m *LogSink) WithName(name string) logr.LogSink {
	ret := _m.Called(name)

	var r0 logr.LogSink
	if rf, ok := ret.Get(0).(func(string) logr.LogSink); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(logr.LogSink)
		}
	}

	return r0
}

// WithValues provides a mock function with given fields: keysAndValues
func (_m *LogSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	var _ca []interface{}
	_ca = append(_ca, keysAndValues...)
	ret := _m.Called(_ca...)

	var r0 logr.LogSink
	if rf, ok := ret.Get(0).(func(...interface{}) logr.LogSink); ok {
		r0 = rf(keysAndValues...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(logr.LogSink)
		}
	}

	return r0
}

type mockConstructorTestingTNewLogSink interface {
	mock.TestingT
	Cleanup(func())
}

// NewLogSink creates a new instance of LogSink. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewLogSink(t mockConstructorTestingTNewLogSink) *LogSink {
	mock := &LogSink{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
