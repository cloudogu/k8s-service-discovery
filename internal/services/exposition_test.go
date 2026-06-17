package services

import (
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestNewExpositionService(t *testing.T) {
	type want struct {
		len int
	}
	tests := []struct {
		name         string
		processorsFn func(t *testing.T) []Processor
		want         want
	}{
		{
			name:         "no processors",
			processorsFn: func(t *testing.T) []Processor { return nil },
			want:         want{len: 0},
		},
		{
			name: "one processor",
			processorsFn: func(t *testing.T) []Processor {
				return []Processor{NewMockProcessor(t)}
			},
			want: want{len: 1},
		},
		{
			name: "three processors preserve order",
			processorsFn: func(t *testing.T) []Processor {
				return []Processor{NewMockProcessor(t), NewMockProcessor(t), NewMockProcessor(t)}
			},
			want: want{len: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			procs := tt.processorsFn(t)
			s := NewExpositionService(procs...)
			require.NotNil(t, s)
			assert.Len(t, s.processors, tt.want.len)
			for i, p := range procs {
				assert.Same(t, p, s.processors[i], "processor at index %d", i)
			}
		})
	}
}

func TestExpositionService_GetOwnableTypes(t *testing.T) {
	cm := &corev1.ConfigMap{}
	svc := &corev1.Service{}

	tests := []struct {
		name         string
		processorsFn func(t *testing.T) []Processor
		want         []client.Object
	}{
		{
			name:         "no processors yields nil",
			processorsFn: func(t *testing.T) []Processor { return nil },
			want:         nil,
		},
		{
			name: "one processor with two types",
			processorsFn: func(t *testing.T) []Processor {
				m := NewMockProcessor(t)
				m.EXPECT().GetOwnableTypes().Return([]client.Object{cm, svc})
				return []Processor{m}
			},
			want: []client.Object{cm, svc},
		},
		{
			name: "two processors aggregate in order",
			processorsFn: func(t *testing.T) []Processor {
				first := NewMockProcessor(t)
				first.EXPECT().GetOwnableTypes().Return([]client.Object{cm})
				second := NewMockProcessor(t)
				second.EXPECT().GetOwnableTypes().Return([]client.Object{svc})
				return []Processor{first, second}
			},
			want: []client.Object{cm, svc},
		},
		{
			name: "processor returning nil contributes nothing",
			processorsFn: func(t *testing.T) []Processor {
				first := NewMockProcessor(t)
				first.EXPECT().GetOwnableTypes().Return(nil)
				second := NewMockProcessor(t)
				second.EXPECT().GetOwnableTypes().Return([]client.Object{cm})
				return []Processor{first, second}
			},
			want: []client.Object{cm},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewExpositionService(tt.processorsFn(t)...)
			assert.Equal(t, tt.want, s.GetOwnableTypes())
		})
	}
}

func TestExpositionService_ProcessExposition(t *testing.T) {
	exposition := types.Exposition{Name: "ldap", Namespace: "ns"}

	expectProc := func(t *testing.T, expectCall bool, ret error) *MockProcessor {
		m := NewMockProcessor(t)
		if expectCall {
			m.EXPECT().ProcessExposition(mock.Anything, exposition).Return(ret)
		}
		return m
	}

	tests := []struct {
		name         string
		processorsFn func(t *testing.T) []Processor
		wantErr      assert.ErrorAssertionFunc
	}{
		{
			name:         "no processors returns nil",
			processorsFn: func(t *testing.T) []Processor { return nil },
			wantErr:      assert.NoError,
		},
		{
			name: "single processor success",
			processorsFn: func(t *testing.T) []Processor {
				return []Processor{expectProc(t, true, nil)}
			},
			wantErr: assert.NoError,
		},
		{
			name: "single processor error",
			processorsFn: func(t *testing.T) []Processor {
				return []Processor{expectProc(t, true, assert.AnError)}
			},
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorContains(t, err, "failed to process exposition with *services.MockProcessor", i...)
			},
		},
		{
			name: "three processors all succeed; all called",
			processorsFn: func(t *testing.T) []Processor {
				return []Processor{
					expectProc(t, true, nil),
					expectProc(t, true, nil),
					expectProc(t, true, nil),
				}
			},
			wantErr: assert.NoError,
		},
		{
			name: "chain stops at first error; later processor not called",
			processorsFn: func(t *testing.T) []Processor {
				return []Processor{
					expectProc(t, true, nil),
					expectProc(t, true, assert.AnError),
					expectProc(t, false, nil), // must not be invoked
				}
			},
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorContains(t, err, "failed to process exposition with", i...)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewExpositionService(tt.processorsFn(t)...)
			err := s.ProcessExposition(t.Context(), exposition)
			tt.wantErr(t, err)
		})
	}
}
