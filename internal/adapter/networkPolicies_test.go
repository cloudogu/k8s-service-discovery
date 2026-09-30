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
	require.NoError(t, corev1.AddToScheme(s))
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

func fixedNetworkPolicy(t *testing.T, c client.Client, networkPoliciesEnabled *bool) NetworkPolicies {
	t.Helper()
	var netpolsEnabled bool
	if networkPoliciesEnabled == nil {
		netpolsEnabled = true
	}
	return NetworkPolicies{Client: c, CesGatewayLabelSelector: testLabelSelector, AllowedExternalCIDR: testCIDR, NetworkPoliciesEnabled: netpolsEnabled}
}

func TestNetworkPolicy_GetOwnableTypes(t *testing.T) {
	n := NetworkPolicies{}
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
			n := NetworkPolicies{}
			assert.Equal(t, tt.want, n.createNameForExternalPorts(tt.expositionName))
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
			n := NetworkPolicies{}
			assert.Equal(t, tt.want, n.mapExternalPorts(tt.in))
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
			n := fixedNetworkPolicy(t, nil, nil)
			exposition := types.Exposition{
				Name:      "ldap",
				Namespace: testNamespace,
				TcpRoutes: tt.tcp,
				UdpRoutes: tt.udp,
				SetOwner:  tt.setOwner,
			}
			got, err := n.generateForExternalPorts(exposition)
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

func TestNetworkPolicy_processExternalPortsNetworkPolicy(t *testing.T) {
	tcpPorts := []types.ExposedPort{{Protocol: corev1.ProtocolTCP, RequestedExternalPort: 80}}
	policyKey := client.ObjectKey{Namespace: testNamespace, Name: "ldap-exposed-ports"}

	tests := []struct {
		name                   string
		clientFn               func(t *testing.T) client.Client
		networkPoliciesEnabled *bool
		tcp                    []types.ExposedPort
		udp                    []types.ExposedPort
		setOwner               types.SetOwnerFunc
		wantErr                assert.ErrorAssertionFunc
		wantCondition          *conditionCall
		postCheck              func(t *testing.T, c client.Client)
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
					Name: "ldap-exposed-ports", Namespace: testNamespace,
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
			name:                   "netpols disabled, existing policy => policy deleted",
			networkPoliciesEnabled: new(false),
			clientFn: func(t *testing.T) client.Client {
				return newFakeClient(t, &networkingv1.NetworkPolicy{
					Name: "ldap-exposed-ports", Namespace: testNamespace,
				})
			},
			tcp:      tcpPorts,
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
				conditionType: NetworkPoliciesConditionType,
				status:        false,
				reason:        networkPoliciesDeletionFailedConditionReason,
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
				conditionType: NetworkPoliciesConditionType,
				status:        false,
				reason:        networkPoliciesGenerationFailedConditionReason,
				message:       "failed to generate network policy \"ldap-exposed-ports\": failed to set owner for network policy: assert.AnError general error for testing",
			},
		},
		{
			name:          "ports present, no existing policy => policy created with desired spec",
			clientFn:      func(t *testing.T) client.Client { return newFakeClient(t) },
			tcp:           tcpPorts,
			setOwner:      okOwner,
			wantErr:       assert.NoError,
			wantCondition: nil,
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
					Name: "ldap-exposed-ports", Namespace: testNamespace,
					Spec: networkingv1.NetworkPolicySpec{},
				})
			},
			tcp:           tcpPorts,
			setOwner:      okOwner,
			wantErr:       assert.NoError,
			wantCondition: nil,
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
				conditionType: NetworkPoliciesConditionType,
				status:        false,
				reason:        networkPoliciesCreateOrUpdateFailedConditionReason,
				message:       "failed to create or update network policy \"ldap-exposed-ports\": assert.AnError general error for testing",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.clientFn(t)
			n := fixedNetworkPolicy(t, c, tt.networkPoliciesEnabled)
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
			err, _ := n.processExternalPortsNetworkPolicy(t.Context(), exposition)
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

func networkPolicyService(name string, protocol corev1.Protocol, servicePort int32, targetPort intstr.IntOrString) *corev1.Service {
	return &corev1.Service{
		Name: name, Namespace: testNamespace,
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": name},
			Ports:    []corev1.ServicePort{{Protocol: protocol, Port: servicePort, TargetPort: targetPort}},
		},
	}
}

