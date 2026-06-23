package expose

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestMiddlewareManager_CreateOrUpdateAlternativeFQDNRedirectMiddleware(t *testing.T) {
	const expectedMiddlewareName = "alternative-fqdn"

	t.Run("should create middleware when it does not exist", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}

		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, expectedMiddlewareName))
		clientMock.EXPECT().Create(t.Context(), mock.AnythingOfType("*v1alpha1.Middleware"), v1.CreateOptions{}).Return(&traefikapi.Middleware{}, nil)

		// when
		result, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt1.example.com", "alt2.example.com"}, "primary.example.com", nil)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedMiddlewareName, result)
	})

	t.Run("should update middleware when it already exists", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}

		existingMiddleware := &traefikapi.Middleware{
			ObjectMeta: v1.ObjectMeta{Name: expectedMiddlewareName, ResourceVersion: "7"},
		}
		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(existingMiddleware, nil)
		clientMock.EXPECT().Update(t.Context(), mock.AnythingOfType("*v1alpha1.Middleware"), v1.UpdateOptions{}).Return(&traefikapi.Middleware{}, nil)

		// when
		result, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt1.example.com"}, "primary.example.com", nil)

		// then
		require.NoError(t, err)
		assert.Equal(t, expectedMiddlewareName, result)
	})

	t.Run("should set resource version from existing middleware on update", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}
		expectedResourceVersion := "55"

		existingMiddleware := &traefikapi.Middleware{
			ObjectMeta: v1.ObjectMeta{Name: expectedMiddlewareName, ResourceVersion: expectedResourceVersion},
		}
		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(existingMiddleware, nil)
		clientMock.EXPECT().Update(t.Context(), mock.MatchedBy(func(mw *traefikapi.Middleware) bool {
			// since the middleware ist never returned, it is validated here
			return mw.ResourceVersion == expectedResourceVersion
		}), v1.UpdateOptions{}).Return(&traefikapi.Middleware{}, nil)

		// when
		_, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt.example.com"}, "primary.example.com", nil)

		// then
		require.NoError(t, err)
	})

	t.Run("should build correct regex pattern with escaped dots and correct replacement", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}

		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, expectedMiddlewareName))
		clientMock.EXPECT().Create(t.Context(), mock.MatchedBy(func(mw *traefikapi.Middleware) bool {
			// since the middleware ist never returned, it is validated here
			spec := mw.Spec.RedirectRegex
			return spec != nil &&
				spec.Regex == `^https?://(alt1\.example\.com|alt2\.example\.com)(.*)` &&
				spec.Replacement == "https://primary.example.com${2}"
		}), v1.CreateOptions{}).Return(&traefikapi.Middleware{}, nil)

		// when
		_, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt1.example.com", "alt2.example.com"}, "primary.example.com", nil)

		// then
		require.NoError(t, err)
	})

	t.Run("should set Namespace on created middleware", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "my-Namespace"}

		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, expectedMiddlewareName))
		clientMock.EXPECT().Create(t.Context(), mock.MatchedBy(func(mw *traefikapi.Middleware) bool {
			// since the middleware ist never returned, it is validated here
			return mw.Namespace == "my-Namespace"
		}), v1.CreateOptions{}).Return(&traefikapi.Middleware{}, nil)

		// when
		_, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt.example.com"}, "primary.example.com", nil)

		// then
		require.NoError(t, err)
	})

	t.Run("should set owner references on created middleware", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}
		ownerRefs := []v1.OwnerReference{{Name: "my-owner"}}

		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, expectedMiddlewareName))
		clientMock.EXPECT().Create(t.Context(), mock.MatchedBy(func(mw *traefikapi.Middleware) bool {
			// since the middleware ist never returned, it is validated here
			return len(mw.OwnerReferences) == 1 && mw.OwnerReferences[0].Name == "my-owner"
		}), v1.CreateOptions{}).Return(&traefikapi.Middleware{}, nil)

		// when
		_, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt.example.com"}, "primary.example.com", ownerRefs)

		// then
		require.NoError(t, err)
	})

	t.Run("should return error when get fails with non-404 error", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}

		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(nil, fmt.Errorf("server unavailable"))

		// when
		result, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt.example.com"}, "primary.example.com", nil)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get middleware")
		assert.Empty(t, result)
	})

	t.Run("should return error when create fails", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}

		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, expectedMiddlewareName))
		clientMock.EXPECT().Create(t.Context(), mock.AnythingOfType("*v1alpha1.Middleware"), v1.CreateOptions{}).Return(nil, fmt.Errorf("create failed"))

		// when
		result, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt.example.com"}, "primary.example.com", nil)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create alternative FQDN redirect middleware")
		assert.Empty(t, result)
	})

	t.Run("should return error when update fails", func(t *testing.T) {
		// given
		clientMock := newMockMiddlewareInterface(t)
		manager := &MiddlewareManager{client: clientMock, namespace: "test-Namespace"}

		existingMiddleware := &traefikapi.Middleware{
			ObjectMeta: v1.ObjectMeta{Name: expectedMiddlewareName, ResourceVersion: "3"},
		}
		clientMock.EXPECT().Get(t.Context(), expectedMiddlewareName, v1.GetOptions{}).Return(existingMiddleware, nil)
		clientMock.EXPECT().Update(t.Context(), mock.AnythingOfType("*v1alpha1.Middleware"), v1.UpdateOptions{}).Return(nil, fmt.Errorf("update failed"))

		// when
		result, err := manager.CreateOrUpdateAlternativeFQDNRedirectMiddleware(t.Context(), []string{"alt.example.com"}, "primary.example.com", nil)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to update alternative FQDN redirect middleware")
		assert.Empty(t, result)
	})
}
