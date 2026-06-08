package adapter

import (
	"testing"

	"github.com/cloudogu/k8s-registry-lib/repository"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	originalRewrite = &types.HttpRewrite{StripPrefix: stringPtr("/ldap")}
	originalRoutes  = []types.HttpRoute{
		{Name: "ui", Service: "ldap-ui", Port: 8080, Path: "/ldap", Rewrite: originalRewrite},
	}
	redirectedRoutes = Ingress{}.redirectHttpRoutesToStaticBackend(originalRoutes)
)

func stringPtr(s string) *string { return &s }

func fixedExposition(routes []types.HttpRoute) types.Exposition {
	return types.Exposition{Name: "ldap", Namespace: "ns", HttpRoutes: routes}
}

func TestIngress_GetOwnableTypes(t *testing.T) {
	want := []client.Object{&networkingv1.Ingress{}, &corev1.Service{}}
	ctrl := newMockIngressController(t)
	ctrl.EXPECT().GetOwnableTypes().Return(want)

	i := Ingress{Controller: ctrl}
	assert.Equal(t, want, i.GetOwnableTypes())
}

func TestIngress_ProcessExposition(t *testing.T) {
	type fields struct {
		doguFn        func(t *testing.T) doguAdapter
		maintenanceFn func(t *testing.T) maintenanceAdapter
		controllerFn  func(t *testing.T) ingressController
	}
	exposition := fixedExposition(originalRoutes)

	okMaintenance := func(active bool) func(t *testing.T) maintenanceAdapter {
		return func(t *testing.T) maintenanceAdapter {
			m := newMockMaintenanceAdapter(t)
			m.EXPECT().GetStatus(t.Context()).Return(repository.MaintenanceModeDescription{}, active, nil)
			return m
		}
	}
	okDogu := func(state types.ApplicationState) func(t *testing.T) doguAdapter {
		return func(t *testing.T) doguAdapter {
			m := newMockDoguAdapter(t)
			m.EXPECT().GetStatus(t.Context(), "ns", "ldap").Return(state, nil)
			return m
		}
	}
	expectController := func(wantState types.ApplicationState, wantRoutes []types.HttpRoute, ret error) func(t *testing.T) ingressController {
		return func(t *testing.T) ingressController {
			m := newMockIngressController(t)
			m.EXPECT().ProcessExposition(t.Context(), wantState, fixedExposition(wantRoutes)).Return(ret)
			return m
		}
	}
	noController := func(t *testing.T) ingressController {
		return newMockIngressController(t)
	}
	noMaintenance := func(t *testing.T) maintenanceAdapter {
		return newMockMaintenanceAdapter(t)
	}

	tests := []struct {
		name    string
		fields  fields
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "dogu Stopped, no maintenance — routes untouched",
			fields: fields{
				doguFn:        okDogu(types.ApplicationStopped),
				maintenanceFn: okMaintenance(false),
				controllerFn:  expectController(types.ApplicationStopped, originalRoutes, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "dogu Running, no maintenance — routes untouched",
			fields: fields{
				doguFn:        okDogu(types.ApplicationRunning),
				maintenanceFn: okMaintenance(false),
				controllerFn:  expectController(types.ApplicationRunning, originalRoutes, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "dogu IsStarting — routes redirected to static backend",
			fields: fields{
				doguFn:        okDogu(types.ApplicationIsStarting),
				maintenanceFn: okMaintenance(false),
				controllerFn:  expectController(types.ApplicationIsStarting, redirectedRoutes, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "dogu Maintenance — routes redirected to static backend",
			fields: fields{
				doguFn:        okDogu(types.ApplicationMaintenance),
				maintenanceFn: okMaintenance(false),
				controllerFn:  expectController(types.ApplicationMaintenance, redirectedRoutes, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "global maintenance promotes Running to Maintenance and redirects",
			fields: fields{
				doguFn:        okDogu(types.ApplicationRunning),
				maintenanceFn: okMaintenance(true),
				controllerFn:  expectController(types.ApplicationMaintenance, redirectedRoutes, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "global maintenance promotes Stopped to Maintenance and redirects",
			fields: fields{
				doguFn:        okDogu(types.ApplicationStopped),
				maintenanceFn: okMaintenance(true),
				controllerFn:  expectController(types.ApplicationMaintenance, redirectedRoutes, nil),
			},
			wantErr: assert.NoError,
		},
		{
			name: "dogu.GetStatus errors are wrapped",
			fields: fields{
				doguFn: func(t *testing.T) doguAdapter {
					m := newMockDoguAdapter(t)
					m.EXPECT().GetStatus(t.Context(), "ns", "ldap").Return(types.ApplicationStopped, assert.AnError)
					return m
				},
				maintenanceFn: noMaintenance,
				controllerFn:  noController,
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get status of dogu", i...)
			},
		},
		{
			name: "maintenance.GetStatus errors are wrapped",
			fields: fields{
				doguFn: okDogu(types.ApplicationRunning),
				maintenanceFn: func(t *testing.T) maintenanceAdapter {
					m := newMockMaintenanceAdapter(t)
					m.EXPECT().GetStatus(t.Context()).Return(repository.MaintenanceModeDescription{}, false, assert.AnError)
					return m
				},
				controllerFn: noController,
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get status of global maintenance mode", i...)
			},
		},
		{
			name: "controller.ProcessExposition errors are wrapped with state info",
			fields: fields{
				doguFn:        okDogu(types.ApplicationRunning),
				maintenanceFn: okMaintenance(false),
				controllerFn:  expectController(types.ApplicationRunning, originalRoutes, assert.AnError),
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to process exposition from", i...) &&
					assert.ErrorContains(t, err, "while dogu is in state Running", i...)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Ingress{
				Dogu:        tt.fields.doguFn(t),
				Maintenance: tt.fields.maintenanceFn(t),
				Controller:  tt.fields.controllerFn(t),
			}
			err := i.ProcessExposition(t.Context(), exposition)
			tt.wantErr(t, err)
		})
	}
}

func Test_Ingress_redirectHttpRoutesToStaticBackend(t *testing.T) {
	tests := []struct {
		name string
		in   []types.HttpRoute
		want []types.HttpRoute
	}{
		{
			name: "nil input yields empty slice",
			in:   nil,
			want: []types.HttpRoute{},
		},
		{
			name: "single route is redirected, Path preserved, Rewrite cleared",
			in: []types.HttpRoute{
				{Name: "ui", Service: "ldap-ui", Port: 8080, Path: "/ldap", Rewrite: &types.HttpRewrite{StripPrefix: stringPtr("/ldap")}},
			},
			want: []types.HttpRoute{
				{Name: "ui", Service: staticContentBackendName, Port: staticContentBackendPort, Path: "/ldap", Rewrite: nil},
			},
		},
		{
			name: "two routes preserve order",
			in: []types.HttpRoute{
				{Name: "a", Service: "svc-a", Port: 80, Path: "/a"},
				{Name: "b", Service: "svc-b", Port: 81, Path: "/b"},
			},
			want: []types.HttpRoute{
				{Name: "a", Service: staticContentBackendName, Port: staticContentBackendPort, Path: "/a", Rewrite: nil},
				{Name: "b", Service: staticContentBackendName, Port: staticContentBackendPort, Path: "/b", Rewrite: nil},
			},
		},
		{
			name: "empty Name and Path still pass through",
			in: []types.HttpRoute{
				{},
			},
			want: []types.HttpRoute{
				{Service: staticContentBackendName, Port: staticContentBackendPort},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Ingress{}
			got := i.redirectHttpRoutesToStaticBackend(tt.in)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got)
		})
	}
}