func assertNetworkPolicyRoute(t *testing.T, c client.Client, name, service string, protocol corev1.Protocol, targetPort intstr.IntOrString) {
	t.Helper()
	policy := &networkingv1.NetworkPolicy{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, policy))
	assert.Equal(t, map[string]string{"app": service}, policy.Spec.PodSelector.MatchLabels)
	assert.Equal(t, "ldap", policy.Labels[ownedByLabelKey])
	for key, value := range util.K8sCesServiceDiscoveryLabels {
		assert.Equal(t, value, policy.Labels[key])
	}
	assert.Equal(t, []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}, policy.Spec.PolicyTypes)
	require.Len(t, policy.Spec.Ingress, 1)
	require.Len(t, policy.Spec.Ingress[0].Ports, 1)
	assert.Equal(t, protocol, *policy.Spec.Ingress[0].Ports[0].Protocol)
	assert.Equal(t, targetPort, *policy.Spec.Ingress[0].Ports[0].Port)
	require.Len(t, policy.Spec.Ingress[0].From, 1)
	assert.Equal(t, &testLabelSelector, policy.Spec.Ingress[0].From[0].PodSelector)
}

func TestNetworkPolicy_ProcessExposition_ReconcilesRoutes(t *testing.T) {
	stale := &networkingv1.NetworkPolicy{
		Name: "ldap-old-http-route", Namespace: testNamespace, Labels: map[string]string{ownedByLabelKey: "ldap"}}
	foreign := &networkingv1.NetworkPolicy{
		Name: "other-http-route", Namespace: testNamespace, Labels: map[string]string{ownedByLabelKey: "other"}}
	oldHTTP := &networkingv1.NetworkPolicy{
		Name: "ldap-ui-http-route", Namespace: testNamespace, Labels: map[string]string{ownedByLabelKey: "ldap"}}
	c := newFakeClient(t,
		networkPolicyService("ui", corev1.ProtocolTCP, 8080, intstr.FromString("http")),
		networkPolicyService("socket", corev1.ProtocolTCP, 8090, intstr.FromInt32(9090)),
		networkPolicyService("dns", corev1.ProtocolUDP, 8053, intstr.FromInt32(5353)),
		stale, foreign, oldHTTP,
	)
	n := fixedNetworkPolicy(t, c, nil)
	var gotCondition conditionCall
	ownerCalls := 0
	exposition := types.Exposition{
		Name: "ldap", Namespace: testNamespace,
		HttpRoutes: []types.HttpRoute{{Name: "ui", Service: "ui", Port: int32(8080)}},
		TcpRoutes:  types.ExposedPorts{{Name: "socket", ServiceName: "socket", ServicePort: int32(8090), Protocol: corev1.ProtocolTCP, RequestedExternalPort: 9000}},
		UdpRoutes:  types.ExposedPorts{{Name: "dns", ServiceName: "dns", ServicePort: int32(8053), Protocol: corev1.ProtocolUDP, RequestedExternalPort: 53}},
		SetOwner: func(obj client.Object) error {
			ownerCalls++
			obj.SetAnnotations(map[string]string{"owner-tested": "yes"})
			return nil
		},
		SetCondition: func(_ context.Context, kind string, status bool, reason, message string) error {
			gotCondition = conditionCall{kind, status, reason, message}
			return nil
		},
	}

	require.NoError(t, n.ProcessExposition(t.Context(), exposition))
	assert.Equal(t, conditionCall{NetworkPoliciesConditionType, true, networkPoliciesCreatedConditionReason, networkPoliciesCreatedConditionMessage}, gotCondition)
	assert.Equal(t, 4, ownerCalls)
	assertNetworkPolicyRoute(t, c, "ldap-ui-http-route", "ui", corev1.ProtocolTCP, intstr.FromString("http"))
	assertNetworkPolicyRoute(t, c, "ldap-socket-tcp-route", "socket", corev1.ProtocolTCP, intstr.FromInt32(9090))
	assertNetworkPolicyRoute(t, c, "ldap-dns-udp-route", "dns", corev1.ProtocolUDP, intstr.FromInt32(5353))
	external := &networkingv1.NetworkPolicy{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: "ldap-exposed-ports"}, external))
	require.Len(t, external.Spec.Ingress, 1)
	assert.Equal(t, []networkingv1.NetworkPolicyPort{
		{Protocol: new(corev1.ProtocolTCP), Port: new(intstr.FromInt32(9000))},
		{Protocol: new(corev1.ProtocolUDP), Port: new(intstr.FromInt32(53))},
	}, external.Spec.Ingress[0].Ports)
	for _, name := range []string{"ldap-ui-http-route", "ldap-socket-tcp-route", "ldap-dns-udp-route", "ldap-exposed-ports"} {
		policy := &networkingv1.NetworkPolicy{}
		require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, policy))
		assert.Equal(t, "yes", policy.Annotations["owner-tested"])
	}
	assert.True(t, apierrors.IsNotFound(c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: stale.Name}, &networkingv1.NetworkPolicy{})))
	require.NoError(t, c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: foreign.Name}, &networkingv1.NetworkPolicy{}))
}

