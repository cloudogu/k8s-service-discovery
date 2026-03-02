package traefik

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/expose"
	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	traefikapi "github.com/traefik/traefik/v3/pkg/provider/kubernetes/crd/traefikio/v1alpha1"
	v1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ingressName            = "testIngress"
	ingressClassName       = "testIngressClass"
	namespace              = "testNamespace"
	testFqdn               = "test.testFqdn"
	defaultCertificateName = "default-certificate"
)

func TestIngressRedirector_RedirectAlternativeFQDN(t *testing.T) {
	tests := []struct {
		name          string
		inAltFQDNList []types.AlternativeFQDN
		inSetOwner    func(targetObject metav1.Object) error
		setupMock     func(*mockIngressInterface, *mockTraefikInterface, []types.AlternativeFQDN)
		expErr        bool
		errMsg        string
	}{
		{
			name: "create new redirect ingress with single alternative fqdn",
			inAltFQDNList: []types.AlternativeFQDN{
				{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
		},
		{
			name: "create new redirect ingress with multiple alternative fqdns and single certificate",
			inAltFQDNList: []types.AlternativeFQDN{
				{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
				{FQDN: "test2.testFqdn", CertificateSecretName: defaultCertificateName},
				{FQDN: "test3.testFqdn", CertificateSecretName: defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
		},
		{
			name: "create new redirect ingress with multiple alternative fqdns with different certificates",
			inAltFQDNList: []types.AlternativeFQDN{
				{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
				{FQDN: "test2.testFqdn", CertificateSecretName: "testCertificate2"},
				{FQDN: "test3.testFqdn", CertificateSecretName: "testCertificate3"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
		},
		{
			name: "update redirect ingress when already exists",
			inAltFQDNList: []types.AlternativeFQDN{
				{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
				{FQDN: "test2.testFqdn", CertificateSecretName: "testCertificate2"},
				{FQDN: "test3.testFqdn", CertificateSecretName: "testCertificate3"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectUpdateAlreadyExists(t),
			expErr:     false,
		},
		{
			name:          "delete redirect ingress when alternative fqdn list is empty and redirect ingress exists",
			inAltFQDNList: []types.AlternativeFQDN{},
			inSetOwner:    func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, t *mockTraefikInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(nil)
			},
			expErr: false,
		},
		{
			name:          "return no error when alternative fqdn list is empty and redirect ingress does not exist",
			inAltFQDNList: []types.AlternativeFQDN{},
			inSetOwner:    func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, t *mockTraefikInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(apierrors.NewNotFound(v1.Resource("ingress"), ingressName))
			},
			expErr: false,
		},
		{
			name: "return error when redirect ingress cannot be created",
			inAltFQDNList: []types.AlternativeFQDN{
				{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, t *mockTraefikInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to upsert redirect ingress",
		},
		{
			name: "return error when redirect ingress cannot be updated",
			inAltFQDNList: []types.AlternativeFQDN{
				{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, t *mockTraefikInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))
				m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to upsert redirect ingress",
		},
		{
			name:          "return error when redirect ingress cannot be deleted",
			inAltFQDNList: []types.AlternativeFQDN{},
			inSetOwner:    func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, t *mockTraefikInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(assert.AnError)
			},
			expErr: true,
			errMsg: "failed to delete redirect ingress",
		},
		{
			name: "return error when owner cannot be set",
			inAltFQDNList: []types.AlternativeFQDN{
				{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return assert.AnError },
			setupMock:  func(m *mockIngressInterface, t *mockTraefikInterface, l []types.AlternativeFQDN) {},
			expErr:     true,
			errMsg:     "failed to set owner for redirect ingress",
		},
		//{
		//	name: "return error when middleware cannot be created",
		//	inAltFQDNList: []types.AlternativeFQDN{
		//		{FQDN: "test.testFqdn", CertificateSecretName: defaultCertificateName},
		//	},
		//	inSetOwner: func(targetObject metav1.Object) error { return nil },
		//	setupMock: func(m *mockIngressInterface, t *mockTraefikInterface, l []types.AlternativeFQDN) {
		//		ingress := &v1.Ingress{
		//			ObjectMeta: metav1.ObjectMeta{
		//				Name: ingressName,
		//				UID:  "test-uid",
		//			},
		//		}
		//		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(ingress, nil)
		//		t.EXPECT().Middlewares(namespace).Return(&mockMiddlewareInterface{
		//			createOrUpdateFunc: func(ctx context.Context, middleware interface{}, opts metav1.UpdateOptions) (interface{}, error) {
		//				return nil, assert.AnError
		//			},
		//			getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (interface{}, error) {
		//				return nil, apierrors.NewNotFound(v1.Resource("middleware"), name)
		//			},
		//		})
		//	},
		//	expErr: true,
		//	errMsg: "failed to create alternative fqdn redirect middleware",
		//},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingressMock := newMockIngressInterface(t)
			traefikMock := newMockTraefikInterface(t)
			tt.setupMock(ingressMock, traefikMock, tt.inAltFQDNList)

			redirector := &IngressRedirector{
				ingressClassName: ingressClassName,
				ingressInterface: ingressMock,
				traefikInterface: traefikMock,
				namespace:        namespace,
			}

			err := redirector.RedirectAlternativeFQDN(context.TODO(), namespace, ingressName, testFqdn, tt.inAltFQDNList, tt.inSetOwner)

			if tt.expErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestIngressRedirector_createRedirectIngress(t *testing.T) {
	tests := []struct {
		name       string
		objectName string
		altFQDNMap map[string][]string
		validateFn func(*testing.T, *v1.Ingress)
	}{
		{
			name:       "create ingress with single certificate and single fqdn",
			objectName: "test-ingress",
			altFQDNMap: map[string][]string{
				"cert1": {"fqdn1.example.com"},
			},
			validateFn: func(t *testing.T, ingress *v1.Ingress) {
				require.NotNil(t, ingress)
				assert.Equal(t, "test-ingress", ingress.Name)
				assert.Equal(t, namespace, ingress.Namespace)
				assert.Equal(t, ingressClassName, *ingress.Spec.IngressClassName)
				assert.Len(t, ingress.Spec.TLS, 1)
				assert.Len(t, ingress.Spec.Rules, 1)
			},
		},
		{
			name:       "create ingress with multiple certificates",
			objectName: "test-ingress",
			altFQDNMap: map[string][]string{
				"cert1": {"fqdn1.example.com", "fqdn2.example.com"},
				"cert2": {"fqdn3.example.com"},
			},
			validateFn: func(t *testing.T, ingress *v1.Ingress) {
				require.NotNil(t, ingress)
				assert.Len(t, ingress.Spec.TLS, 2)
				assert.Len(t, ingress.Spec.Rules, 3)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			redirector := &IngressRedirector{
				ingressClassName: ingressClassName,
			}

			ingress := redirector.createRedirectIngress(namespace, tt.objectName, tt.altFQDNMap)

			tt.validateFn(t, ingress)
		})
	}
}

func TestIngressRedirector_upsertIngress(t *testing.T) {
	tests := []struct {
		name      string
		setupMock func(*mockIngressInterface)
		expErr    bool
		errMsg    string
	}{
		{
			name: "create ingress successfully",
			setupMock: func(m *mockIngressInterface) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(&v1.Ingress{}, nil)
			},
			expErr: false,
		},
		{
			name: "update ingress when already exists",
			setupMock: func(m *mockIngressInterface) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))
				m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(&v1.Ingress{}, nil)
			},
			expErr: false,
		},
		{
			name: "return error on create failure",
			setupMock: func(m *mockIngressInterface) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to create redirect ingress",
		},
		{
			name: "return error on update failure",
			setupMock: func(m *mockIngressInterface) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))
				m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to update redirect ingress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingressMock := newMockIngressInterface(t)
			tt.setupMock(ingressMock)

			redirector := &IngressRedirector{
				ingressInterface: ingressMock,
			}

			ingress := &v1.Ingress{
				ObjectMeta: metav1.ObjectMeta{Name: ingressName},
			}

			_, err := redirector.upsertIngress(context.TODO(), ingress)

			if tt.expErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func Test_createIngressRules(t *testing.T) {
	tests := []struct {
		name     string
		hostList []string
		validate func(*testing.T, []v1.IngressRule)
	}{
		{
			name:     "create rules for single host",
			hostList: []string{"host1.example.com"},
			validate: func(t *testing.T, rules []v1.IngressRule) {
				require.Len(t, rules, 1)
				assert.Equal(t, "host1.example.com", rules[0].Host)
				assert.NotNil(t, rules[0].HTTP)
				assert.Len(t, rules[0].HTTP.Paths, 1)
				assert.Equal(t, "/", rules[0].HTTP.Paths[0].Path)
			},
		},
		{
			name:     "create rules for multiple hosts",
			hostList: []string{"host1.example.com", "host2.example.com", "host3.example.com"},
			validate: func(t *testing.T, rules []v1.IngressRule) {
				require.Len(t, rules, 3)
				for i, rule := range rules {
					assert.Equal(t, fmt.Sprintf("host%d.example.com", i+1), rule.Host)
					assert.NotNil(t, rule.HTTP)
				}
			},
		},
		{
			name:     "create no rules for empty host list",
			hostList: []string{},
			validate: func(t *testing.T, rules []v1.IngressRule) {
				require.Len(t, rules, 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := createIngressRules(tt.hostList)
			tt.validate(t, rules)
		})
	}
}

func Test_groupFQDNsBySecretName(t *testing.T) {
	tests := []struct {
		name        string
		altFQDNList []types.AlternativeFQDN
		expected    map[string][]string
	}{
		{
			name: "group single fqdn",
			altFQDNList: []types.AlternativeFQDN{
				{FQDN: "fqdn1.example.com", CertificateSecretName: "cert1"},
			},
			expected: map[string][]string{
				"cert1": {"fqdn1.example.com"},
			},
		},
		{
			name: "group multiple fqdns with same certificate",
			altFQDNList: []types.AlternativeFQDN{
				{FQDN: "fqdn1.example.com", CertificateSecretName: "cert1"},
				{FQDN: "fqdn2.example.com", CertificateSecretName: "cert1"},
			},
			expected: map[string][]string{
				"cert1": {"fqdn1.example.com", "fqdn2.example.com"},
			},
		},
		{
			name: "group multiple fqdns with different certificates",
			altFQDNList: []types.AlternativeFQDN{
				{FQDN: "fqdn1.example.com", CertificateSecretName: "cert1"},
				{FQDN: "fqdn2.example.com", CertificateSecretName: "cert2"},
				{FQDN: "fqdn3.example.com", CertificateSecretName: "cert1"},
			},
			expected: map[string][]string{
				"cert1": {"fqdn1.example.com", "fqdn3.example.com"},
				"cert2": {"fqdn2.example.com"},
			},
		},
		{
			name:        "return empty map for empty list",
			altFQDNList: []types.AlternativeFQDN{},
			expected:    map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupFQDNsBySecretName(tt.altFQDNList)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_createRedirectMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		primaryFqdn string
		fqdnList    []types.AlternativeFQDN
		setupMock   func(*mockTraefikInterface)
		expErr      bool
		errMsg      string
	}{
		{
			name:        "create middleware successfully",
			primaryFqdn: "primary.example.com",
			fqdnList: []types.AlternativeFQDN{
				{FQDN: "alt1.example.com", CertificateSecretName: "cert1"},
			},
			setupMock: func(m *mockTraefikInterface) {
				mwi := newMockMiddlewareInterface(t)
				middleware := &traefikapi.Middleware{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: traefikapi.MiddlewareSpec{},
				}
				mwi.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(middleware, nil)
				m.EXPECT().Middlewares(namespace).Return(mwi)
			},
			expErr: false,
		},
		{
			name:        "return error when middleware creation fails",
			primaryFqdn: "primary.example.com",
			fqdnList: []types.AlternativeFQDN{
				{FQDN: "alt1.example.com", CertificateSecretName: "cert1"},
			},
			setupMock: func(m *mockTraefikInterface) {
				mwi := newMockMiddlewareInterface(t)
				mwi.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
				m.EXPECT().Middlewares(namespace).Return(mwi)
			},
			expErr: true,
			errMsg: "failed to create or update traefik alternative-fqdn middleware",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			traefikMock := newMockTraefikInterface(t)
			tt.setupMock(traefikMock)

			owner := &v1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: ingressName,
					UID:  "test-uid",
				},
			}

			middlewareManager := expose.NewMiddlewareManager(traefikMock, namespace)
			err := createRedirectMiddleware(context.TODO(), tt.primaryFqdn, tt.fqdnList, owner, middlewareManager)

			if tt.expErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func assertRedirectIngress(t *testing.T, ingress *v1.Ingress, altFQDNList []types.AlternativeFQDN) {
	t.Helper()

	altFQDNMap := groupFQDNsBySecretName(altFQDNList)

	// expected http rule
	prefix := redirectPathType

	expIngressRuleValue := v1.IngressRuleValue{
		HTTP: &v1.HTTPIngressRuleValue{
			Paths: []v1.HTTPIngressPath{
				{
					Path:     redirectIngressPath,
					PathType: &prefix,
					Backend: v1.IngressBackend{
						Service: &v1.IngressServiceBackend{
							Name: redirectEndpointName,
							Port: v1.ServiceBackendPort{
								Number: redirectEndpointPort,
							},
						},
						Resource: nil,
					},
				},
			},
		},
	}

	// name
	require.Equal(t, ingressName, ingress.Name)
	require.Equal(t, namespace, ingress.Namespace)

	// annotations
	annotations := ingress.GetAnnotations()
	require.Len(t, annotations, 1)

	rAnnotation, ok := annotations[traefikMiddlewareAnnotation]
	require.True(t, ok)
	require.Equal(t, "alternative-fqdn@kubernetescrd", rAnnotation)

	// labels
	labels := ingress.GetLabels()
	require.Equal(t, util.K8sCesServiceDiscoveryLabels, labels)

	//tls
	require.Len(t, ingress.Spec.TLS, len(altFQDNMap))

	// ingress class
	require.Equal(t, ingressClassName, *ingress.Spec.IngressClassName)

	for cert, fqdns := range altFQDNMap {
		found := false
		for _, tlsEntry := range ingress.Spec.TLS {
			if tlsEntry.SecretName == cert {
				require.ElementsMatch(t, fqdns, tlsEntry.Hosts)
				found = true
				break
			}
		}

		require.True(t, found, "missing certificate in tls", cert)

		// rules
		for _, fqdn := range fqdns {
			assert.True(t, slices.ContainsFunc(ingress.Spec.Rules, func(rule v1.IngressRule) bool {
				require.Equal(t, expIngressRuleValue, rule.IngressRuleValue)

				return rule.Host == fqdn
			}), "missing rule for fqdn", fqdn)
		}
	}
}

func expectCreateWithAssertion(t *testing.T) func(m *mockIngressInterface, tr *mockTraefikInterface, inAltFQDNList []types.AlternativeFQDN) {
	return func(m *mockIngressInterface, tr *mockTraefikInterface, inAltFQDNList []types.AlternativeFQDN) {
		ingress := &v1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name: ingressName,
				UID:  "test-uid",
			},
		}
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.CreateOptions) {
				assertRedirectIngress(t, ingress, inAltFQDNList)
			}).
			Return(ingress, nil)

		mwi := newMockMiddlewareInterface(t)
		middleware := &traefikapi.Middleware{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: traefikapi.MiddlewareSpec{},
		}
		mwi.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(middleware, nil)
		mwi.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewNotFound(v1.Resource("middleware"), "test"))
		tr.EXPECT().Middlewares(namespace).Return(mwi)
	}
}

func expectUpdateWithAssertion(t *testing.T) func(m *mockIngressInterface, tr *mockTraefikInterface, inAltFQDNList []types.AlternativeFQDN) {
	return func(m *mockIngressInterface, tr *mockTraefikInterface, inAltFQDNList []types.AlternativeFQDN) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))

		ingress := &v1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name: ingressName,
				UID:  "test-uid",
			},
		}
		m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.UpdateOptions) {
				assertRedirectIngress(t, ingress, inAltFQDNList)
			}).
			Return(ingress, nil)

		mwi := newMockMiddlewareInterface(t)
		middleware := &traefikapi.Middleware{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: traefikapi.MiddlewareSpec{},
		}
		mwi.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(middleware, nil)
		mwi.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewNotFound(v1.Resource("middleware"), "test"))
		tr.EXPECT().Middlewares(namespace).Return(mwi)
	}
}

func expectUpdateAlreadyExists(t *testing.T) func(m *mockIngressInterface, tr *mockTraefikInterface, inAltFQDNList []types.AlternativeFQDN) {
	return func(m *mockIngressInterface, tr *mockTraefikInterface, inAltFQDNList []types.AlternativeFQDN) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))

		ingress := &v1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name: ingressName,
				UID:  "test-uid",
			},
		}
		m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.UpdateOptions) {
				assertRedirectIngress(t, ingress, inAltFQDNList)
			}).
			Return(ingress, nil)

		mwi := newMockMiddlewareInterface(t)
		middleware := &traefikapi.Middleware{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Spec: traefikapi.MiddlewareSpec{},
		}
		mwi.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(middleware, nil)

		mwi.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewNotFound(v1.Resource("middleware"), "test"))
		tr.EXPECT().Middlewares(namespace).Return(mwi)
	}
}
