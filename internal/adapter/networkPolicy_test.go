package adapter

import (
	"context"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	testNamespace = "ns"
	testCIDR      = "10.0.0.0/8"
)

var testLabelSelector = metav1.LabelSelector{MatchLabels: map[string]string{"app.kubernetes.io/name": "traefik"}}

func newScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, networkingv1.AddToScheme(s))
	return s
}

func newFakeClient(t *testing.T, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objs...).Build()
}

func newFakeClientWithInterceptor(t *testing.T, ic interceptor.Funcs, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objs...).WithInterceptorFuncs(ic).Build()
}

func okOwner(_ client.Object) error  { return nil }
func errOwner(_ client.Object) error { return assert.AnError }

func fixedNetworkPolicy(t *testing.T, c client.Client) NetworkPolicy {
	t.Helper()
	return NetworkPolicy{Client: c, LabelSelector: testLabelSelector, AllowedCIDR: testCIDR}
}

func TestNetworkPolicy_GetOwnableTypes(t *testing.T) {
	n := NetworkPolicy{}
	got := n.GetOwnableTypes()
	require.Len(t, got, 1)
	assert.IsType(t, &networkingv1.NetworkPolicy{}, got[0])
}

func Test_NetworkPolicy_createNetworkPolicyName(t *testing.T) {
	tests := []struct {
		name           string
		expositionName string
		want           string
	}{
		{name: "named exposition", expositionName: "ldap", want: "ldap-exposed-ports"},
		{name: "empty name yields formatted suffix only", expositionName: "", want: "-exposed-ports"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NetworkPolicy{}
			assert.Equal(t, tt.want, n.createNetworkPolicyName(tt.expositionName))
		})
	}
}

func Test_NetworkPolicy_mapExposedPorts(t *testing.T) {
	tests := []struct {
		name string
		in   []types.ExposedPort
		want []networkingv1.NetworkPolicyPort
	}{
		{
			name: "empty input yields empty slice",
			in:   nil,
			want: []networkingv1.NetworkPolicyPort{},
		},
		{
			name: "single TCP port",
			in:   []types.ExposedPort{{Protocol: corev1.ProtocolTCP, RequestedExternalPort: 80}},
			want: []networkingv1.NetworkPolicyPort{
				{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(80))},
			},
		},
		{
			name: "single UDP port",
			in:   []types.ExposedPort{{Protocol: corev1.ProtocolUDP, RequestedExternalPort: 53}},
			want: []networkingv1.NetworkPolicyPort{
				{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(53))},
			},
		},
		{
			name: "mixed TCP+UDP preserves order and protocols",
			in: []types.ExposedPort{
				{Protocol: corev1.ProtocolTCP, RequestedExternalPort: 80},
				{Protocol: corev1.ProtocolUDP, RequestedExternalPort: 53},
			},
			want: []networkingv1.NetworkPolicyPort{
				{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(80))},
				{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(53))},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NetworkPolicy{}
			assert.Equal(t, tt.want, n.mapExposedPorts(tt.in))
		})
	}
}

