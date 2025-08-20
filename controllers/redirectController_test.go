package controllers

import (
	"context"
	"testing"

	"github.com/cloudogu/k8s-registry-lib/config"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	primaryFQDN           = "ecosystem.cloudogu.com"
	globalConfigName      = "global-config"
	globalConfigNamespace = "default"
)

func Test_globalConfigPredicate(t *testing.T) {
	globalCfgPredicateFuncs := globalConfigPredicate()

	t.Run("reconcile global-config", func(t *testing.T) {
		assert.True(t, globalCfgPredicateFuncs.CreateFunc(event.CreateEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "global-config", Namespace: "default"}}}))
		assert.True(t, globalCfgPredicateFuncs.UpdateFunc(event.UpdateEvent{
			ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "global-config", Namespace: "default"}},
			ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "global-config", Namespace: "default"}},
		}))
		assert.True(t, globalCfgPredicateFuncs.DeleteFunc(event.DeleteEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "global-config", Namespace: "default"}}}))
		assert.True(t, globalCfgPredicateFuncs.GenericFunc(event.GenericEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "global-config", Namespace: "default"}}}))
	})

	t.Run("ignore any other config map", func(t *testing.T) {
		assert.False(t, globalCfgPredicateFuncs.CreateFunc(event.CreateEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}}}))
		assert.False(t, globalCfgPredicateFuncs.UpdateFunc(event.UpdateEvent{
			ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}},
			ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}},
		}))
		assert.False(t, globalCfgPredicateFuncs.DeleteFunc(event.DeleteEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}}}))
		assert.False(t, globalCfgPredicateFuncs.GenericFunc(event.GenericEvent{Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: "default"}}}))
	})
}

func Test_redirectIngressPredicate(t *testing.T) {
	redirectIngressPredicateFuncs := redirectIngressPredicate()

	t.Run("ignore created redirect ingress", func(t *testing.T) {
		assert.False(t, redirectIngressPredicateFuncs.CreateFunc(event.CreateEvent{Object: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}}}))
	})

	t.Run("reconcile when updated redirected ingress are different", func(t *testing.T) {
		assert.True(t, redirectIngressPredicateFuncs.UpdateFunc(event.UpdateEvent{
			ObjectOld: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}},
			ObjectNew: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}, Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "other-host"}}}},
		}))

		assert.True(t, redirectIngressPredicateFuncs.UpdateFunc(event.UpdateEvent{
			ObjectOld: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}},
			ObjectNew: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default", Annotations: map[string]string{"nginx.ingress.kubernetes.io/rewrite-target": "/"}}},
		}))
	})

	t.Run("ignore when updated redirected ingress are equal", func(t *testing.T) {
		assert.False(t, redirectIngressPredicateFuncs.UpdateFunc(event.UpdateEvent{
			ObjectOld: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}},
			ObjectNew: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}},
		}))
	})

	t.Run("reconcile when deleted redirected ingress", func(t *testing.T) {
		assert.True(t, redirectIngressPredicateFuncs.DeleteFunc(event.DeleteEvent{Object: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}}}))
	})

	t.Run("ignore generic event", func(t *testing.T) {
		assert.False(t, redirectIngressPredicateFuncs.GenericFunc(event.GenericEvent{Object: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: redirectObjectName, Namespace: "default"}}}))
	})

	t.Run("ignore any other ingress", func(t *testing.T) {
		assert.False(t, redirectIngressPredicateFuncs.CreateFunc(event.CreateEvent{Object: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "other-ingress", Namespace: "default"}}}))
		assert.False(t, redirectIngressPredicateFuncs.UpdateFunc(event.UpdateEvent{
			ObjectOld: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "other-ingress", Namespace: "default"}},
			ObjectNew: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "other-ingress", Namespace: "default"}},
		}))
		assert.False(t, redirectIngressPredicateFuncs.DeleteFunc(event.DeleteEvent{Object: &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "other-ingress", Namespace: "default"}}}))
	})

}

