// Code generated by mockery v2.44.1. DO NOT EDIT.

package controllers

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networkingv1 "k8s.io/api/networking/v1"

	types "k8s.io/apimachinery/pkg/types"

	v1 "k8s.io/client-go/applyconfigurations/networking/v1"

	watch "k8s.io/apimachinery/pkg/watch"
)

// mockIngressInterface is an autogenerated mock type for the ingressInterface type
type mockIngressInterface struct {
	mock.Mock
}

type mockIngressInterface_Expecter struct {
	mock *mock.Mock
}

func (_m *mockIngressInterface) EXPECT() *mockIngressInterface_Expecter {
	return &mockIngressInterface_Expecter{mock: &_m.Mock}
}

// Apply provides a mock function with given fields: ctx, ingress, opts
func (_m *mockIngressInterface) Apply(ctx context.Context, ingress *v1.IngressApplyConfiguration, opts metav1.ApplyOptions) (*networkingv1.Ingress, error) {
	ret := _m.Called(ctx, ingress, opts)

	if len(ret) == 0 {
		panic("no return value specified for Apply")
	}

	var r0 *networkingv1.Ingress
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) (*networkingv1.Ingress, error)); ok {
		return rf(ctx, ingress, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) *networkingv1.Ingress); ok {
		r0 = rf(ctx, ingress, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.Ingress)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) error); ok {
		r1 = rf(ctx, ingress, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_Apply_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Apply'
type mockIngressInterface_Apply_Call struct {
	*mock.Call
}

// Apply is a helper method to define mock.On call
//   - ctx context.Context
//   - ingress *v1.IngressApplyConfiguration
//   - opts metav1.ApplyOptions
func (_e *mockIngressInterface_Expecter) Apply(ctx interface{}, ingress interface{}, opts interface{}) *mockIngressInterface_Apply_Call {
	return &mockIngressInterface_Apply_Call{Call: _e.mock.On("Apply", ctx, ingress, opts)}
}

func (_c *mockIngressInterface_Apply_Call) Run(run func(ctx context.Context, ingress *v1.IngressApplyConfiguration, opts metav1.ApplyOptions)) *mockIngressInterface_Apply_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*v1.IngressApplyConfiguration), args[2].(metav1.ApplyOptions))
	})
	return _c
}

func (_c *mockIngressInterface_Apply_Call) Return(result *networkingv1.Ingress, err error) *mockIngressInterface_Apply_Call {
	_c.Call.Return(result, err)
	return _c
}

func (_c *mockIngressInterface_Apply_Call) RunAndReturn(run func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) (*networkingv1.Ingress, error)) *mockIngressInterface_Apply_Call {
	_c.Call.Return(run)
	return _c
}

// ApplyStatus provides a mock function with given fields: ctx, ingress, opts
func (_m *mockIngressInterface) ApplyStatus(ctx context.Context, ingress *v1.IngressApplyConfiguration, opts metav1.ApplyOptions) (*networkingv1.Ingress, error) {
	ret := _m.Called(ctx, ingress, opts)

	if len(ret) == 0 {
		panic("no return value specified for ApplyStatus")
	}

	var r0 *networkingv1.Ingress
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) (*networkingv1.Ingress, error)); ok {
		return rf(ctx, ingress, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) *networkingv1.Ingress); ok {
		r0 = rf(ctx, ingress, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.Ingress)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) error); ok {
		r1 = rf(ctx, ingress, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_ApplyStatus_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ApplyStatus'
type mockIngressInterface_ApplyStatus_Call struct {
	*mock.Call
}

// ApplyStatus is a helper method to define mock.On call
//   - ctx context.Context
//   - ingress *v1.IngressApplyConfiguration
//   - opts metav1.ApplyOptions
func (_e *mockIngressInterface_Expecter) ApplyStatus(ctx interface{}, ingress interface{}, opts interface{}) *mockIngressInterface_ApplyStatus_Call {
	return &mockIngressInterface_ApplyStatus_Call{Call: _e.mock.On("ApplyStatus", ctx, ingress, opts)}
}

func (_c *mockIngressInterface_ApplyStatus_Call) Run(run func(ctx context.Context, ingress *v1.IngressApplyConfiguration, opts metav1.ApplyOptions)) *mockIngressInterface_ApplyStatus_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*v1.IngressApplyConfiguration), args[2].(metav1.ApplyOptions))
	})
	return _c
}

