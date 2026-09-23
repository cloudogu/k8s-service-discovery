package controllers

import (
	"context"
	"testing"

	k8sv2 "github.com/cloudogu/k8s-dogu-lib/v2/api/v2"
	expositionv1 "github.com/cloudogu/k8s-exposition-lib/api/v1"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	testLBNamespace = "default"
)

func Test_loadbalancerConfigPredicate(t *testing.T) {
	loadbalancerCfgPredicate := loadbalancerConfigPredicate()

	t.Run("reconcile loadbalancer-config", func(t *testing.T) {
		assert.True(t, loadbalancerCfgPredicate.CreateFunc(event.CreateEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: "default"}}}))
		assert.True(t, loadbalancerCfgPredicate.UpdateFunc(event.UpdateEvent{
			ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: "default"}},
			ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: "default"}},
		}))
		assert.True(t, loadbalancerCfgPredicate.DeleteFunc(event.DeleteEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: "default"}}}))
		assert.True(t, loadbalancerCfgPredicate.GenericFunc(event.GenericEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: "default"}}}))
	})

	t.Run("ignore any other config map", func(t *testing.T) {
		assert.False(t, loadbalancerCfgPredicate.CreateFunc(event.CreateEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}}}))
		assert.False(t, loadbalancerCfgPredicate.UpdateFunc(event.UpdateEvent{
			ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}},
			ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}},
		}))
		assert.False(t, loadbalancerCfgPredicate.DeleteFunc(event.DeleteEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}}}))
		assert.False(t, loadbalancerCfgPredicate.GenericFunc(event.GenericEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}}}))
	})
}

func Test_exposedPortServicePredicate(t *testing.T) {
	const exposedPortServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"

	r := &LoadBalancerReconciler{Client: testclient.NewClientBuilder().Build()}
	expPortServicePredicate := r.exposedPortServicePredicate()

	t.Run("reconcile exposed port service on create", func(t *testing.T) {
		assert.True(t, expPortServicePredicate.CreateFunc(event.CreateEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					PortExposerLabel: "testDogu",
				},
				Annotations: map[string]string{
					exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 50000, TargetPort: intstr.FromInt32(50000)}},
			}}},
		))
	})

	t.Run("ignore dogu service without exposed ports on create", func(t *testing.T) {
		assert.False(t, expPortServicePredicate.CreateFunc(event.CreateEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}}},
		))
	})

	t.Run("ignore any other service on create", func(t *testing.T) {
		assert.False(t, expPortServicePredicate.CreateFunc(event.CreateEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}}},
		))
	})

	t.Run("reconcile exposed port service on delete", func(t *testing.T) {
		assert.True(t, expPortServicePredicate.DeleteFunc(event.DeleteEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
				Annotations: map[string]string{
					exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 50000, TargetPort: intstr.FromInt32(50000)}},
			}}},
		))
	})

	t.Run("ignore dogu service without exposed ports on delete", func(t *testing.T) {
		assert.False(t, expPortServicePredicate.DeleteFunc(event.DeleteEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}}},
		))
	})

	t.Run("ignore any other service on delete", func(t *testing.T) {
		assert.False(t, expPortServicePredicate.DeleteFunc(event.DeleteEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}}},
		))
	})

	t.Run("reconcile exposed port service on generic", func(t *testing.T) {
		assert.True(t, expPortServicePredicate.GenericFunc(event.GenericEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
				Annotations: map[string]string{
					exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type:  corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 50000, TargetPort: intstr.FromInt32(50000)}},
			}}},
		))
	})

	t.Run("ignore dogu service without exposed ports on generic", func(t *testing.T) {
		assert.False(t, expPortServicePredicate.GenericFunc(event.GenericEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}}},
		))
	})

	t.Run("ignore any other service on generic", func(t *testing.T) {
		assert.False(t, expPortServicePredicate.GenericFunc(event.GenericEvent{Object: &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			}}},
		))
	})

	t.Run("reconcile exposed port service on update", func(t *testing.T) {
		exposedDoguService := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
				Annotations: map[string]string{
					exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Protocol:   corev1.ProtocolTCP,
						Port:       50000,
						TargetPort: intstr.FromInt32(50000),
					},
				},
			},
		}

		noExposedPorts := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
			},
		}

		otherExposedPorts := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
				Annotations: map[string]string{
					exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":60000}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Protocol:   corev1.ProtocolTCP,
						Port:       50000,
						TargetPort: intstr.FromInt32(60000),
					},
				},
			},
		}

		invalidExposedPorts := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					k8sv2.DoguLabelName: "testDogu",
				},
				Annotations: map[string]string{
					exposedPortServiceAnnotation: `[{"protocol":"INVALID","port":50000,"targetPort":60000}]`,
				},
			},
			Spec: corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{
					{
						Protocol:   corev1.ProtocolTCP,
						Port:       50000,
						TargetPort: intstr.FromInt32(60000),
					},
				},
			},
		}

		t.Run("reconcile when at least one service is exposed dogu service", func(t *testing.T) {
			assert.True(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: exposedDoguService,
				ObjectNew: &corev1.Service{},
			}))
		})

		t.Run("reconcile when dogu service has no exposed ports anymore", func(t *testing.T) {
			assert.True(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: exposedDoguService,
				ObjectNew: noExposedPorts,
			}))
		})

		t.Run("reconcile when ports are getting updated", func(t *testing.T) {
			assert.True(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: exposedDoguService,
				ObjectNew: otherExposedPorts,
			}))
		})

		t.Run("ignore when both service are no dogu services", func(t *testing.T) {
			assert.False(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: &corev1.Service{},
				ObjectNew: &corev1.Service{},
			}))
		})

		t.Run("ignore when exposed services are equal", func(t *testing.T) {
			assert.False(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: exposedDoguService,
				ObjectNew: exposedDoguService,
			}))
		})

		t.Run("reconcile when old service has unparseable annotation", func(t *testing.T) {
			// old fails to map (invalid protocol) → treated as non-dogu-service transition → reconcile
			assert.True(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: invalidExposedPorts,
				ObjectNew: exposedDoguService,
			}))
		})

		t.Run("reconcile when new service has unparseable annotation", func(t *testing.T) {
			// new fails to map (invalid protocol) → treated as non-dogu-service transition → reconcile
			assert.True(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: exposedDoguService,
				ObjectNew: invalidExposedPorts,
			}))
		})
	})
}

