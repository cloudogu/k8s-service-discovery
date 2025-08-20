package nginx

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
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
		name          string
		inAltFQDNList []types.AlternativeFQDN
		inSetOwner    func(targetObject metav1.Object) error
		setupMock     func(*mockIngressInterface, []types.AlternativeFQDN)
		expErr        bool
		errMsg        string
	}{
		{
			name: "create new redirect ingress with single alternative testFqdn and no certificate",
			inAltFQDNList: []types.AlternativeFQDN{
				{"test.testFqdn", defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name: "create new redirect ingress with multiple alternative fqdns and single certificate",
			inAltFQDNList: []types.AlternativeFQDN{
				{"test.testFqdn", defaultCertificateName},
				{"test2.testFqdn", defaultCertificateName},
				{"test3.testFqdn", defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name: "create new redirect ingress with multiple alternative fqdns with different certificates",
			inAltFQDNList: []types.AlternativeFQDN{
				{"test.testFqdn", defaultCertificateName},
				{"test2.testFqdn", "testCertificate2"},
				{"test3.testFqdn", "testCertificate3"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectCreateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name: "update redirect ingress when already exists",
			inAltFQDNList: []types.AlternativeFQDN{
				{"test.testFqdn", defaultCertificateName},
				{"test2.testFqdn", "testCertificate2"},
				{"test3.testFqdn", "testCertificate3"},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock:  expectUpdateWithAssertion(t),
			expErr:     false,
			errMsg:     "",
		},
		{
			name:          "delete redirect ingress when alternative testFqdn is empty and redirect ingress exists",
			inAltFQDNList: []types.AlternativeFQDN{},
			inSetOwner:    func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(nil)
			},
			expErr: false,
			errMsg: "",
		},
		{
			name:          "return no error when alternative testFqdn is empty and redirect ingress does not exist",
			inAltFQDNList: []types.AlternativeFQDN{},
			inSetOwner:    func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(apierrors.NewNotFound(v1.Resource("ingress"), ingressName))
			},
			expErr: false,
			errMsg: "",
		},
		{
			name: "return error when redirect ingress cannot be created",
			inAltFQDNList: []types.AlternativeFQDN{
				{"test.testFqdn", defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to create redirect ingress",
		},
		{
			name: "return error when redirect ingress cannot be updated",
			inAltFQDNList: []types.AlternativeFQDN{
				{"test.testFqdn", defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))
				m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil, assert.AnError)
			},
			expErr: true,
			errMsg: "failed to update redirect ingress",
		},
		{
			name:          "return error when redirect ingress cannot be deleted",
			inAltFQDNList: []types.AlternativeFQDN{},
			inSetOwner:    func(targetObject metav1.Object) error { return nil },
			setupMock: func(m *mockIngressInterface, l []types.AlternativeFQDN) {
				m.EXPECT().Delete(mock.Anything, ingressName, mock.Anything).Return(assert.AnError)
			},
			expErr: true,
			errMsg: "failed to delete redirect ingress",
		},
		{
			name: "return error when owner cannot be set",
			inAltFQDNList: []types.AlternativeFQDN{
				{"test.testFqdn", defaultCertificateName},
			},
			inSetOwner: func(targetObject metav1.Object) error { return assert.AnError },
			setupMock:  func(m *mockIngressInterface, l []types.AlternativeFQDN) {},
			expErr:     true,
			errMsg:     "failed to set owner for redirect ingress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingressMock := newMockIngressInterface(t)
			tt.setupMock(ingressMock, tt.inAltFQDNList)

			redirector := &IngressRedirector{
				ingressClassName: ingressClassName,
				ingressInterface: ingressMock,
			}

			err := redirector.RedirectAlternativeFQDN(context.TODO(), namespace, ingressName, testFqdn, tt.inAltFQDNList, tt.inSetOwner)

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

func expectCreateWithAssertion(t *testing.T) func(m *mockIngressInterface, inAltFQDNList []types.AlternativeFQDN) {
	return func(m *mockIngressInterface, inAltFQDNList []types.AlternativeFQDN) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.CreateOptions) {
				assertRedirectIngress(t, ingress, inAltFQDNList)
			}).
			Return(&v1.Ingress{}, nil)
	}
}

func expectUpdateWithAssertion(t *testing.T) func(m *mockIngressInterface, inAltFQDNList []types.AlternativeFQDN) {
	return func(m *mockIngressInterface, inAltFQDNList []types.AlternativeFQDN) {
		m.EXPECT().Create(mock.Anything, mock.Anything, mock.Anything).Return(nil, apierrors.NewAlreadyExists(v1.Resource("ingress"), ingressName))

		m.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).
			Run(func(ctx context.Context, ingress *v1.Ingress, opts metav1.UpdateOptions) {
				assertRedirectIngress(t, ingress, inAltFQDNList)
			}).
			Return(&v1.Ingress{}, nil)
	}
}
