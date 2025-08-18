package nginx

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
		name         string
		inAltFQDNMap map[string][]string
		inSetOwner   func(targetObject metav1.Object) error
		setupMock    func(*mockIngressInterface, map[string][]string)
		expErr       bool
		errMsg       string
	}{
		{
			name: "create new redirect ingress with single alternative testFqdn and no certificate",
			inAltFQDNMap: map[string][]string{
				defaultCertificateName: {"test.testFqdn"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name: "create new redirect ingress with multiple alternative fqdns and single certificate",
			inAltFQDNMap: map[string][]string{
				defaultCertificateName: {"test.testFqdn", "test2.testFqdn", "test3.testFqdn"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name: "create new redirect ingress with multiple alternative fqdns with different certificates",
			inAltFQDNMap: map[string][]string{
				defaultCertificateName: {"test.testFqdn"},
				"testCertificate2":     {"test2.testFqdn"},
				"testCertificate3":     {"test3.testFqdn"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name: "update redirect ingress when already exists",
			inAltFQDNMap: map[string][]string{
				defaultCertificateName: {"test.testFqdn"},
				"testCertificate2":     {"test2.testFqdn"},
				"testCertificate3":     {"test3.testFqdn"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectUpdateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name:         "delete redirect ingress when alternative testFqdn is empty and redirect ingress exists",
			inAltFQDNMap: map[string][]string{},
			inSetOwner:   func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, m2 map[string][]string) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(nil)
			},
			expErr: false,
			errMsg: "",
		},
		{
			name:         "return no error when alternative testFqdn is empty and redirect ingress does not exist",
			inAltFQDNMap: map[string][]string{},
			inSetOwner:   func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, m2 map[string][]string) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(apierrors.NewNotFound(v1.Resource("ingress"), ingressName))
			},
			expErr: false,
			errMsg: "",
		},
		{
			name: "return error when redirect ingress cannot be created",
			inAltFQDNMap: map[string][]string{
				defaultCertificateName: {"test.testFqdn"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, m2 map[string][]string) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to create redirect ingress",
		},
		{
			name: "return error when redirect ingress cannot be updated",
			inAltFQDNMap: map[string][]string{
				defaultCertificateName: {"test.testFqdn"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, m2 map[string][]string) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))
				m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to update redirect ingress",
		},
		{
			name:         "return error when redirect ingress cannot be deleted",
			inAltFQDNMap: map[string][]string{},
			inSetOwner:   func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, m2 map[string][]string) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(assert.AnError)
			},
			expErr: true,
			errMsg: "failed to delete redirect ingress",
		},
		{
			name: "return error when owner cannot be set",
			inAltFQDNMap: map[string][]string{
				defaultCertificateName: {"test.testFqdn"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return assert.AnError },
			setupMock:  func(m *mockIngressInterface, m2 map[string][]string) {},
			expErr:     true,
			errMsg:     "failed to set owner for redirect ingress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingressMock := newMockIngressInterface(t)
			tt.setupMock(ingressMock, tt.inAltFQDNMap)

			redirector := &IngressRedirector{
				ingressClassName: ingressClassName,
				ingressInterface: ingressMock,
			}

			err := redirector.RedirectAlternativeFQDN(context.TODO(), namespace, ingressName, testFqdn, tt.inAltFQDNMap, tt.inSetOwner)

			if tt.expErr {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tt.errMsg)

				return
			}

			assert.NoError(t, err)
		})
	}
}

func assertRedirectIngress(t *testing.T, ingress *v1.Ingress, altFQDNMap map[string][]string) {
	t.Helper()

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

	rAnnotation, ok := annotations[redirectAnnotation]
	require.True(t, ok)
	require.Equal(t, fmt.Sprintf("return 308 https://%s$request_uri;", testFqdn), rAnnotation)

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

func expectCreateWithAssertion(t *testing.T) func(m *mockIngressInterface, inAltFQDNMap map[string][]string) {
	return func(m *mockIngressInterface, inAltFQDNMap map[string][]string) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.CreateOptions) {
				assertRedirectIngress(t, ingress, inAltFQDNMap)
			}).
			Return(&v1.Ingress{}, nil)
	}
}

func expectUpdateWithAssertion(t *testing.T) func(m *mockIngressInterface, inAltFQDNMap map[string][]string) {
	return func(m *mockIngressInterface, inAltFQDNMap map[string][]string) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))

		m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.UpdateOptions) {
				assertRedirectIngress(t, ingress, inAltFQDNMap)
			}).
			Return(&v1.Ingress{}, nil)
	}
}