func Test_loadbalancerServicePredicate(t *testing.T) {
	validLB := &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.LoadbalancerName,
			Namespace: "testNamespace",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{},
	}

	updatedSpec := &corev1.Service{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.LoadbalancerName,
			Namespace: "testNamespace",
		},
		Spec: corev1.ServiceSpec{
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
			Type:                  corev1.ServiceTypeLoadBalancer,
		},
		Status: corev1.ServiceStatus{},
	}

	lbServicePredicate := loadbalancerServicePredicate()

	t.Run("ignore loadbalancer on create", func(t *testing.T) {
		assert.False(t, lbServicePredicate.CreateFunc(event.CreateEvent{Object: validLB}))
	})

	t.Run("reconcile loadbalancer on delete", func(t *testing.T) {
		assert.True(t, lbServicePredicate.DeleteFunc(event.DeleteEvent{Object: validLB}))
	})

	t.Run("ignore any other service on delete", func(t *testing.T) {
		assert.False(t, lbServicePredicate.DeleteFunc(event.DeleteEvent{Object: &corev1.Service{}}))
	})

	t.Run("reconcile loadbalancer on generic", func(t *testing.T) {
		assert.True(t, lbServicePredicate.GenericFunc(event.GenericEvent{Object: validLB}))
	})

	t.Run("ignore any other service on generic", func(t *testing.T) {
		assert.False(t, lbServicePredicate.GenericFunc(event.GenericEvent{Object: &corev1.Service{}}))
	})

	t.Run("reconcile loadbalancer on update when specs have changed", func(t *testing.T) {
		assert.True(t, lbServicePredicate.UpdateFunc(event.UpdateEvent{
			ObjectOld: validLB,
			ObjectNew: updatedSpec,
		}))
	})

	t.Run("ignore when oldObject is invalid", func(t *testing.T) {
		assert.False(t, lbServicePredicate.UpdateFunc(event.UpdateEvent{
			ObjectOld: &corev1.Service{},
			ObjectNew: updatedSpec,
		}))
	})

	t.Run("ignore when newObject is invalid", func(t *testing.T) {
		assert.False(t, lbServicePredicate.UpdateFunc(event.UpdateEvent{
			ObjectOld: validLB,
			ObjectNew: &corev1.Service{},
		}))
	})
}

func Test_enqueueLoadBalancerConfig(t *testing.T) {
	t.Run("map every object to loadbalancer-config", func(t *testing.T) {
		reconcileObjects := enqueueLoadBalancerConfig(context.TODO(), &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: "test"}})

		assert.Len(t, reconcileObjects, 1)
		assert.Equal(t, types.LoadBalancerConfigName, reconcileObjects[0].Name)
		assert.Equal(t, "test", reconcileObjects[0].Namespace)
	})
}

