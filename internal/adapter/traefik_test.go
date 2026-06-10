package adapter

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const testIngressClass = "traefik"

func newTraefikScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, networkingv1.AddToScheme(s))
	require.NoError(t, traefikapi.AddToScheme(s))

	return s
}

func newTraefikFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().WithScheme(newTraefikScheme(t)).WithObjects(objs...).Build()
}

func newTraefikFakeClientWithInterceptor(t *testing.T, ic interceptor.Funcs, objs ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().WithScheme(newTraefikScheme(t)).WithObjects(objs...).WithInterceptorFuncs(ic).Build()
}

func fixedTraefikExposition(routes []types.HttpRoute) types.Exposition {
	return types.Exposition{
		Name:       "ldap",
		Namespace:  testNamespace,
		HttpRoutes: routes,
		SetOwner:   okOwner,
	}
}

func fixedTraefikController(c client.Client) *TraefikIngressController {
	return &TraefikIngressController{IngressClass: testIngressClass, Client: c}
}

func TestTraefikIngressController_GetOwnableTypes(t *testing.T) {
	ctrl := &TraefikIngressController{}

	got := ctrl.GetOwnableTypes()

	require.Len(t, got, 2)
	assert.IsType(t, &networkingv1.Ingress{}, got[0])
	assert.IsType(t, &traefikapi.Middleware{}, got[1])
}