func (_c *mockIngressInterface_ApplyStatus_Call) Return(result *networkingv1.Ingress, err error) *mockIngressInterface_ApplyStatus_Call {
	_c.Call.Return(result, err)
	return _c
}

func (_c *mockIngressInterface_ApplyStatus_Call) RunAndReturn(run func(context.Context, *v1.IngressApplyConfiguration, metav1.ApplyOptions) (*networkingv1.Ingress, error)) *mockIngressInterface_ApplyStatus_Call {
	_c.Call.Return(run)
	return _c
}

// Create provides a mock function with given fields: ctx, ingress, opts
func (_m *mockIngressInterface) Create(ctx context.Context, ingress *networkingv1.Ingress, opts metav1.CreateOptions) (*networkingv1.Ingress, error) {
	ret := _m.Called(ctx, ingress, opts)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 *networkingv1.Ingress
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *networkingv1.Ingress, metav1.CreateOptions) (*networkingv1.Ingress, error)); ok {
		return rf(ctx, ingress, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *networkingv1.Ingress, metav1.CreateOptions) *networkingv1.Ingress); ok {
		r0 = rf(ctx, ingress, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.Ingress)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *networkingv1.Ingress, metav1.CreateOptions) error); ok {
		r1 = rf(ctx, ingress, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type mockIngressInterface_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - ingress *networkingv1.Ingress
//   - opts metav1.CreateOptions
func (_e *mockIngressInterface_Expecter) Create(ctx interface{}, ingress interface{}, opts interface{}) *mockIngressInterface_Create_Call {
	return &mockIngressInterface_Create_Call{Call: _e.mock.On("Create", ctx, ingress, opts)}
}

func (_c *mockIngressInterface_Create_Call) Run(run func(ctx context.Context, ingress *networkingv1.Ingress, opts metav1.CreateOptions)) *mockIngressInterface_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*networkingv1.Ingress), args[2].(metav1.CreateOptions))
	})
	return _c
}

func (_c *mockIngressInterface_Create_Call) Return(_a0 *networkingv1.Ingress, _a1 error) *mockIngressInterface_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockIngressInterface_Create_Call) RunAndReturn(run func(context.Context, *networkingv1.Ingress, metav1.CreateOptions) (*networkingv1.Ingress, error)) *mockIngressInterface_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Delete provides a mock function with given fields: ctx, name, opts
func (_m *mockIngressInterface) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	ret := _m.Called(ctx, name, opts)

	if len(ret) == 0 {
		panic("no return value specified for Delete")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, metav1.DeleteOptions) error); ok {
		r0 = rf(ctx, name, opts)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// mockIngressInterface_Delete_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Delete'
type mockIngressInterface_Delete_Call struct {
	*mock.Call
}

// Delete is a helper method to define mock.On call
//   - ctx context.Context
//   - name string
//   - opts metav1.DeleteOptions
func (_e *mockIngressInterface_Expecter) Delete(ctx interface{}, name interface{}, opts interface{}) *mockIngressInterface_Delete_Call {
	return &mockIngressInterface_Delete_Call{Call: _e.mock.On("Delete", ctx, name, opts)}
}

func (_c *mockIngressInterface_Delete_Call) Run(run func(ctx context.Context, name string, opts metav1.DeleteOptions)) *mockIngressInterface_Delete_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(metav1.DeleteOptions))
	})
	return _c
}