func TestLoadBalancerReconciler_Reconcile(t *testing.T) {
	const exposedPortServiceAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"

	lbConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: testLBNamespace}}
	exposedService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-dogu",
			Namespace: testLBNamespace,
			Labels: map[string]string{
				k8sv2.DoguLabelName: "testDogu",
			},
			Annotations: map[string]string{
				exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
			},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{Protocol: corev1.ProtocolTCP, Port: 50000, TargetPort: intstr.FromInt32(50000)}},
		}}
	testExposition := &expositionv1.Exposition{
		ObjectMeta: metav1.ObjectMeta{Name: "test-exposition", Namespace: testLBNamespace},
		Spec: expositionv1.ExpositionSpec{
			TCP: []expositionv1.TCPEntry{{Name: "ssh", Service: "test-svc", Port: 22}},
		},
	}

	existingLB := types.CreateLoadBalancer(testLBNamespace, createDefaultLoadbalancerConfig(), []types.Exposition{}, map[string]string{"test": "test"})

	tests := []struct {
		name                 string
		expoConfig           types.ExpositionConfig
		inClientMock         client.Client
		setupLoggerMock      func(m *MockLogSink)
		setupPortExposerMock func(m *MockPortExposer)
		expErr               bool
		errMsg               string
	}{
		{
			name:                 "create new loadbalancer",
			inClientMock:         createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               false,
		},
		{
			name:                 "update loadbalancer",
			inClientMock:         createDefaultLBClientMock(lbConfigMap, exposedService, existingLB.ToK8sService()),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               false,
		},
		{
			name:                 "error client get loadbalancer config map",
			inClientMock:         createDefaultLBClientMock(),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: func(m *MockPortExposer) {},
			expErr:               true,
			errMsg:               "failed to get config map for loadbalancer",
		},
		{
			name: "error parsing loadbalancer config map",
			inClientMock: createDefaultLBClientMock(&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.LoadBalancerConfigName,
					Namespace: testLBNamespace},
				Data: map[string]string{
					"config.yaml": `
internalTrafficPolicy: invalid
externalTrafficPolicy: Local
`,
				}}),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: func(m *MockPortExposer) {},
			expErr:               true,
			errMsg:               "failed to parse loadbalancer config",
		},
		{
			name:       "error fetching exposed services",
			expoConfig: types.ExpositionConfig{Enabled: true, DiscoverServices: true},
			inClientMock: testclient.NewClientBuilder().
				WithObjects(lbConfigMap).
				Build(),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: func(m *MockPortExposer) {},
			expErr:               true,
			errMsg:               "failed to list exposed services",
		},
		{
			// services with corrupted annotations are logged and skipped, not fatal
			name:       "success when one service has corrupted exposed port annotation",
			expoConfig: types.ExpositionConfig{Enabled: true, DiscoverServices: true},
			inClientMock: createDefaultLBClientMock(lbConfigMap, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{k8sv2.DoguLabelName: "testDogu"},
					Annotations: map[string]string{exposedPortServiceAnnotation: `INVALID`},
				},
				Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
			}),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               false,
		},
		{
			name:                 "success with ExpositionConfig enabled and services",
			expoConfig:           types.ExpositionConfig{Enabled: true, DiscoverServices: true},
			inClientMock:         createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               false,
		},
		{
			name:                 "success with ExpositionConfig enabled and expositions",
			expoConfig:           types.ExpositionConfig{Enabled: true, DiscoverExpositions: true},
			inClientMock:         createLBClientWithScheme(t, lbConfigMap, testExposition),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               false,
		},
		{
			name:       "error fetching expositions",
			expoConfig: types.ExpositionConfig{Enabled: true, DiscoverExpositions: true},
			inClientMock: testclient.NewClientBuilder().
				WithScheme(getScheme(t)).
				WithObjects(lbConfigMap).
				Build(),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: func(m *MockPortExposer) {},
			expErr:               true,
			errMsg:               "failed to list expositions",
		},
		{
			name: "error upserting loadbalancer - get current loadbalancer",
			inClientMock: testclient.NewClientBuilder().
				WithObjects(lbConfigMap, exposedService).
				WithIndex(&corev1.Service{}, exposedPortIndexKey, func(object client.Object) []string {
					return []string{"true"}
				}).
				WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*corev1.Service); ok {
							return assert.AnError
						}
						return c.Get(ctx, key, obj, opts...)
					},
				}).
				Build(),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               true,
			errMsg:               "failed to get service for loadbalancer",
		},
		{
			name: "error upserting loadbalancer - create loadbalancer",
			inClientMock: testclient.NewClientBuilder().
				WithObjects(lbConfigMap, exposedService).
				WithIndex(&corev1.Service{}, exposedPortIndexKey, func(object client.Object) []string {
					return []string{"true"}
				}).
				WithInterceptorFuncs(interceptor.Funcs{
					Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
						return assert.AnError
					},
				}).
				Build(),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               true,
			errMsg:               "failed to create new loadbalancer service",
		},
		{
			name: "error upserting loadbalancer - update loadbalancer",
			inClientMock: testclient.NewClientBuilder().
				WithObjects(lbConfigMap, exposedService, existingLB.ToK8sService()).
				WithIndex(&corev1.Service{}, exposedPortIndexKey, func(object client.Object) []string {
					return []string{"true"}
				}).
				WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
						return assert.AnError
					},
				}).
				Build(),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               true,
			errMsg:               "failed to update existing loadbalancer",
		},
		{
			// pre-seed with a plain Service (no LoadBalancer type) at the LB name — ParseLoadBalancer returns !ok
			name: "error upserting loadbalancer - parsing existing loadbalancer",
			inClientMock: createDefaultLBClientMock(lbConfigMap, exposedService, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: types.LoadbalancerName, Namespace: testLBNamespace},
			}),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               true,
			errMsg:               "could not parse existing service to LoadBalancer",
		},
		{
			name:            "error exposing ports in ingress controller",
			inClientMock:    createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock: createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: func(m *MockPortExposer) {
				m.EXPECT().ExposePorts(mock.Anything, mock.Anything).Return(assert.AnError)
			},
			expErr: true,
			errMsg: "failed to expose ports in ingress controller",
		},
		{
			// pre-seed the desired LB so lb.Equals(desired) is true and no Update is called
			name: "skip update when loadbalancer already in desired state",
			inClientMock: func() client.Client {
				selector := map[string]string{"service.name": "service"}
				desiredLB := types.CreateLoadBalancer(testLBNamespace, types.LoadbalancerConfig{
					ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
					InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyCluster,
				}, []types.Exposition{}, selector)
				return createDefaultLBClientMock(lbConfigMap, desiredLB.ToK8sService())
			}(),
			setupLoggerMock:      createDefaultLoadbalancerLoggerMock(),
			setupPortExposerMock: createNoErrorExposePorts(),
			expErr:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mock logger to catch log messages
			mockLogSink := NewMockLogSink(t)
			logger := logr.Logger{}
			logger = logger.WithSink(mockLogSink) // overwrite original logger with the given LogSink

			tt.setupLoggerMock(mockLogSink)

			// inject logger into context this way because the context search key is private to the logging framework
			valuedTestCtx := log.IntoContext(t.Context(), logger)

			portExposerMock := NewMockPortExposer(t)
			tt.setupPortExposerMock(portExposerMock)

			lbReconciler := &LoadBalancerReconciler{
				ExpositionConfig: tt.expoConfig,
				IngressSelector:  map[string]string{"service.name": "service"},
				Client:           tt.inClientMock,
				PortExposer:      portExposerMock,
			}

			request := ctrl.Request{NamespacedName: k8stypes.NamespacedName{Namespace: testLBNamespace, Name: types.LoadBalancerConfigName}}

			result, err := lbReconciler.Reconcile(valuedTestCtx, request)

			if tt.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tt.errMsg)

				return
			}

			require.NoError(t, err)
			require.Equal(t, ctrl.Result{}, result)
		})
	}
}

