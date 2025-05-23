// Code generated by mockery v2.20.0. DO NOT EDIT.

package ssl

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// mockSelfSignedCertificateCreator is an autogenerated mock type for the selfSignedCertificateCreator type
type mockSelfSignedCertificateCreator struct {
	mock.Mock
}

type mockSelfSignedCertificateCreator_Expecter struct {
	mock *mock.Mock
}

func (_m *mockSelfSignedCertificateCreator) EXPECT() *mockSelfSignedCertificateCreator_Expecter {
	return &mockSelfSignedCertificateCreator_Expecter{mock: &_m.Mock}
}

// CreateAndSafeCertificate provides a mock function with given fields: ctx, certExpireDays, country, province, locality, altDNSNames
func (_m *mockSelfSignedCertificateCreator) CreateAndSafeCertificate(ctx context.Context, certExpireDays int, country string, province string, locality string, altDNSNames []string) error {
	ret := _m.Called(ctx, certExpireDays, country, province, locality, altDNSNames)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int, string, string, string, []string) error); ok {
		r0 = rf(ctx, certExpireDays, country, province, locality, altDNSNames)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CreateAndSafeCertificate'
type mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call struct {
	*mock.Call
}

// CreateAndSafeCertificate is a helper method to define mock.On call
//   - ctx context.Context
//   - certExpireDays int
//   - country string
//   - province string
//   - locality string
//   - altDNSNames []string
func (_e *mockSelfSignedCertificateCreator_Expecter) CreateAndSafeCertificate(ctx interface{}, certExpireDays interface{}, country interface{}, province interface{}, locality interface{}, altDNSNames interface{}) *mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call {
	return &mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call{Call: _e.mock.On("CreateAndSafeCertificate", ctx, certExpireDays, country, province, locality, altDNSNames)}
}

func (_c *mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call) Run(run func(ctx context.Context, certExpireDays int, country string, province string, locality string, altDNSNames []string)) *mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(int), args[2].(string), args[3].(string), args[4].(string), args[5].([]string))
	})
	return _c
}

func (_c *mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call) Return(_a0 error) *mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call) RunAndReturn(run func(context.Context, int, string, string, string, []string) error) *mockSelfSignedCertificateCreator_CreateAndSafeCertificate_Call {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTnewMockSelfSignedCertificateCreator interface {
	mock.TestingT
	Cleanup(func())
}

// newMockSelfSignedCertificateCreator creates a new instance of mockSelfSignedCertificateCreator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func newMockSelfSignedCertificateCreator(t mockConstructorTestingTnewMockSelfSignedCertificateCreator) *mockSelfSignedCertificateCreator {
	mock := &mockSelfSignedCertificateCreator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