func (_c *mockIngressInterface_Delete_Call) Return(_a0 error) *mockIngressInterface_Delete_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockIngressInterface_Delete_Call) RunAndReturn(run func(context.Context, string, metav1.DeleteOptions) error) *mockIngressInterface_Delete_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteCollection provides a mock function with given fields: ctx, opts, listOpts
func (_m *mockIngressInterface) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	ret := _m.Called(ctx, opts, listOpts)

	if len(ret) == 0 {
		panic("no return value specified for DeleteCollection")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, metav1.DeleteOptions, metav1.ListOptions) error); ok {
		r0 = rf(ctx, opts, listOpts)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// mockIngressInterface_DeleteCollection_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteCollection'
type mockIngressInterface_DeleteCollection_Call struct {
	*mock.Call
}

// DeleteCollection is a helper method to define mock.On call
//   - ctx context.Context
//   - opts metav1.DeleteOptions
//   - listOpts metav1.ListOptions
func (_e *mockIngressInterface_Expecter) DeleteCollection(ctx interface{}, opts interface{}, listOpts interface{}) *mockIngressInterface_DeleteCollection_Call {
	return &mockIngressInterface_DeleteCollection_Call{Call: _e.mock.On("DeleteCollection", ctx, opts, listOpts)}
}

func (_c *mockIngressInterface_DeleteCollection_Call) Run(run func(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions)) *mockIngressInterface_DeleteCollection_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(metav1.DeleteOptions), args[2].(metav1.ListOptions))
	})
	return _c
}

func (_c *mockIngressInterface_DeleteCollection_Call) Return(_a0 error) *mockIngressInterface_DeleteCollection_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockIngressInterface_DeleteCollection_Call) RunAndReturn(run func(context.Context, metav1.DeleteOptions, metav1.ListOptions) error) *mockIngressInterface_DeleteCollection_Call {
	_c.Call.Return(run)
	return _c
}

// Get provides a mock function with given fields: ctx, name, opts
func (_m *mockIngressInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*networkingv1.Ingress, error) {
	ret := _m.Called(ctx, name, opts)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 *networkingv1.Ingress
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, metav1.GetOptions) (*networkingv1.Ingress, error)); ok {
		return rf(ctx, name, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, metav1.GetOptions) *networkingv1.Ingress); ok {
		r0 = rf(ctx, name, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.Ingress)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, metav1.GetOptions) error); ok {
		r1 = rf(ctx, name, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type mockIngressInterface_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
//   - ctx context.Context
//   - name string
//   - opts metav1.GetOptions
func (_e *mockIngressInterface_Expecter) Get(ctx interface{}, name interface{}, opts interface{}) *mockIngressInterface_Get_Call {
	return &mockIngressInterface_Get_Call{Call: _e.mock.On("Get", ctx, name, opts)}
}

func (_c *mockIngressInterface_Get_Call) Run(run func(ctx context.Context, name string, opts metav1.GetOptions)) *mockIngressInterface_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(metav1.GetOptions))
	})
	return _c
}

func (_c *mockIngressInterface_Get_Call) Return(_a0 *networkingv1.Ingress, _a1 error) *mockIngressInterface_Get_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockIngressInterface_Get_Call) RunAndReturn(run func(context.Context, string, metav1.GetOptions) (*networkingv1.Ingress, error)) *mockIngressInterface_Get_Call {
	_c.Call.Return(run)
	return _c
}

// List provides a mock function with given fields: ctx, opts
func (_m *mockIngressInterface) List(ctx context.Context, opts metav1.ListOptions) (*networkingv1.IngressList, error) {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for List")
	}

	var r0 *networkingv1.IngressList
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, metav1.ListOptions) (*networkingv1.IngressList, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, metav1.ListOptions) *networkingv1.IngressList); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.IngressList)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, metav1.ListOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_List_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'List'
type mockIngressInterface_List_Call struct {
	*mock.Call
}

// List is a helper method to define mock.On call
//   - ctx context.Context
//   - opts metav1.ListOptions
func (_e *mockIngressInterface_Expecter) List(ctx interface{}, opts interface{}) *mockIngressInterface_List_Call {
	return &mockIngressInterface_List_Call{Call: _e.mock.On("List", ctx, opts)}
}

func (_c *mockIngressInterface_List_Call) Run(run func(ctx context.Context, opts metav1.ListOptions)) *mockIngressInterface_List_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(metav1.ListOptions))
	})
	return _c
}