func TestRedirectReconciler_Reconcile(t *testing.T) {
	globalConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: globalConfigName, Namespace: globalConfigNamespace}}

	tests := []struct {
		name                  string
		inClientMock          client.Client
		setupLoggerMock       func(m *MockLogSink)
		setupGlobalConfigMock func(m *MockGlobalConfigRepository)
		setupRedirectorMock   func(t *testing.T, m *MockAlternativeFQDNRedirector)
		expErr                bool
		errMsg                string
	}{
		{
			name:            "redirect with single alternative fqdn without certificate",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: "alt.cloudogu.com",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt.cloudogu.com", certEcosystemSecretName},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "redirect with single alternative fqdn with certificate",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: "alt.cloudogu.com:my-cert-secret",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt.cloudogu.com", "my-cert-secret"},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "redirect with multiple alternative fqdns without certificate",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: "alt1.cloudogu.com,alt2.cloudogu.com",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt1.cloudogu.com", certEcosystemSecretName},
				{"alt2.cloudogu.com", certEcosystemSecretName},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "redirect with multiple alternative fqdns with own certificates",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: "alt1.cloudogu.com:my-cert-secret,alt2.cloudogu.com:my-cert-secret2",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt1.cloudogu.com", "my-cert-secret"},
				{"alt2.cloudogu.com", "my-cert-secret2"},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "redirect multiple alternative fqdns that reference same certificate",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: "alt1.cloudogu.com:my-cert-secret,alt2.cloudogu.com:my-cert-secret",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt1.cloudogu.com", "my-cert-secret"},
				{"alt2.cloudogu.com", "my-cert-secret"},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "redirect multiple alternative fqdns with own certificate and without certificate",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: "alt1.cloudogu.com,alt2.cloudogu.com:my-cert-secret,alt3.cloudogu.com,alt4.cloudogu.com:my-cert-secret,alt5.cloudogu.com:my-second-cert-secret",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt1.cloudogu.com", certEcosystemSecretName},
				{"alt2.cloudogu.com", "my-cert-secret"},
				{"alt3.cloudogu.com", certEcosystemSecretName},
				{"alt4.cloudogu.com", "my-cert-secret"},
				{"alt5.cloudogu.com", "my-second-cert-secret"},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "ignore white space in alternative fqdns entry",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: " alt1.cloudogu.com    ,alt2.cloudogu.com  :  my-cert-secret,alt3.cloudogu.com  ,alt4.cloudogu.com:  my-cert-secret,  alt5.cloudogu.com  :my-second-cert-secret   ",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt1.cloudogu.com", certEcosystemSecretName},
				{"alt2.cloudogu.com", "my-cert-secret"},
				{"alt3.cloudogu.com", certEcosystemSecretName},
				{"alt4.cloudogu.com", "my-cert-secret"},
				{"alt5.cloudogu.com", "my-second-cert-secret"},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "ignore alternative fqdns that reference multiple certificates",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey:     primaryFQDN,
				alternativeFQDNKey: " alt1.cloudogu.com,alt2.cloudogu.com:my-cert-secret:my-other-secret",
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{
				{"alt1.cloudogu.com", certEcosystemSecretName},
			}),
			expErr: false,
			errMsg: "",
		},
		{
			name:            "pass empty alternative fqdn map when alternative fqdn string is empty",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey: primaryFQDN,
			}),
			setupRedirectorMock: createRedirectorMockWithExpectedRedirectAltFQDNList([]types.AlternativeFQDN{}),
			expErr:              false,
			errMsg:              "",
		},
		{
			name:            "be able to set OwnerReference on Redirect object to global config map",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey: primaryFQDN,
			}),
			setupRedirectorMock: func(t *testing.T, m *MockAlternativeFQDNRedirector) {
				m.EXPECT().RedirectAlternativeFQDN(mock.Anything, globalConfigNamespace, redirectObjectName, primaryFQDN, mock.Anything, mock.Anything).
					Run(func(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(metav1.Object) error) {
						tmpCM := &corev1.ConfigMap{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "testCM",
								Namespace: globalConfigNamespace,
							},
						}
						err := setOwner(tmpCM)
						require.NoError(t, err)

						ownRef := tmpCM.GetOwnerReferences()
						require.Len(t, ownRef, 1)

						require.Equal(t, globalConfigMap.Name, ownRef[0].Name)
						require.True(t, *ownRef[0].Controller)
						require.False(t, *ownRef[0].BlockOwnerDeletion)
					}).
					Return(nil)
			},
			expErr: false,
			errMsg: "",
		},
		{
			name:                  "return error when global config map cannot be received",
			inClientMock:          testclient.NewClientBuilder().Build(),
			setupLoggerMock:       createDefaultLoggerMock(),
			setupGlobalConfigMock: func(m *MockGlobalConfigRepository) {},
			setupRedirectorMock:   func(t *testing.T, m *MockAlternativeFQDNRedirector) {},
			expErr:                true,
			errMsg:                "failed to get global config map",
		},
		{
			name:            "return error when global config cannot be received",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: func(m *MockGlobalConfigRepository) {
				m.EXPECT().Get(mock.Anything).Return(config.GlobalConfig{}, assert.AnError)
			},
			setupRedirectorMock: func(t *testing.T, m *MockAlternativeFQDNRedirector) {},
			expErr:              true,
			errMsg:              "failed to get global config",
		},
		{
			name:                  "return error when fqdn key is not set in global config",
			inClientMock:          createDefaultClientMock(globalConfigMap),
			setupLoggerMock:       createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{}),
			setupRedirectorMock:   func(t *testing.T, m *MockAlternativeFQDNRedirector) {},
			expErr:                true,
			errMsg:                "fqdn not found in global config",
		},
		{
			name:            "return error when fqdn cannot be redirected",
			inClientMock:    createDefaultClientMock(globalConfigMap),
			setupLoggerMock: createDefaultLoggerMock(),
			setupGlobalConfigMock: createGlobalConfigWithEntries(map[config.Key]config.Value{
				primaryFQDNKey: primaryFQDN,
			}),
			setupRedirectorMock: func(t *testing.T, m *MockAlternativeFQDNRedirector) {
				m.EXPECT().RedirectAlternativeFQDN(mock.Anything, globalConfigNamespace, redirectObjectName, primaryFQDN, mock.Anything, mock.Anything).Return(assert.AnError)
			},
			expErr: true,
			errMsg: "failed to redirect alternative fqdns",
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
			valuedTestCtx := log.IntoContext(testCtx, logger)

			globalConfigRepoMock := NewMockGlobalConfigRepository(t)
			tt.setupGlobalConfigMock(globalConfigRepoMock)

			redirectorMock := NewMockAlternativeFQDNRedirector(t)
			tt.setupRedirectorMock(t, redirectorMock)

			redirectReconciler := &RedirectReconciler{
				Client:             tt.inClientMock,
				GlobalConfigGetter: globalConfigRepoMock,
				Redirector:         redirectorMock,
			}

			request := ctrl.Request{NamespacedName: k8stypes.NamespacedName{Namespace: globalConfigNamespace, Name: globalConfigName}}

			result, err := redirectReconciler.Reconcile(valuedTestCtx, request)

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

func createGlobalConfigWithEntries(entries map[config.Key]config.Value) func(m *MockGlobalConfigRepository) {
	return func(m *MockGlobalConfigRepository) {
		m.EXPECT().Get(mock.Anything).Return(config.CreateGlobalConfig(entries), nil)
	}
}

func createRedirectorMockWithExpectedRedirectAltFQDNList(expAltFQDNList []types.AlternativeFQDN) func(t *testing.T, m *MockAlternativeFQDNRedirector) {
	return func(t *testing.T, m *MockAlternativeFQDNRedirector) {
		m.EXPECT().RedirectAlternativeFQDN(mock.Anything, globalConfigNamespace, redirectObjectName, primaryFQDN, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(metav1.Object) error) {
				require.ElementsMatch(t, expAltFQDNList, altFQDNList)
			}).Return(nil)
	}
}

func createDefaultClientMock(o client.Object) client.Client {
	return testclient.NewClientBuilder().WithObjects(o).Build()
}

func createDefaultLoggerMock() func(m *MockLogSink) {
	return func(m *MockLogSink) {
		m.EXPECT().WithValues().Return(m)
		m.EXPECT().Enabled(mock.Anything).Return(true)
		m.EXPECT().Info(0, mock.Anything).Return()
	}
}