func TestNetworkPolicy_ProcessExposition_DisabledRemovesPolicies(t *testing.T) {
	c := newFakeClient(t,
		&networkingv1.NetworkPolicy{Name: "ldap-exposed-ports", Namespace: testNamespace},
		&networkingv1.NetworkPolicy{Name: "ldap-old-http-route", Namespace: testNamespace, Labels: map[string]string{ownedByLabelKey: "ldap"}},
	)
	n := fixedNetworkPolicy(t, c, new(false))
	exposition := types.Exposition{
		Name: "ldap", Namespace: testNamespace,
		TcpRoutes: types.ExposedPorts{{Name: "socket", Protocol: corev1.ProtocolTCP}},
		SetCondition: func(_ context.Context, kind string, status bool, reason, message string) error {
			assert.Equal(t, conditionCall{NetworkPoliciesConditionType, true, networkPoliciesCreatedConditionReason, networkPoliciesCreatedConditionMessage}, conditionCall{kind, status, reason, message})
			return nil
		},
	}
	require.NoError(t, n.ProcessExposition(t.Context(), exposition))
	for _, name := range []string{"ldap-exposed-ports", "ldap-old-http-route"} {
		assert.True(t, apierrors.IsNotFound(c.Get(t.Context(), client.ObjectKey{Namespace: testNamespace, Name: name}, &networkingv1.NetworkPolicy{})))
	}
}

func TestNetworkPolicy_ProcessExposition_InternalRouteErrors(t *testing.T) {
	tests := []struct {
		name       string
		makeClient func(*testing.T) client.Client
		setOwner   types.SetOwnerFunc
		want       string
	}{
		{
			name:       "missing HTTP service",
			makeClient: func(t *testing.T) client.Client { return newFakeClient(t) },
			setOwner:   okOwner,
			want:       "generate for http route \"ui\"",
		},
		{
			name: "HTTP owner failure",
			makeClient: func(t *testing.T) client.Client {
				return newFakeClient(t, networkPolicyService("ui", corev1.ProtocolTCP, 8000, intstr.FromInt32(8080)))
			},
			setOwner: errOwner,
			want:     "generate network policies for http routes",
		},
		{
			name: "list failure",
			makeClient: func(t *testing.T) client.Client {
				return newFakeClientWithInterceptor(t, interceptor.Funcs{
					List: func(_ context.Context, _ client.WithWatch, _ client.ObjectList, _ ...client.ListOption) error {
						return assert.AnError
					},
				}, networkPolicyService("ui", corev1.ProtocolTCP, 8000, intstr.FromInt32(8080)))
			},
			setOwner: okOwner,
			want:     "failed to list existing network policies",
		},
		{
			name: "create failure",
			makeClient: func(t *testing.T) client.Client {
				return newFakeClientWithInterceptor(t, interceptor.Funcs{
					Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
						return assert.AnError
					},
				}, networkPolicyService("ui", corev1.ProtocolTCP, 8000, intstr.FromInt32(8080)))
			},
			setOwner: okOwner,
			want:     "failed to create or update network policy \"ldap-ui-http-route\"",
		},
		{
			name: "delete stale failure",
			makeClient: func(t *testing.T) client.Client {
				return newFakeClientWithInterceptor(t, interceptor.Funcs{
					Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
						return assert.AnError
					},
				}, networkPolicyService("ui", corev1.ProtocolTCP, 8000, intstr.FromInt32(8080)),
					networkPolicyService("socket", corev1.ProtocolTCP, 8090, intstr.FromInt32(9090)),
					&networkingv1.NetworkPolicy{Name: "ldap-old-http-route", Namespace: testNamespace, Labels: map[string]string{ownedByLabelKey: "ldap"}})
			},
			setOwner: okOwner,
			want:     "failed to delete outdated network policy \"ldap-old-http-route\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.makeClient(t)
			n := fixedNetworkPolicy(t, c, nil)
			exposition := types.Exposition{
				Name: "ldap", Namespace: testNamespace,
				HttpRoutes: []types.HttpRoute{{Name: "ui", Service: "ui", Port: int32(8000)}},
				SetOwner:   tt.setOwner,
				SetCondition: func(_ context.Context, _ string, _ bool, reason, _ string) error {
					if tt.name == "missing HTTP service" || tt.name == "HTTP owner failure" {
						assert.Equal(t, networkPoliciesGenerationFailedConditionReason, reason)
					}
					return nil
				},
			}
			if tt.name == "delete stale failure" {
				exposition.TcpRoutes = types.ExposedPorts{{Name: "socket", ServiceName: "socket", ServicePort: int32(8090), Protocol: corev1.ProtocolTCP, RequestedExternalPort: 9000}}
			}
			err := n.ProcessExposition(t.Context(), exposition)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.want)
		})
	}
}

