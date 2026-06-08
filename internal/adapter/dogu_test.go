package adapter

import (
	"context"
	"errors"
	"testing"

	doguv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	doguTestNamespace = "ns"
	doguTestName      = "ldap"
)

func doguScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, doguv2.AddToScheme(s))
	return s
}

func newDoguFakeClient(t *testing.T, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(doguScheme(t)).WithObjects(objs...).Build()
}

func newDoguFakeClientWithInterceptor(t *testing.T, ic interceptor.Funcs) client.Client {
	return fake.NewClientBuilder().WithScheme(doguScheme(t)).WithInterceptorFuncs(ic).Build()
}

func doguCR(stopped bool, healthy *metav1.ConditionStatus) *doguv2.Dogu {
	d := &doguv2.Dogu{
		ObjectMeta: metav1.ObjectMeta{Name: doguTestName, Namespace: doguTestNamespace},
		Status:     doguv2.DoguStatus{Stopped: stopped},
	}
	if healthy != nil {
		d.Status.Conditions = []metav1.Condition{{Type: doguv2.ConditionHealthy, Status: *healthy}}
	}
	return d
}

func TestDogu_GetStatus(t *testing.T) {
	conditionTrue := metav1.ConditionTrue
	conditionFalse := metav1.ConditionFalse
	conditionUnknown := metav1.ConditionUnknown

	tests := []struct {
		name     string
		clientFn func(t *testing.T) client.Client
		want     types.ApplicationState
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name: "Stopped + healthy True yields ApplicationStopped (Stopped takes precedence)",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClient(t, doguCR(true, &conditionTrue))
			},
			want:    types.ApplicationStopped,
			wantErr: assert.NoError,
		},
		{
			name: "Stopped without healthy condition yields ApplicationStopped",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClient(t, doguCR(true, nil))
			},
			want:    types.ApplicationStopped,
			wantErr: assert.NoError,
		},
		{
			name: "running + healthy True yields ApplicationRunning",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClient(t, doguCR(false, &conditionTrue))
			},
			want:    types.ApplicationRunning,
			wantErr: assert.NoError,
		},
		{
			name: "running + healthy False yields ApplicationIsStarting",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClient(t, doguCR(false, &conditionFalse))
			},
			want:    types.ApplicationIsStarting,
			wantErr: assert.NoError,
		},
		{
			name: "running + healthy Unknown yields ApplicationIsStarting",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClient(t, doguCR(false, &conditionUnknown))
			},
			want:    types.ApplicationIsStarting,
			wantErr: assert.NoError,
		},
		{
			name: "running + no healthy condition yields ApplicationIsStarting",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClient(t, doguCR(false, nil))
			},
			want:    types.ApplicationIsStarting,
			wantErr: assert.NoError,
		},
		{
			name: "Get returns NotFound is wrapped",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClient(t) // no objects pre-loaded
			},
			want: 0,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorContains(t, err, "failed to get dogu", i...) &&
					assert.True(t, apierrors.IsNotFound(errors.Unwrap(err)), "expected NotFound underneath, got %v", err)
			},
		},
		{
			name: "Get returns generic error is wrapped",
			clientFn: func(t *testing.T) client.Client {
				return newDoguFakeClientWithInterceptor(t, interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return assert.AnError
					},
				})
			},
			want: 0,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to get dogu", i...)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Dogu{client: tt.clientFn(t)}
			got, err := d.GetStatus(t.Context(), doguTestNamespace, doguTestName)
			if !tt.wantErr(t, err) {
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