func createDefaultLoadbalancerConfig() types.LoadbalancerConfig {
	return types.LoadbalancerConfig{
		Annotations: map[string]string{
			"testKey": "testValue",
		},
		InternalTrafficPolicy: corev1.ServiceInternalTrafficPolicyCluster,
		ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyLocal,
	}
}

func createNoErrorExposePorts() func(m *MockPortExposer) {
	return func(m *MockPortExposer) {
		m.EXPECT().ExposePorts(mock.Anything, mock.Anything).Return(nil).Maybe()
	}
}

func createDefaultLBClientMock(obj ...client.Object) client.Client {
	return testclient.NewClientBuilder().
		WithObjects(obj...).
		WithStatusSubresource(&corev1.Service{}).
		WithIndex(&corev1.Service{}, exposedPortIndexKey, func(object client.Object) []string {
			return []string{"true"}
		}).
		Build()
}

func createDefaultLoadbalancerLoggerMock() func(m *MockLogSink) {
	return func(m *MockLogSink) {
		m.EXPECT().WithValues().Return(m)
		m.EXPECT().Enabled(mock.Anything).Return(true).Maybe()
		m.EXPECT().Info(0, mock.Anything).Return().Maybe()
		m.EXPECT().Error(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return().Maybe()
	}
}

func createLBClientWithScheme(t *testing.T, obj ...client.Object) client.Client {
	t.Helper()
	return testclient.NewClientBuilder().
		WithScheme(getScheme(t)).
		WithObjects(obj...).
		WithStatusSubresource(&corev1.Service{}, &expositionv1.Exposition{}).
		WithIndex(&corev1.Service{}, exposedPortIndexKey, func(object client.Object) []string {
			return []string{"true"}
		}).
		WithIndex(&expositionv1.Exposition{}, exposedPortIndexKey, func(object client.Object) []string {
			return []string{"true"}
		}).
		Build()
}

func Test_exposedPortExpositionPredicate(t *testing.T) {
	pred := exposedPortExpositionPredicate()

	expositionWithTCP := &expositionv1.Exposition{
		Spec: expositionv1.ExpositionSpec{
			TCP: []expositionv1.TCPEntry{{Name: "ssh", Service: "svc", Port: 22}},
		},
	}
	expositionWithUDP := &expositionv1.Exposition{
		Spec: expositionv1.ExpositionSpec{
			UDP: []expositionv1.UDPEntry{{Name: "dns", Service: "svc", Port: 53}},
		},
	}
	emptyExposition := &expositionv1.Exposition{}
	nonExposition := &corev1.ConfigMap{}

	t.Run("create", func(t *testing.T) {
		assert.True(t, pred.CreateFunc(event.CreateEvent{Object: expositionWithTCP}))
		assert.True(t, pred.CreateFunc(event.CreateEvent{Object: expositionWithUDP}))
		assert.False(t, pred.CreateFunc(event.CreateEvent{Object: emptyExposition}))
		assert.False(t, pred.CreateFunc(event.CreateEvent{Object: nonExposition}))
	})

	t.Run("delete", func(t *testing.T) {
		assert.True(t, pred.DeleteFunc(event.DeleteEvent{Object: expositionWithTCP}))
		assert.True(t, pred.DeleteFunc(event.DeleteEvent{Object: expositionWithUDP}))
		assert.False(t, pred.DeleteFunc(event.DeleteEvent{Object: emptyExposition}))
		assert.False(t, pred.DeleteFunc(event.DeleteEvent{Object: nonExposition}))
	})

	t.Run("generic", func(t *testing.T) {
		assert.True(t, pred.GenericFunc(event.GenericEvent{Object: expositionWithTCP}))
		assert.True(t, pred.GenericFunc(event.GenericEvent{Object: expositionWithUDP}))
		assert.False(t, pred.GenericFunc(event.GenericEvent{Object: emptyExposition}))
		assert.False(t, pred.GenericFunc(event.GenericEvent{Object: nonExposition}))
	})

	t.Run("update", func(t *testing.T) {
		tests := []struct {
			name      string
			objectOld client.Object
			objectNew client.Object
			want      bool
		}{
			{
				name:      "old is non-Exposition",
				objectOld: nonExposition,
				objectNew: expositionWithTCP,
				want:      false,
			},
			{
				name:      "new is non-Exposition",
				objectOld: expositionWithTCP,
				objectNew: nonExposition,
				want:      false,
			},
			{
				name:      "same TCP entries",
				objectOld: expositionWithTCP,
				objectNew: expositionWithTCP,
				want:      false,
			},
			{
				name:      "TCP count differs",
				objectOld: expositionWithTCP,
				objectNew: &expositionv1.Exposition{
					Spec: expositionv1.ExpositionSpec{
						TCP: []expositionv1.TCPEntry{
							{Name: "ssh", Service: "svc", Port: 22},
							{Name: "extra", Service: "svc", Port: 8080},
						},
					},
				},
				want: true,
			},
			{
				name:      "UDP count differs",
				objectOld: expositionWithUDP,
				objectNew: &expositionv1.Exposition{
					Spec: expositionv1.ExpositionSpec{
						UDP: []expositionv1.UDPEntry{
							{Name: "dns", Service: "svc", Port: 53},
							{Name: "extra", Service: "svc", Port: 5353},
						},
					},
				},
				want: true,
			},
			{
				name: "same TCP entries different order",
				objectOld: &expositionv1.Exposition{
					Spec: expositionv1.ExpositionSpec{
						TCP: []expositionv1.TCPEntry{
							{Name: "aaa", Service: "svc", Port: 1000},
							{Name: "bbb", Service: "svc", Port: 2000},
						},
					},
				},
				objectNew: &expositionv1.Exposition{
					Spec: expositionv1.ExpositionSpec{
						TCP: []expositionv1.TCPEntry{
							{Name: "bbb", Service: "svc", Port: 2000},
							{Name: "aaa", Service: "svc", Port: 1000},
						},
					},
				},
				want: false,
			},
			{
				name:      "TCP entry port changed",
				objectOld: expositionWithTCP,
				objectNew: &expositionv1.Exposition{
					Spec: expositionv1.ExpositionSpec{
						TCP: []expositionv1.TCPEntry{{Name: "ssh", Service: "svc", Port: 2222}},
					},
				},
				want: true,
			},
			{
				name:      "UDP entry port changed",
				objectOld: expositionWithUDP,
				objectNew: &expositionv1.Exposition{
					Spec: expositionv1.ExpositionSpec{
						UDP: []expositionv1.UDPEntry{{Name: "dns", Service: "svc", Port: 5353}},
					},
				},
				want: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := pred.UpdateFunc(event.UpdateEvent{
					ObjectOld: tt.objectOld,
					ObjectNew: tt.objectNew,
				})
				assert.Equal(t, tt.want, result)
			})
		}
	})
}