func (_c *mockIngressInterface_List_Call) Return(_a0 *networkingv1.IngressList, _a1 error) *mockIngressInterface_List_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockIngressInterface_List_Call) RunAndReturn(run func(context.Context, metav1.ListOptions) (*networkingv1.IngressList, error)) *mockIngressInterface_List_Call {
	_c.Call.Return(run)
	return _c
}

// Patch provides a mock function with given fields: ctx, name, pt, data, opts, subresources
func (_m *mockIngressInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*networkingv1.Ingress, error) {
	_va := make([]interface{}, len(subresources))
	for _i := range subresources {
		_va[_i] = subresources[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, name, pt, data, opts)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Patch")
	}

	var r0 *networkingv1.Ingress
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*networkingv1.Ingress, error)); ok {
		return rf(ctx, name, pt, data, opts, subresources...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) *networkingv1.Ingress); ok {
		r0 = rf(ctx, name, pt, data, opts, subresources...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.Ingress)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) error); ok {
		r1 = rf(ctx, name, pt, data, opts, subresources...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_Patch_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Patch'
type mockIngressInterface_Patch_Call struct {
	*mock.Call
}

// Patch is a helper method to define mock.On call
//   - ctx context.Context
//   - name string
//   - pt types.PatchType
//   - data []byte
//   - opts metav1.PatchOptions
//   - subresources ...string
func (_e *mockIngressInterface_Expecter) Patch(ctx interface{}, name interface{}, pt interface{}, data interface{}, opts interface{}, subresources ...interface{}) *mockIngressInterface_Patch_Call {
	return &mockIngressInterface_Patch_Call{Call: _e.mock.On("Patch",
		append([]interface{}{ctx, name, pt, data, opts}, subresources...)...)}
}

func (_c *mockIngressInterface_Patch_Call) Run(run func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string)) *mockIngressInterface_Patch_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]string, len(args)-5)
		for i, a := range args[5:] {
			if a != nil {
				variadicArgs[i] = a.(string)
			}
		}
		run(args[0].(context.Context), args[1].(string), args[2].(types.PatchType), args[3].([]byte), args[4].(metav1.PatchOptions), variadicArgs...)
	})
	return _c
}

func (_c *mockIngressInterface_Patch_Call) Return(result *networkingv1.Ingress, err error) *mockIngressInterface_Patch_Call {
	_c.Call.Return(result, err)
	return _c
}

func (_c *mockIngressInterface_Patch_Call) RunAndReturn(run func(context.Context, string, types.PatchType, []byte, metav1.PatchOptions, ...string) (*networkingv1.Ingress, error)) *mockIngressInterface_Patch_Call {
	_c.Call.Return(run)
	return _c
}

// Update provides a mock function with given fields: ctx, ingress, opts
func (_m *mockIngressInterface) Update(ctx context.Context, ingress *networkingv1.Ingress, opts metav1.UpdateOptions) (*networkingv1.Ingress, error) {
	ret := _m.Called(ctx, ingress, opts)

	if len(ret) == 0 {
		panic("no return value specified for Update")
	}

	var r0 *networkingv1.Ingress
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) (*networkingv1.Ingress, error)); ok {
		return rf(ctx, ingress, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) *networkingv1.Ingress); ok {
		r0 = rf(ctx, ingress, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.Ingress)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) error); ok {
		r1 = rf(ctx, ingress, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_Update_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Update'
type mockIngressInterface_Update_Call struct {
	*mock.Call
}

// Update is a helper method to define mock.On call
//   - ctx context.Context
//   - ingress *networkingv1.Ingress
//   - opts metav1.UpdateOptions
func (_e *mockIngressInterface_Expecter) Update(ctx interface{}, ingress interface{}, opts interface{}) *mockIngressInterface_Update_Call {
	return &mockIngressInterface_Update_Call{Call: _e.mock.On("Update", ctx, ingress, opts)}
}

func (_c *mockIngressInterface_Update_Call) Run(run func(ctx context.Context, ingress *networkingv1.Ingress, opts metav1.UpdateOptions)) *mockIngressInterface_Update_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*networkingv1.Ingress), args[2].(metav1.UpdateOptions))
	})
	return _c
}

