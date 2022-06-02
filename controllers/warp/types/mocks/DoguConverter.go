// Code generated by mockery v2.10.2. DO NOT EDIT.

package mocks

import (
	registry "github.com/cloudogu/cesapp-lib/registry"
	"github.com/cloudogu/k8s-service-discovery/controllers/warp/types"
	mock "github.com/stretchr/testify/mock"
)

// DoguConverter is an autogenerated mock type for the DoguConverter type
type DoguConverter struct {
	mock.Mock
}

// readAndUnmarshalDogu provides a mock function with given fields: _a0, key, tag
func (_m *DoguConverter) ReadAndUnmarshalDogu(_a0 registry.WatchConfigurationContext, key string, tag string) (types.EntryWithCategory, error) {
	ret := _m.Called(_a0, key, tag)

	var r0 types.EntryWithCategory
	if rf, ok := ret.Get(0).(func(registry.WatchConfigurationContext, string, string) types.EntryWithCategory); ok {
		r0 = rf(_a0, key, tag)
	} else {
		r0 = ret.Get(0).(types.EntryWithCategory)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(registry.WatchConfigurationContext, string, string) error); ok {
		r1 = rf(_a0, key, tag)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}