func Test_createLoadBalancerExposedPorts(t *testing.T) {
	tests := []struct {
		name     string
		input    []types.Exposition
		expected types.ExposedPorts
	}{
		{
			name:  "empty input returns defaults only",
			input: []types.Exposition{},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "custom port added alongside defaults",
			input: []types.Exposition{
				{TcpRoutes: types.ExposedPorts{{Name: "custom", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000}}},
			},
			expected: types.ExposedPorts{
				{Name: "custom", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "port 80 in input is stripped and replaced by default",
			input: []types.Exposition{
				{TcpRoutes: types.ExposedPorts{{Name: "myhttp", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80}}},
			},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "port 443 in input is stripped and replaced by default",
			input: []types.Exposition{
				{TcpRoutes: types.ExposedPorts{{Name: "myhttps", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443}}},
			},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "both 80 and 443 in input are stripped",
			input: []types.Exposition{
				{TcpRoutes: types.ExposedPorts{
					{Name: "p80", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
					{Name: "p443", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
				}},
			},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "80, 443, and custom port: 80+443 stripped, custom kept with defaults",
			input: []types.Exposition{
				{TcpRoutes: types.ExposedPorts{
					{Name: "p80", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
					{Name: "p443", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
					{Name: "custom", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
				}},
			},
			expected: types.ExposedPorts{
				{Name: "custom", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := types.CreateLoadBalancerExposedPorts(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoadBalancerReconciler_getExpositionsForServices(t *testing.T) {
	const expPortAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"

	lbConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: testLBNamespace}}

	t.Run("returns empty when DiscoverServices is disabled", func(t *testing.T) {
		r := &LoadBalancerReconciler{
			ExpositionConfig: types.ExpositionConfig{Enabled: true, DiscoverServices: false},
			Client:           createDefaultLBClientMock(lbConfigMap),
		}
		result, err := r.getExpositionsForServices(t.Context())
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns exposition with TCP routes from indexed service", func(t *testing.T) {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dogu-svc",
				Namespace: testLBNamespace,
				Labels:    map[string]string{k8sv2.DoguLabelName: "dogu"},
				Annotations: map[string]string{
					expPortAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
				},
			},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP, Ports: []corev1.ServicePort{
				{Protocol: corev1.ProtocolTCP, Port: 50000, TargetPort: intstr.FromInt32(50000)},
			}},
		}
		r := &LoadBalancerReconciler{
			ExpositionConfig: types.ExpositionConfig{Enabled: true, DiscoverServices: true},
			Client:           createDefaultLBClientMock(lbConfigMap, svc),
		}
		result, err := r.getExpositionsForServices(t.Context())
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "dogu-svc", result[0].Name)
		assert.Equal(t, testLBNamespace, result[0].Namespace)
		assert.Equal(t, types.ExposedPorts{
			{Name: "dogu-svc-expose-50000-50000", ServiceName: "dogu-svc", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
		}, result[0].TcpRoutes)
		assert.NotNil(t, result[0].SetOwner)
	})

	t.Run("logs and skips service with invalid annotation", func(t *testing.T) {
		badSvc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "bad-svc",
				Namespace:   testLBNamespace,
				Labels:      map[string]string{k8sv2.DoguLabelName: "bad"},
				Annotations: map[string]string{expPortAnnotation: `INVALID`},
			},
			Spec: corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		}
		r := &LoadBalancerReconciler{
			ExpositionConfig: types.ExpositionConfig{Enabled: true, DiscoverServices: true},
			Client:           createDefaultLBClientMock(lbConfigMap, badSvc),
		}
		result, err := r.getExpositionsForServices(t.Context())
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns error when list fails (no index registered)", func(t *testing.T) {
		r := &LoadBalancerReconciler{
			ExpositionConfig: types.ExpositionConfig{Enabled: true, DiscoverServices: true},
			Client:           testclient.NewClientBuilder().WithObjects(lbConfigMap).Build(),
		}
		_, err := r.getExpositionsForServices(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to list exposed services")
	})
}

func TestLoadBalancerReconciler_getExpositionsForExpositionCRs(t *testing.T) {
	lbConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.LoadBalancerConfigName, Namespace: testLBNamespace}}

	t.Run("returns empty when DiscoverExpositions is disabled", func(t *testing.T) {
		r := &LoadBalancerReconciler{
			ExpositionConfig: types.ExpositionConfig{Enabled: true, DiscoverExpositions: false},
			Client:           createLBClientWithScheme(t, lbConfigMap),
		}
		result, err := r.getExpositionsForExpositionCRs(t.Context())
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns exposition with TCP and UDP routes from indexed CR", func(t *testing.T) {
		exp := &expositionv1.Exposition{
			ObjectMeta: metav1.ObjectMeta{Name: "ssh-exp", Namespace: testLBNamespace},
			Spec: expositionv1.ExpositionSpec{
				TCP: []expositionv1.TCPEntry{{Name: "ssh", Service: "svc", Port: 22}},
				UDP: []expositionv1.UDPEntry{{Name: "dns", Service: "svc", Port: 53}},
			},
		}
		r := &LoadBalancerReconciler{
			ExpositionConfig: types.ExpositionConfig{Enabled: true, DiscoverExpositions: true},
			Client:           createLBClientWithScheme(t, lbConfigMap, exp),
		}
		result, err := r.getExpositionsForExpositionCRs(t.Context())
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "ssh-exp", result[0].Name)
		assert.Equal(t, testLBNamespace, result[0].Namespace)
		assert.Equal(t, types.ExposedPorts{
			{Name: "ssh-exp-ssh", ServiceName: "svc", Protocol: corev1.ProtocolTCP, ServicePort: 22, RequestedExternalPort: 22},
		}, result[0].TcpRoutes)
		assert.Equal(t, types.ExposedPorts{
			{Name: "ssh-exp-dns", ServiceName: "svc", Protocol: corev1.ProtocolUDP, ServicePort: 53, RequestedExternalPort: 53},
		}, result[0].UdpRoutes)
		assert.NotNil(t, result[0].SetOwner)
		assert.NotNil(t, result[0].SetCondition)
	})

	t.Run("returns error when list fails (no index registered)", func(t *testing.T) {
		r := &LoadBalancerReconciler{
			ExpositionConfig: types.ExpositionConfig{Enabled: true, DiscoverExpositions: true},
			Client:           testclient.NewClientBuilder().WithScheme(getScheme(t)).WithObjects(lbConfigMap).Build(),
		}
		_, err := r.getExpositionsForExpositionCRs(t.Context())
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to list expositions")
	})
}

func Test_exposedService(t *testing.T) {
	tcpPort := types.ExposedPort{Name: "ssh", Protocol: corev1.ProtocolTCP, ServicePort: 22, RequestedExternalPort: 22}
	udpPort := types.ExposedPort{Name: "dns", Protocol: corev1.ProtocolUDP, ServicePort: 53, RequestedExternalPort: 53}

	t.Run("HasExposedPorts", func(t *testing.T) {
		tests := []struct {
			name string
			es   exposedService
			want bool
		}{
			{name: "empty", es: exposedService{types.Exposition{}}, want: false},
			{name: "tcp only", es: exposedService{types.Exposition{TcpRoutes: types.ExposedPorts{tcpPort}}}, want: true},
			{name: "udp only", es: exposedService{types.Exposition{UdpRoutes: types.ExposedPorts{udpPort}}}, want: true},
			{name: "both", es: exposedService{types.Exposition{TcpRoutes: types.ExposedPorts{tcpPort}, UdpRoutes: types.ExposedPorts{udpPort}}}, want: true},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, tt.es.HasExposedPorts())
			})
		}
	})

	t.Run("GetExposedPorts", func(t *testing.T) {
		tests := []struct {
			name     string
			es       exposedService
			expected types.ExposedPorts
		}{
			{
				name:     "empty",
				es:       exposedService{types.Exposition{}},
				expected: types.ExposedPorts{},
			},
			{
				name:     "tcp only",
				es:       exposedService{types.Exposition{TcpRoutes: types.ExposedPorts{tcpPort}}},
				expected: types.ExposedPorts{tcpPort},
			},
			{
				name:     "udp only",
				es:       exposedService{types.Exposition{UdpRoutes: types.ExposedPorts{udpPort}}},
				expected: types.ExposedPorts{udpPort},
			},
			{
				name:     "both",
				es:       exposedService{types.Exposition{TcpRoutes: types.ExposedPorts{tcpPort}, UdpRoutes: types.ExposedPorts{udpPort}}},
				expected: types.ExposedPorts{tcpPort, udpPort},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, tt.es.GetExposedPorts())
			})
		}
	})
}