func TestTraefikIngressController_ProcessExposition(t *testing.T) {
	baseRoute := types.HttpRoute{Name: "ldap-ui", Service: "ldap-ui", Port: 8080, Path: "/ldap"}
	rewrittenRoute := baseRoute
	rewrittenRoute.Rewrite = &types.HttpRewrite{StripPrefix: new("/ldap")}
	regexRoute := types.HttpRoute{
		Name:    "ldap-api",
		Service: "ldap-api",
		Port:    9090,
		Path:    "/api",
		Rewrite: &types.HttpRewrite{Regex: &types.RegexReplacement{Pattern: "^/api/(.*)", Replacement: "/$1"}},
	}

	tests := []struct {
		name       string
		clientFn   func(t *testing.T) client.Client
		appState   types.ApplicationState
		exposition types.Exposition
		wantErr    assert.ErrorAssertionFunc
		postCheck  func(t *testing.T, c client.Client)
	}{
		{
			name:       "running route without rewrite creates ingress without middleware",
			clientFn:   func(t *testing.T) client.Client { return newTraefikFakeClient(t) },
			appState:   types.ApplicationRunning,
			exposition: fixedTraefikExposition([]types.HttpRoute{baseRoute}),
			wantErr:    assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				ingress := getIngress(t, c, "ldap-ui")
				assert.Equal(t, testNamespace, ingress.Namespace)
				assert.Equal(t, testIngressClass, *ingress.Spec.IngressClassName)
				assert.Empty(t, ingress.Annotations)
				assertExpectedTraefikLabels(t, ingress.Labels)
				assertIngressPath(t, ingress, "/ldap", "ldap-ui", 8080)
				assertNoMiddleware(t, c, "ldap-ui-rewrite")
			},
		},
		{
			name:       "running route with StripPrefix rewrite creates referenced middleware",
			clientFn:   func(t *testing.T) client.Client { return newTraefikFakeClient(t) },
			appState:   types.ApplicationRunning,
			exposition: fixedTraefikExposition([]types.HttpRoute{rewrittenRoute}),
			wantErr:    assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				ingress := getIngress(t, c, "ldap-ui")
				assert.Equal(t, map[string]string{traefikMiddlewareAnnotationKey: "ns-ldap-ui-rewrite@kubernetescrd"}, ingress.Annotations)

				middleware := getMiddleware(t, c, "ldap-ui-rewrite")
				require.NotNil(t, middleware.Spec.StripPrefix)
				assert.Equal(t, []string{"/ldap"}, middleware.Spec.StripPrefix.Prefixes)
				assert.Nil(t, middleware.Spec.ReplacePathRegex)
				assertExpectedTraefikLabels(t, middleware.Labels)
			},
		},
		{
			name:       "running route with regex rewrite creates ReplacePathRegex middleware",
			clientFn:   func(t *testing.T) client.Client { return newTraefikFakeClient(t) },
			appState:   types.ApplicationRunning,
			exposition: fixedTraefikExposition([]types.HttpRoute{regexRoute}),
			wantErr:    assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				middleware := getMiddleware(t, c, "ldap-api-rewrite")
				require.NotNil(t, middleware.Spec.ReplacePathRegex)
				assert.Equal(t, "^/api/(.*)", middleware.Spec.ReplacePathRegex.Regex)
				assert.Equal(t, "/$1", middleware.Spec.ReplacePathRegex.Replacement)
				assert.Nil(t, middleware.Spec.StripPrefix)
			},
		},
		{
			name: "stopped application deletes stale owned ingress and middleware",
			clientFn: func(t *testing.T) client.Client {
				return newTraefikFakeClient(t,
					staleIngress("ldap-ui"),
					staleMiddleware("ldap-ui-rewrite"),
				)
			},
			appState:   types.ApplicationStopped,
			exposition: fixedTraefikExposition([]types.HttpRoute{rewrittenRoute}),
			wantErr:    assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				assertNoIngress(t, c, "ldap-ui")
				assertNoMiddleware(t, c, "ldap-ui-rewrite")
			},
		},
		{
			name: "starting application uses static middleware and removes stale rewrite middleware",
			clientFn: func(t *testing.T) client.Client {
				return newTraefikFakeClient(t, staleMiddleware("ldap-ui-rewrite"))
			},
			appState:   types.ApplicationIsStarting,
			exposition: fixedTraefikExposition([]types.HttpRoute{rewrittenRoute}),
			wantErr:    assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				ingress := getIngress(t, c, "ldap-ui")
				assert.Equal(t, map[string]string{traefikMiddlewareAnnotationKey: staticContentDoguIsStartingRewrite}, ingress.Annotations)
				assertNoMiddleware(t, c, "ldap-ui-rewrite")
			},
		},
		{
			name:       "maintenance application uses static maintenance middleware",
			clientFn:   func(t *testing.T) client.Client { return newTraefikFakeClient(t) },
			appState:   types.ApplicationMaintenance,
			exposition: fixedTraefikExposition([]types.HttpRoute{baseRoute}),
			wantErr:    assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				ingress := getIngress(t, c, "ldap-ui")
				assert.Equal(t, map[string]string{traefikMiddlewareAnnotationKey: staticContentMaintenanceRewrite}, ingress.Annotations)
			},
		},
		{
			name: "multiple desired routes update matching objects, delete stale owned objects, keep unrelated objects",
			clientFn: func(t *testing.T) client.Client {
				return newTraefikFakeClient(t,
					staleIngress("ldap-ui"),
					staleIngress("ldap-old"),
					staleMiddleware("ldap-old-rewrite"),
					foreignIngress("foreign-ui"),
					foreignMiddleware("foreign-ui-rewrite"),
				)
			},
			appState:   types.ApplicationRunning,
			exposition: fixedTraefikExposition([]types.HttpRoute{baseRoute, regexRoute}),
			wantErr:    assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				assertIngressPath(t, getIngress(t, c, "ldap-ui"), "/ldap", "ldap-ui", 8080)
				assertIngressPath(t, getIngress(t, c, "ldap-api"), "/api", "ldap-api", 9090)
				assertNoIngress(t, c, "ldap-old")
				assertNoMiddleware(t, c, "ldap-old-rewrite")
				require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "foreign-ui"}, &networkingv1.Ingress{}))
				require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "foreign-ui-rewrite"}, &traefikapi.Middleware{}))
			},
		},
		{
			name:     "SetOwner failure is wrapped and no resources are upserted",
			clientFn: func(t *testing.T) client.Client { return newTraefikFakeClient(t) },
			appState: types.ApplicationRunning,
			exposition: types.Exposition{
				Name:       "ldap",
				Namespace:  testNamespace,
				HttpRoutes: []types.HttpRoute{baseRoute},
				SetOwner:   errOwner,
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to generate ingresses or middlewares", i...)
			},
			postCheck: func(t *testing.T, c client.Client) {
				assertNoIngress(t, c, "ldap-ui")
			},
		},
		{
			name: "ingress client errors are wrapped",
			clientFn: func(t *testing.T) client.Client {
				return newTraefikFakeClientWithInterceptor(t, interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
						if _, ok := list.(*networkingv1.IngressList); ok {
							return assert.AnError
						}
						return nil
					},
				})
			},
			appState:   types.ApplicationRunning,
			exposition: fixedTraefikExposition([]types.HttpRoute{baseRoute}),
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, `failed to upsert ingresses or middlewares for "ldap"`, i...) &&
					assert.ErrorContains(t, err, "failed to list existing ingresses", i...)
			},
		},
		{
			name: "middleware client errors are wrapped",
			clientFn: func(t *testing.T) client.Client {
				return newTraefikFakeClientWithInterceptor(t, interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
						if _, ok := list.(*traefikapi.MiddlewareList); ok {
							return assert.AnError
						}
						return nil
					},
				})
			},
			appState:   types.ApplicationRunning,
			exposition: fixedTraefikExposition([]types.HttpRoute{baseRoute}),
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, `failed to upsert ingresses or middlewares for "ldap"`, i...) &&
					assert.ErrorContains(t, err, "failed to list existing middlewares", i...)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.clientFn(t)
			ctrl := fixedTraefikController(c)

			err := ctrl.ProcessExposition(t.Context(), tt.appState, tt.exposition)

			if !tt.wantErr(t, err) {
				return
			}
			if tt.postCheck != nil {
				tt.postCheck(t, c)
			}
		})
	}
}