func TestNetworkPolicy_ProcessExposition_ExposedRouteErrors(t *testing.T) {
	tests := []struct {
		name     string
		protocol corev1.Protocol
		service  *corev1.Service
		ownerErr bool
		want     string
	}{
		{name: "TCP service missing", protocol: corev1.ProtocolTCP, want: `generate for TCP route "socket"`},
		{name: "UDP service missing", protocol: corev1.ProtocolUDP, want: `generate for UDP route "socket"`},
		{name: "TCP owner failure", protocol: corev1.ProtocolTCP, service: networkPolicyService("socket", corev1.ProtocolTCP, 8090, intstr.FromInt32(9000)), ownerErr: true, want: "generate network policies for TCP routes"},
		{name: "UDP owner failure", protocol: corev1.ProtocolUDP, service: networkPolicyService("socket", corev1.ProtocolUDP, 8090, intstr.FromInt32(9000)), ownerErr: true, want: "generate network policies for UDP routes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object
			if tt.service != nil {
				objects = append(objects, tt.service)
			}
			c := newFakeClient(t, objects...)
			n := fixedNetworkPolicy(t, c, nil)
			route := types.ExposedPort{Name: "socket", ServiceName: "socket", ServicePort: int32(8090), Protocol: tt.protocol, RequestedExternalPort: 9000}
			exposition := types.Exposition{
				Name: "ldap", Namespace: testNamespace,
				SetOwner: func(obj client.Object) error {
					if tt.ownerErr && obj.GetName() != "ldap-exposed-ports" {
						return assert.AnError
					}
					return nil
				},
				SetCondition: func(_ context.Context, kind string, status bool, reason, message string) error {
					assert.Equal(t, NetworkPoliciesConditionType, kind)
					assert.False(t, status)
					assert.Equal(t, networkPoliciesGenerationFailedConditionReason, reason)
					assert.Contains(t, message, tt.want)
					return nil
				},
			}
			if tt.protocol == corev1.ProtocolTCP {
				exposition.TcpRoutes = types.ExposedPorts{route}
			} else {
				exposition.UdpRoutes = types.ExposedPorts{route}
			}
			err := n.ProcessExposition(t.Context(), exposition)
			require.Error(t, err)
			assert.ErrorContains(t, err, tt.want)
			if tt.ownerErr {
				assert.ErrorIs(t, err, assert.AnError)
			}
		})
	}
}

func TestNetworkPolicy_ProcessExposition_ExternalDeletionError(t *testing.T) {
	c := newFakeClientWithInterceptor(t, interceptor.Funcs{
		Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
			return assert.AnError
		},
	})
	n := fixedNetworkPolicy(t, c, nil)
	conditionCalled := false
	exposition := types.Exposition{
		Name: "ldap", Namespace: testNamespace,
		SetCondition: func(_ context.Context, kind string, status bool, reason, _ string) error {
			conditionCalled = true
			assert.Equal(t, NetworkPoliciesConditionType, kind)
			assert.False(t, status)
			assert.Equal(t, networkPoliciesDeletionFailedConditionReason, reason)
			return nil
		},
	}
	err := n.ProcessExposition(t.Context(), exposition)
	assert.ErrorIs(t, err, assert.AnError)
	assert.True(t, conditionCalled)
}
