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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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

	expPortServicePredicate := exposedPortServicePredicate()

	t.Run("reconcile exposed port service on create", func(t *testing.T) {
		assert.True(t, expPortServicePredicate.CreateFunc(event.CreateEvent{Object: &corev1.Service{
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
				Type: corev1.ServiceTypeClusterIP,
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
				Type: corev1.ServiceTypeClusterIP,
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

		t.Run("ignore when getting exposed ports fails on oldObject", func(t *testing.T) {
			assert.False(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
				ObjectOld: invalidExposedPorts,
				ObjectNew: exposedDoguService,
			}))
		})

		t.Run("ignore when getting exposed ports fails on newObject", func(t *testing.T) {
			assert.False(t, expPortServicePredicate.UpdateFunc(event.UpdateEvent{
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
			Labels: map[string]string{
				k8sv2.DoguLabelName: "testDogu",
			},
			Annotations: map[string]string{
				exposedPortServiceAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
		}}
	testExposition := &expositionv1.Exposition{
		ObjectMeta: metav1.ObjectMeta{Name: "test-exposition", Namespace: testLBNamespace},
		Spec: expositionv1.ExpositionSpec{
			TCP: []expositionv1.TCPEntry{{Name: "ssh", Service: "test-svc", Port: 22}},
		},
	}

	tests := []struct {
		name                       string
		expoConfig                 ExpositionConfig
		inClientMock               client.Client
		setupLoggerMock            func(m *MockLogSink)
		setupIngressControllerMock func(m *MockIngressController)
		setupServiceClientMock     func(m *mockServiceClient)
		expErr                     bool
		errMsg                     string
	}{
		{
			name:                       "create new loadbalancer",
			inClientMock:               createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock:     createSvcNewLoadbalancer(false),
			expErr:                     false,
			errMsg:                     "",
		},
		{
			name:                       "update loadbalancer",
			inClientMock:               createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock:     createSvcUpdateLoadbalancer(false),
			expErr:                     false,
			errMsg:                     "",
		},
		{
			name:                       "error client get loadbalancer config map",
			inClientMock:               createDefaultLBClientMock(),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: func(m *MockIngressController) {},
			setupServiceClientMock:     func(m *mockServiceClient) {},
			expErr:                     true,
			errMsg:                     "failed to get config map for loadbalancer",
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
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: func(m *MockIngressController) {},
			setupServiceClientMock:     func(m *mockServiceClient) {},
			expErr:                     true,
			errMsg:                     "failed to parse loadbalancer config",
		},
		{
			name:       "error fetching exposed services",
			expoConfig: ExpositionConfig{ExposePorts: true, DiscoverServices: true},
			inClientMock: testclient.NewClientBuilder().
				WithObjects(lbConfigMap).
				Build(),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: func(m *MockIngressController) {},
			setupServiceClientMock:     func(m *mockServiceClient) {},
			expErr:                     true,
			errMsg:                     "failed to list exposed services",
		},
		{
			name:       "error fetching exposed ports of service",
			expoConfig: ExpositionConfig{ExposePorts: true, DiscoverServices: true},
			inClientMock: createDefaultLBClientMock(lbConfigMap, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						k8sv2.DoguLabelName: "testDogu",
					},
					Annotations: map[string]string{
						exposedPortServiceAnnotation: `INVALID`,
					},
				},
				Spec: corev1.ServiceSpec{
					Type: corev1.ServiceTypeClusterIP,
				}}),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: func(m *MockIngressController) {},
			setupServiceClientMock:     func(m *mockServiceClient) {},
			expErr:                     true,
			errMsg:                     "failed to get exposed ports from services",
		},
		{
			name:                       "success with ExpositionConfig enabled and services",
			expoConfig:                 ExpositionConfig{ExposePorts: true, DiscoverServices: true},
			inClientMock:               createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock:     createSvcNewLoadbalancer(false),
			expErr:                     false,
		},
		{
			name:                       "success with ExpositionConfig enabled and expositions",
			expoConfig:                 ExpositionConfig{ExposePorts: true, DiscoverExpositions: true},
			inClientMock:               createLBClientWithScheme(t, lbConfigMap, testExposition),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock:     createSvcNewLoadbalancer(false),
			expErr:                     false,
		},
		{
			name:       "error fetching expositions",
			expoConfig: ExpositionConfig{ExposePorts: true, DiscoverExpositions: true},
			inClientMock: testclient.NewClientBuilder().
				WithScheme(getScheme(t)).
				WithObjects(lbConfigMap).
				Build(),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: func(m *MockIngressController) {},
			setupServiceClientMock:     func(m *mockServiceClient) {},
			expErr:                     true,
			errMsg:                     "failed to list expositions",
		},
		{
			name:                       "error upserting loadbalancer - get current loadbalancer",
			inClientMock:               createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock: func(m *mockServiceClient) {
				m.EXPECT().Get(mock.Anything, types.LoadbalancerName, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to get service for loadbalancer",
		},
		{
			name:                       "error upserting loadbalancer - create loadbalancer",
			inClientMock:               createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock:     createSvcNewLoadbalancer(true),
			expErr:                     true,
			errMsg:                     "failed to create new loadbalancer service",
		},
		{
			name:                       "error upserting loadbalancer - update loadbalancer",
			inClientMock:               createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock:     createSvcUpdateLoadbalancer(true),
			expErr:                     true,
			errMsg:                     "failed to update existing loadbalancer",
		},
		{
			name:                       "error upserting loadbalancer - parsing existing loadbalancer",
			inClientMock:               createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock:            createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: createNoErrorExposePorts(),
			setupServiceClientMock: func(m *mockServiceClient) {
				m.EXPECT().Get(mock.Anything, types.LoadbalancerName, mock.Anything).Return(&corev1.Service{}, nil)
			},
			expErr: true,
			errMsg: "could not parse existing service to LoadBalancer",
		},
		{
			name:            "error exposing ports in ingress controller",
			inClientMock:    createDefaultLBClientMock(lbConfigMap, exposedService),
			setupLoggerMock: createDefaultLoadbalancerLoggerMock(),
			setupIngressControllerMock: func(m *MockIngressController) {
				m.EXPECT().GetSelector().Return(map[string]string{
					"service.name": "service",
				})
				m.EXPECT().ExposePorts(mock.Anything, testLBNamespace, mock.Anything).Return(assert.AnError)
			},
			setupServiceClientMock: createSvcNewLoadbalancer(false),
			expErr:                 true,
			errMsg:                 "failed to update exposed ports in ingress controller",
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

			ingressControllerMock := NewMockIngressController(t)
			tt.setupIngressControllerMock(ingressControllerMock)

			serviceClientMock := newMockServiceClient(t)
			tt.setupServiceClientMock(serviceClientMock)

			lbReconciler := &LoadBalancerReconciler{
				ExpositionConfig:  tt.expoConfig,
				Client:            tt.inClientMock,
				IngressController: ingressControllerMock,
				SvcClient:         serviceClientMock,
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

func createNoErrorExposePorts() func(m *MockIngressController) {
	return func(m *MockIngressController) {
		m.EXPECT().GetSelector().Return(map[string]string{
			"service.name": "service",
		}).Maybe()
		m.EXPECT().ExposePorts(mock.Anything, testLBNamespace, mock.Anything).Return(nil).Maybe()
	}
}

func createSvcNewLoadbalancer(returnErr bool) func(m *mockServiceClient) {
	return func(m *mockServiceClient) {
		m.EXPECT().Get(mock.Anything, types.LoadbalancerName, mock.Anything).Return(nil, apierrors.NewNotFound(schema.GroupResource{}, "error"))

		if returnErr {
			m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
		} else {
			m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.LoadbalancerName,
					Namespace: testLBNamespace,
					UID:       types.LoadbalancerName,
				}}, nil)
		}
	}
}

func createSvcUpdateLoadbalancer(returnErr bool) func(m *mockServiceClient) {
	return func(m *mockServiceClient) {
		defaultLB := types.CreateLoadBalancer(testLBNamespace, createDefaultLoadbalancerConfig(), types.ExposedPorts{}, map[string]string{"test": "test"})

		m.EXPECT().Get(mock.Anything, types.LoadbalancerName, mock.Anything).Return(defaultLB.ToK8sService(), nil)

		if returnErr {
			m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
		} else {
			m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(&corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      types.LoadbalancerName,
					Namespace: testLBNamespace,
					UID:       types.LoadbalancerName,
				}}, nil)
		}

	}
}

func createDefaultLBClientMock(obj ...client.Object) client.Client {
	return testclient.NewClientBuilder().
		WithObjects(obj...).
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
	}
}

func createLBClientWithScheme(t *testing.T, obj ...client.Object) client.Client {
	t.Helper()
	return testclient.NewClientBuilder().
		WithScheme(getScheme(t)).
		WithObjects(obj...).
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
		input    types.ExposedPorts
		expected types.ExposedPorts
	}{
		{
			name:  "empty input returns defaults only",
			input: types.ExposedPorts{},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "custom port added alongside defaults",
			input: types.ExposedPorts{
				{Name: "custom", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
			},
			expected: types.ExposedPorts{
				{Name: "custom", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "port 80 in input is stripped and replaced by default",
			input: types.ExposedPorts{
				{Name: "myhttp", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
			},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "port 443 in input is stripped and replaced by default",
			input: types.ExposedPorts{
				{Name: "myhttps", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "both 80 and 443 in input are stripped",
			input: types.ExposedPorts{
				{Name: "p80", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "p443", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
			expected: types.ExposedPorts{
				{Name: "http", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "https", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
			},
		},
		{
			name: "80, 443, and custom port: 80+443 stripped, custom kept with defaults",
			input: types.ExposedPorts{
				{Name: "p80", Protocol: corev1.ProtocolTCP, ServicePort: 80, RequestedExternalPort: 80},
				{Name: "p443", Protocol: corev1.ProtocolTCP, ServicePort: 443, RequestedExternalPort: 443},
				{Name: "custom", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
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
			result := createLoadBalancerExposedPorts(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_getExposedPorts_generic(t *testing.T) {
	const expPortAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"

	tests := []struct {
		name      string
		input     []types.Service
		expected  types.ExposedPorts
		expErr    bool
		errSubstr string
	}{
		{
			name:     "empty list returns empty result",
			input:    []types.Service{},
			expected: types.ExposedPorts{},
		},
		{
			name: "single service with valid ports",
			input: []types.Service{
				types.Service(corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-svc",
						Annotations: map[string]string{
							expPortAnnotation: `[{"protocol":"tcp","port":50000,"targetPort":50000}]`,
						},
					},
				}),
			},
			expected: types.ExposedPorts{
				{Name: "test-svc-50000", ServiceName: "test-svc", Protocol: corev1.ProtocolTCP, ServicePort: 50000, RequestedExternalPort: 50000},
			},
		},
		{
			name: "multiple services ports are combined",
			input: []types.Service{
				types.Service(corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "svc-a",
						Annotations: map[string]string{
							expPortAnnotation: `[{"protocol":"tcp","port":1000,"targetPort":1000}]`,
						},
					},
				}),
				types.Service(corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "svc-b",
						Annotations: map[string]string{
							expPortAnnotation: `[{"protocol":"udp","port":2000,"targetPort":2000}]`,
						},
					},
				}),
			},
			expected: types.ExposedPorts{
				{Name: "svc-a-1000", ServiceName: "svc-a", Protocol: corev1.ProtocolTCP, ServicePort: 1000, RequestedExternalPort: 1000},
				{Name: "svc-b-2000", ServiceName: "svc-b", Protocol: corev1.ProtocolUDP, ServicePort: 2000, RequestedExternalPort: 2000},
			},
		},
		{
			name: "service with invalid JSON annotation returns error",
			input: []types.Service{
				types.Service(corev1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name: "bad-svc",
						Annotations: map[string]string{
							expPortAnnotation: `INVALID JSON`,
						},
					},
				}),
			},
			expected:  nil,
			expErr:    true,
			errSubstr: "failed to get exposed ports from object with type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := getExposedPorts(tt.input)

			if tt.expErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.errSubstr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