func assertExpectedTraefikLabels(t *testing.T, got map[string]string) {
	t.Helper()

	want := map[string]string{ownedByLabelKey: "ldap"}
	for k, v := range util.K8sCesServiceDiscoveryLabels {
		want[k] = v
	}
	assert.Equal(t, want, got)
}

func assertIngressPath(t *testing.T, ingress *networkingv1.Ingress, path, service string, port int32) {
	t.Helper()

	require.Len(t, ingress.Spec.Rules, 1)
	require.NotNil(t, ingress.Spec.Rules[0].HTTP)
	require.Len(t, ingress.Spec.Rules[0].HTTP.Paths, 1)

	gotPath := ingress.Spec.Rules[0].HTTP.Paths[0]
	assert.Equal(t, path, gotPath.Path)
	require.NotNil(t, gotPath.PathType)
	assert.Equal(t, networkingv1.PathTypePrefix, *gotPath.PathType)
	require.NotNil(t, gotPath.Backend.Service)
	assert.Equal(t, service, gotPath.Backend.Service.Name)
	assert.Equal(t, port, gotPath.Backend.Service.Port.Number)
}

func getIngress(t *testing.T, c client.Client, name string) *networkingv1.Ingress {
	t.Helper()

	ingress := &networkingv1.Ingress{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, ingress))
	return ingress
}

func getMiddleware(t *testing.T, c client.Client, name string) *traefikapi.Middleware {
	t.Helper()

	middleware := &traefikapi.Middleware{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, middleware))
	return middleware
}

func assertNoIngress(t *testing.T, c client.Client, name string) {
	t.Helper()

	err := c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, &networkingv1.Ingress{})
	assert.True(t, apierrors.IsNotFound(err), "expected ingress %q to be absent, got %v", name, err)
}

func assertNoMiddleware(t *testing.T, c client.Client, name string) {
	t.Helper()

	err := c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, &traefikapi.Middleware{})
	assert.True(t, apierrors.IsNotFound(err), "expected middleware %q to be absent, got %v", name, err)
}

func staleIngress(name string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{ownedByLabelKey: "ldap"},
		},
	}
}

func foreignIngress(name string) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{ownedByLabelKey: "foreign"},
		},
	}
}

func staleMiddleware(name string) *traefikapi.Middleware {
	return &traefikapi.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{ownedByLabelKey: "ldap"},
		},
	}
}

func foreignMiddleware(name string) *traefikapi.Middleware {
	return &traefikapi.Middleware{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNamespace,
			Labels:    map[string]string{ownedByLabelKey: "foreign"},
		},
	}
}

func TestTraefikIngressController_ProcessExposition_joinsIngressAndMiddlewareErrors(t *testing.T) {
	c := newTraefikFakeClientWithInterceptor(t, interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
			switch list.(type) {
			case *networkingv1.IngressList, *traefikapi.MiddlewareList:
				return assert.AnError
			default:
				return nil
			}
		},
	})
	ctrl := fixedTraefikController(c)
	exposition := fixedTraefikExposition([]types.HttpRoute{{Name: "ui", Service: "ldap-ui", Port: 8080, Path: "/ldap"}})

	err := ctrl.ProcessExposition(t.Context(), types.ApplicationRunning, exposition)

	require.Error(t, err)
	assert.True(t, errors.Is(err, assert.AnError))
	assert.ErrorContains(t, err, "failed to list existing ingresses")
	assert.ErrorContains(t, err, "failed to list existing middlewares")
}