func Test_NetworkPolicy_createNetworkPolicy(t *testing.T) {
	tcpPorts := []types.ExposedPort{{Protocol: corev1.ProtocolTCP, RequestedExternalPort: 80}}
	udpPorts := []types.ExposedPort{{Protocol: corev1.ProtocolUDP, RequestedExternalPort: 53}}

	tests := []struct {
		name     string
		tcp      []types.ExposedPort
		udp      []types.ExposedPort
		setOwner types.SetOwnerFunc
		wantErr  assert.ErrorAssertionFunc
		check    func(t *testing.T, np *networkingv1.NetworkPolicy)
	}{
		{
			name:     "no ports yields empty Ports slice",
			tcp:      nil,
			udp:      nil,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			check: func(t *testing.T, np *networkingv1.NetworkPolicy) {
				require.Len(t, np.Spec.Ingress, 1)
				assert.Empty(t, np.Spec.Ingress[0].Ports)
			},
		},
		{
			name:     "TCP and UDP ports populate Ingress.Ports in order",
			tcp:      tcpPorts,
			udp:      udpPorts,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			check: func(t *testing.T, np *networkingv1.NetworkPolicy) {
				require.Len(t, np.Spec.Ingress, 1)
				got := np.Spec.Ingress[0].Ports
				require.Len(t, got, 2)
				assert.Equal(t, corev1.ProtocolTCP, *got[0].Protocol)
				assert.Equal(t, intstr.FromInt32(80), *got[0].Port)
				assert.Equal(t, corev1.ProtocolUDP, *got[1].Protocol)
				assert.Equal(t, intstr.FromInt32(53), *got[1].Port)
			},
		},
		{
			name:     "Spec.PodSelector mirrors configured labelSelector",
			tcp:      tcpPorts,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			check: func(t *testing.T, np *networkingv1.NetworkPolicy) {
				assert.Equal(t, testLabelSelector, np.Spec.PodSelector)
			},
		},
		{
			name:     "Ingress.From.IPBlock.CIDR mirrors allowedCIDR",
			tcp:      tcpPorts,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			check: func(t *testing.T, np *networkingv1.NetworkPolicy) {
				require.Len(t, np.Spec.Ingress, 1)
				require.Len(t, np.Spec.Ingress[0].From, 1)
				require.NotNil(t, np.Spec.Ingress[0].From[0].IPBlock)
				assert.Equal(t, testCIDR, np.Spec.Ingress[0].From[0].IPBlock.CIDR)
			},
		},
		{
			name:     "ObjectMeta carries discovery labels and formatted name",
			tcp:      tcpPorts,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			check: func(t *testing.T, np *networkingv1.NetworkPolicy) {
				assert.Equal(t, "ldap-exposed-ports", np.Name)
				assert.Equal(t, testNamespace, np.Namespace)
				assert.Equal(t, util.K8sCesServiceDiscoveryLabels, np.Labels)
				assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, np.Spec.PolicyTypes)
			},
		},
		{
			name:     "SetOwner failure surfaces wrapped error",
			tcp:      tcpPorts,
			setOwner: errOwner,
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to set owner for network policy", i...)
			},
			check: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := fixedNetworkPolicy(t, nil)
			exposition := types.Exposition{
				Name:      "ldap",
				Namespace: testNamespace,
				TcpRoutes: tt.tcp,
				UdpRoutes: tt.udp,
				SetOwner:  tt.setOwner,
			}
			got, err := n.generateNetworkPolicy(exposition)
			if !tt.wantErr(t, err) {
				return
			}
			if tt.check != nil {
				require.NotNil(t, got)
				tt.check(t, got)
			}
		})
	}
}