func Test_checkPortCollisions(t *testing.T) {
	tcpPort22 := types.ExposedPort{Name: "ssh", Protocol: corev1.ProtocolTCP, ServicePort: 22, RequestedExternalPort: 22}
	tcpPort53 := types.ExposedPort{Name: "dns-tcp", Protocol: corev1.ProtocolTCP, ServicePort: 53, RequestedExternalPort: 53}
	udpPort53 := types.ExposedPort{Name: "dns-udp", Protocol: corev1.ProtocolUDP, ServicePort: 53, RequestedExternalPort: 53}
	tcpPort80 := types.ExposedPort{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80}

	t.Run("empty input returns empty valid list and empty collision map", func(t *testing.T) {
		valid, collisions := checkPortCollisions([]types.Exposition{})
		assert.Empty(t, valid)
		assert.Empty(t, collisions)
	})

	t.Run("no collisions returns all expositions and empty collision map", func(t *testing.T) {
		input := []types.Exposition{
			{Name: "a", TcpRoutes: types.ExposedPorts{tcpPort22}},
			{Name: "b", TcpRoutes: types.ExposedPorts{tcpPort80}},
			{Name: "c", UdpRoutes: types.ExposedPorts{udpPort53}},
		}
		valid, collisions := checkPortCollisions(input)
		assert.Len(t, valid, 3)
		assert.Empty(t, collisions)
	})

	t.Run("two expositions claiming same TCP port are both excluded", func(t *testing.T) {
		input := []types.Exposition{
			{Name: "a", TcpRoutes: types.ExposedPorts{tcpPort22}},
			{Name: "b", TcpRoutes: types.ExposedPorts{tcpPort22}},
		}
		valid, collisions := checkPortCollisions(input)
		assert.Empty(t, valid)
		assert.Len(t, collisions, 2)
	})

	t.Run("same port number with different protocols is not a collision", func(t *testing.T) {
		input := []types.Exposition{
			{Name: "a", TcpRoutes: types.ExposedPorts{tcpPort53}},
			{Name: "b", UdpRoutes: types.ExposedPorts{udpPort53}},
		}
		valid, collisions := checkPortCollisions(input)
		assert.Len(t, valid, 2)
		assert.Empty(t, collisions)
	})

	t.Run("partial collision: colliders excluded, non-collider returned", func(t *testing.T) {
		input := []types.Exposition{
			{Name: "a", TcpRoutes: types.ExposedPorts{tcpPort22}},
			{Name: "b", TcpRoutes: types.ExposedPorts{tcpPort22}},
			{Name: "c", TcpRoutes: types.ExposedPorts{tcpPort80}},
		}
		valid, collisions := checkPortCollisions(input)
		require.Len(t, valid, 1)
		assert.Equal(t, "c", valid[0].Name)
		assert.Len(t, collisions, 2)
	})
}