func (_c *mockIngressInterface_Update_Call) Return(_a0 *networkingv1.Ingress, _a1 error) *mockIngressInterface_Update_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockIngressInterface_Update_Call) RunAndReturn(run func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) (*networkingv1.Ingress, error)) *mockIngressInterface_Update_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateStatus provides a mock function with given fields: ctx, ingress, opts
func (_m *mockIngressInterface) UpdateStatus(ctx context.Context, ingress *networkingv1.Ingress, opts metav1.UpdateOptions) (*networkingv1.Ingress, error) {
	ret := _m.Called(ctx, ingress, opts)

	if len(ret) == 0 {
		panic("no return value specified for UpdateStatus")
	}

	var r0 *networkingv1.Ingress
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) (*networkingv1.Ingress, error)); ok {
		return rf(ctx, ingress, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) *networkingv1.Ingress); ok {
		r0 = rf(ctx, ingress, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*networkingv1.Ingress)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) error); ok {
		r1 = rf(ctx, ingress, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_UpdateStatus_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateStatus'
type mockIngressInterface_UpdateStatus_Call struct {
	*mock.Call
}

// UpdateStatus is a helper method to define mock.On call
//   - ctx context.Context
//   - ingress *networkingv1.Ingress
//   - opts metav1.UpdateOptions
func (_e *mockIngressInterface_Expecter) UpdateStatus(ctx interface{}, ingress interface{}, opts interface{}) *mockIngressInterface_UpdateStatus_Call {
	return &mockIngressInterface_UpdateStatus_Call{Call: _e.mock.On("UpdateStatus", ctx, ingress, opts)}
}

func (_c *mockIngressInterface_UpdateStatus_Call) Run(run func(ctx context.Context, ingress *networkingv1.Ingress, opts metav1.UpdateOptions)) *mockIngressInterface_UpdateStatus_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*networkingv1.Ingress), args[2].(metav1.UpdateOptions))
	})
	return _c
}

func (_c *mockIngressInterface_UpdateStatus_Call) Return(_a0 *networkingv1.Ingress, _a1 error) *mockIngressInterface_UpdateStatus_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockIngressInterface_UpdateStatus_Call) RunAndReturn(run func(context.Context, *networkingv1.Ingress, metav1.UpdateOptions) (*networkingv1.Ingress, error)) *mockIngressInterface_UpdateStatus_Call {
	_c.Call.Return(run)
	return _c
}

// Watch provides a mock function with given fields: ctx, opts
func (_m *mockIngressInterface) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for Watch")
	}

	var r0 watch.Interface
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, metav1.ListOptions) (watch.Interface, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, metav1.ListOptions) watch.Interface); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(watch.Interface)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, metav1.ListOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockIngressInterface_Watch_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Watch'
type mockIngressInterface_Watch_Call struct {
	*mock.Call
}

// Watch is a helper method to define mock.On call
//   - ctx context.Context
//   - opts metav1.ListOptions
func (_e *mockIngressInterface_Expecter) Watch(ctx interface{}, opts interface{}) *mockIngressInterface_Watch_Call {
	return &mockIngressInterface_Watch_Call{Call: _e.mock.On("Watch", ctx, opts)}
}

func (_c *mockIngressInterface_Watch_Call) Run(run func(ctx context.Context, opts metav1.ListOptions)) *mockIngressInterface_Watch_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(metav1.ListOptions))
	})
	return _c
}

func (_c *mockIngressInterface_Watch_Call) Return(_a0 watch.Interface, _a1 error) *mockIngressInterface_Watch_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockIngressInterface_Watch_Call) RunAndReturn(run func(context.Context, metav1.ListOptions) (watch.Interface, error)) *mockIngressInterface_Watch_Call {
	_c.Call.Return(run)
	return _c
}

// newMockIngressInterface creates a new instance of mockIngressInterface. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockIngressInterface(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockIngressInterface {
	mock := &mockIngressInterface{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