func TestNetworkPolicy_ProcessExposition(t *testing.T) {
	tcpPorts := []types.ExposedPort{{Protocol: corev1.ProtocolTCP, RequestedExternalPort: 80}}
	policyKey := client.ObjectKey{Namespace: testNamespace, Name: "ldap-exposed-ports"}

	tests := []struct {
		name          string
		clientFn      func(t *testing.T) client.Client
		tcp           []types.ExposedPort
		udp           []types.ExposedPort
		setOwner      types.SetOwnerFunc
		wantErr       assert.ErrorAssertionFunc
		wantCondition *conditionCall
		postCheck     func(t *testing.T, c client.Client)
	}{
		{
			name:     "no ports, no existing policy => no-op",
			clientFn: func(t *testing.T) client.Client { return newFakeClient(t) },
			tcp:      nil,
			udp:      nil,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				err := c.Get(t.Context(), policyKey, &networkingv1.NetworkPolicy{})
				assert.True(t, apierrors.IsNotFound(err), "expected NotFound, got %v", err)
			},
		},
		{
			name: "no ports, existing policy => policy deleted",
			clientFn: func(t *testing.T) client.Client {
				return newFakeClient(t, &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "ldap-exposed-ports", Namespace: testNamespace},
				})
			},
			tcp:      nil,
			udp:      nil,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			postCheck: func(t *testing.T, c client.Client) {
				err := c.Get(t.Context(), policyKey, &networkingv1.NetworkPolicy{})
				assert.True(t, apierrors.IsNotFound(err), "expected NotFound after delete, got %v", err)
			},
		},
		{
			name: "no ports, Delete returns non-NotFound => wrapped error",
			clientFn: func(t *testing.T) client.Client {
				return newFakeClientWithInterceptor(t, interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return assert.AnError
					},
				})
			},
			tcp:      nil,
			udp:      nil,
			setOwner: okOwner,
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to delete network policy", i...)
			},
			wantCondition: &conditionCall{
				conditionType: NetworkPolicyConditionType,
				status:        false,
				reason:        networkPolicyDeletionFailedConditionReason,
				message:       "failed to delete network policy \"ldap-exposed-ports\": assert.AnError general error for testing",
			},
		},
		{
			name:     "ports present, SetOwner errors => wrapped error",
			clientFn: func(t *testing.T) client.Client { return newFakeClient(t) },
			tcp:      tcpPorts,
			setOwner: errOwner,
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to generate network policy", i...)
			},
			wantCondition: &conditionCall{
				conditionType: NetworkPolicyConditionType,
				status:        false,
				reason:        networkPolicyGenerationFailedConditionReason,
				message:       "failed to generate network policy: failed to set owner for network policy: assert.AnError general error for testing",
			},
		},
		{
			name:     "ports present, no existing policy => policy created with desired spec",
			clientFn: func(t *testing.T) client.Client { return newFakeClient(t) },
			tcp:      tcpPorts,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			wantCondition: &conditionCall{
				conditionType: NetworkPolicyConditionType,
				status:        true,
				reason:        networkPolicyCreatedConditionReason,
				message:       networkPolicyCreatedConditionMessage,
			},
			postCheck: func(t *testing.T, c client.Client) {
				got := &networkingv1.NetworkPolicy{}
				require.NoError(t, c.Get(t.Context(), policyKey, got))
				assert.Equal(t, util.K8sCesServiceDiscoveryLabels, got.Labels)
				assert.Equal(t, testLabelSelector, got.Spec.PodSelector)
				assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, got.Spec.PolicyTypes)
				require.Len(t, got.Spec.Ingress, 1)
				require.Len(t, got.Spec.Ingress[0].Ports, 1)
				assert.Equal(t, corev1.ProtocolTCP, *got.Spec.Ingress[0].Ports[0].Protocol)
				assert.Equal(t, intstr.FromInt32(80), *got.Spec.Ingress[0].Ports[0].Port)
				require.Len(t, got.Spec.Ingress[0].From, 1)
				assert.Equal(t, testCIDR, got.Spec.Ingress[0].From[0].IPBlock.CIDR)
			},
		},
		{
			name: "ports present, existing stale policy => spec updated",
			clientFn: func(t *testing.T) client.Client {
				return newFakeClient(t, &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{Name: "ldap-exposed-ports", Namespace: testNamespace},
					Spec:       networkingv1.NetworkPolicySpec{},
				})
			},
			tcp:      tcpPorts,
			setOwner: okOwner,
			wantErr:  assert.NoError,
			wantCondition: &conditionCall{
				conditionType: NetworkPolicyConditionType,
				status:        true,
				reason:        networkPolicyCreatedConditionReason,
				message:       networkPolicyCreatedConditionMessage,
			},
			postCheck: func(t *testing.T, c client.Client) {
				got := &networkingv1.NetworkPolicy{}
				require.NoError(t, c.Get(t.Context(), policyKey, got))
				require.Len(t, got.Spec.Ingress, 1)
				require.Len(t, got.Spec.Ingress[0].Ports, 1)
				assert.Equal(t, intstr.FromInt32(80), *got.Spec.Ingress[0].Ports[0].Port)
			},
		},
		{
			name: "ports present, CreateOrUpdate Get errors => wrapped error",
			clientFn: func(t *testing.T) client.Client {
				return newFakeClientWithInterceptor(t, interceptor.Funcs{
					Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
						return assert.AnError
					},
				})
			},
			tcp:      tcpPorts,
			setOwner: okOwner,
			wantErr: func(t assert.TestingT, err error, i ...any) bool {
				return assert.ErrorIs(t, err, assert.AnError, i...) &&
					assert.ErrorContains(t, err, "failed to create or update network policy", i...)
			},
			wantCondition: &conditionCall{
				conditionType: NetworkPolicyConditionType,
				status:        false,
				reason:        networkPolicyCreateOrUpdateFailedConditionReason,
				message:       "failed to create or update network policy \"ldap-exposed-ports\": assert.AnError general error for testing",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.clientFn(t)
			n := fixedNetworkPolicy(t, c)
			gotCondition := conditionCall{}
			conditionSet := false
			exposition := types.Exposition{
				Name:      "ldap",
				Namespace: testNamespace,
				TcpRoutes: tt.tcp,
				UdpRoutes: tt.udp,
				SetOwner:  tt.setOwner,
				SetCondition: func(_ context.Context, conditionType string, conditionStatus bool, reason string, msg string) error {
					conditionSet = true
					gotCondition = conditionCall{
						conditionType: conditionType,
						status:        conditionStatus,
						reason:        reason,
						message:       msg,
					}
					return nil
				},
			}
			err := n.ProcessExposition(t.Context(), exposition)
			if !tt.wantErr(t, err) {
				return
			}
			if tt.wantCondition == nil {
				assert.False(t, conditionSet)
			} else {
				require.True(t, conditionSet)
				assert.Equal(t, *tt.wantCondition, gotCondition)
			}
			if tt.postCheck != nil {
				tt.postCheck(t, c)
			}
		})
	}
}