func Test_collisionMessage(t *testing.T) {
	t.Run("single key", func(t *testing.T) {
		msg := collisionMessage([]portProtocolKey{{port: 22, protocol: corev1.ProtocolTCP}})
		assert.Equal(t, "port collision for: TCP/22", msg)
	})

	t.Run("multiple keys contain all port/protocol pairs", func(t *testing.T) {
		msg := collisionMessage([]portProtocolKey{
			{port: 22, protocol: corev1.ProtocolTCP},
			{port: 53, protocol: corev1.ProtocolUDP},
		})
		assert.Contains(t, msg, "port collision for:")
		assert.Contains(t, msg, "TCP/22")
		assert.Contains(t, msg, "UDP/53")
	})
}

func Test_setPortsAllocatedConditionError(t *testing.T) {
	testCtx := context.TODO()

	t.Run("empty collision map is a no-op", func(t *testing.T) {
		err := setPortsAllocatedConditionError(testCtx, map[*types.Exposition][]portProtocolKey{})
		assert.NoError(t, err)
	})

	t.Run("exposition with nil SetCondition is skipped", func(t *testing.T) {
		e := &types.Exposition{Name: "no-setter"}
		err := setPortsAllocatedConditionError(testCtx, map[*types.Exposition][]portProtocolKey{
			e: {{port: 22, protocol: corev1.ProtocolTCP}},
		})

		assert.NoError(t, err)
	})

	t.Run("SetCondition success does not return an error", func(t *testing.T) {
		e := &types.Exposition{
			Name: "has-setter",
			SetCondition: func(_ context.Context, _ string, _ bool, _, _ string) error {
				return nil
			},
		}
		err := setPortsAllocatedConditionError(testCtx, map[*types.Exposition][]portProtocolKey{
			e: {{port: 22, protocol: corev1.ProtocolTCP}},
		})

		assert.NoError(t, err)
	})

	t.Run("SetCondition failure is returned", func(t *testing.T) {
		e := &types.Exposition{
			Name: "error-setter",
			SetCondition: func(_ context.Context, _ string, _ bool, _, _ string) error {
				return assert.AnError
			},
		}
		err := setPortsAllocatedConditionError(testCtx, map[*types.Exposition][]portProtocolKey{
			e: {{port: 22, protocol: corev1.ProtocolTCP}},
		})

		assert.Error(t, err)
	})
}

func Test_setPortsAllocatedCondition(t *testing.T) {
	ctx := context.TODO()

	t.Run("empty list is a no-op", func(t *testing.T) {
		err := setPortsAllocatedCondition(ctx, []types.Exposition{})
		assert.NoError(t, err)
	})

	t.Run("exposition with nil SetCondition is skipped", func(t *testing.T) {
		err := setPortsAllocatedCondition(ctx, []types.Exposition{{Name: "no-setter"}})
		assert.NoError(t, err)
	})

	t.Run("SetCondition success does return an error", func(t *testing.T) {
		err := setPortsAllocatedCondition(ctx, []types.Exposition{{
			Name: "has-setter",
			SetCondition: func(_ context.Context, _ string, _ bool, _, _ string) error {
				return nil
			},
		}})

		assert.NoError(t, err)
	})

	t.Run("SetCondition failure is logged", func(t *testing.T) {
		err := setPortsAllocatedCondition(ctx, []types.Exposition{{
			Name: "error-setter",
			SetCondition: func(_ context.Context, _ string, _ bool, _, _ string) error {
				return assert.AnError
			},
		}})

		assert.Error(t, err)
	})
}
