package expose

import (
	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"testing"
)

func TestNewExposedPortHandler(t *testing.T) {
	// given
	serviceInterfaceMock := newMockServiceInterface(t)
	ingressControllerMock := newMockIngressController(t)

	// when
	sut := NewExposedPortHandler(serviceInterfaceMock, ingressControllerMock, testNamespace)

	// then
	require.NotNil(t, sut)
}

var (
	unexposedService  = &v1.Service{}
	scmServiceName    = "scm"
	scmExposedService = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        scmServiceName,
			Namespace:   testNamespace,
			Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "[{\"port\": 2222, \"targetPort\": 2222, \"protocol\": \"TCP\"}]"},
		},
		Spec: v1.ServiceSpec{Ports: []v1.ServicePort{
			{
				Port:       2222,
				TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 2222},
				Protocol:   v1.ProtocolTCP,
			},
		}},
	}
	scmExposedPorts = util.ExposedPorts{{Port: 2222, TargetPort: 2222, Protocol: v1.ProtocolTCP}}

	ipSingleStackPolicy = v1.IPFamilyPolicySingleStack
	lbService           = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ces-loadbalancer",
			Namespace: testNamespace,
			Labels:    map[string]string{"app": "ces"},
		},
		Spec: v1.ServiceSpec{
			Type:           v1.ServiceTypeLoadBalancer,
			IPFamilyPolicy: &ipSingleStackPolicy,
			IPFamilies:     []v1.IPFamily{v1.IPv4Protocol},
			Selector: map[string]string{
				k8sv2.DoguLabelName: "nginx-ingress",
			},
			Ports: []v1.ServicePort{
				{
					Name:       "scm-2222",
					Port:       2222,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 2222},
					Protocol:   v1.ProtocolTCP,
				},
			},
		},
	}
)

func Test_doguExposedPortHandler_CreateOrUpdateCesLoadbalancerService(t *testing.T) {
	t.Run("should return nil if service has no exposed ports", func(t *testing.T) {
		// given
		sut := &exposedPortHandler{}

		// when
		err := sut.UpsertCesLoadbalancerService(testCtx, unexposedService)

		// then
		require.Nil(t, err)
	})

	t.Run("should return error on error getting ces loadbalancer", func(t *testing.T) {
		// given
		serviceInterfaceMock := newMockServiceInterface(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(nil, assert.AnError)
		sut := &exposedPortHandler{serviceInterface: serviceInterfaceMock}

		// when
		err := sut.UpsertCesLoadbalancerService(testCtx, scmExposedService)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get loadbalancer service ces-loadbalancer")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should return an error on service creation error", func(t *testing.T) {
		// given
		serviceInterfaceMock := newMockServiceInterface(t)
		ingressControllerMock := newMockIngressController(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		serviceInterfaceMock.EXPECT().Create(testCtx, lbService, metav1.CreateOptions{}).Return(nil, assert.AnError)
		ingressControllerMock.EXPECT().GetName().Return("nginx-ingress")

		sut := &exposedPortHandler{serviceInterface: serviceInterfaceMock, ingressController: ingressControllerMock, namespace: testNamespace}

		// when
		err := sut.UpsertCesLoadbalancerService(testCtx, scmExposedService)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to create ces-loadbalancer service")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should return an error on tcp/udp exposure error", func(t *testing.T) {
		// given
		serviceInterfaceMock := newMockServiceInterface(t)
		ingressControllerMock := newMockIngressController(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		serviceInterfaceMock.EXPECT().Create(testCtx, lbService, metav1.CreateOptions{}).Return(nil, nil)
		ingressControllerMock.EXPECT().ExposeOrUpdateExposedPorts(testCtx, testNamespace, scmServiceName, scmExposedPorts).Return(assert.AnError)
		ingressControllerMock.EXPECT().GetName().Return("nginx-ingress")

		sut := &exposedPortHandler{serviceInterface: serviceInterfaceMock, ingressController: ingressControllerMock, namespace: testNamespace}

		// when
		err := sut.UpsertCesLoadbalancerService(testCtx, scmExposedService)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to expose ces-services [\"{Port: 2222, TargetPort: 2222, Protocol: TCP}\"]")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should return an error on exposing tcp/udp service error if a loadbalancer is available", func(t *testing.T) {
		// given
		serviceInterfaceMock := newMockServiceInterface(t)
		ingressControllerMock := newMockIngressController(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(lbService, nil)
		ingressControllerMock.EXPECT().ExposeOrUpdateExposedPorts(testCtx, testNamespace, scmServiceName, scmExposedPorts).Return(assert.AnError)

		sut := &exposedPortHandler{serviceInterface: serviceInterfaceMock, ingressController: ingressControllerMock, namespace: testNamespace}

		// when
		err := sut.UpsertCesLoadbalancerService(testCtx, scmExposedService)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to expose ces-services [\"{Port: 2222, TargetPort: 2222, Protocol: TCP}\"]")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should update the existing loadbalancer", func(t *testing.T) {
		// given
		serviceInterfaceMock := newMockServiceInterface(t)
		ingressControllerMock := newMockIngressController(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(lbService, nil)
		serviceInterfaceMock.EXPECT().Update(testCtx, lbService, metav1.UpdateOptions{}).Return(nil, nil)
		changeExposedPorts := util.ExposedPorts{{Port: 2222, TargetPort: 1111, Protocol: v1.ProtocolTCP}}
		ingressControllerMock.EXPECT().ExposeOrUpdateExposedPorts(testCtx, testNamespace, scmServiceName, changeExposedPorts).Return(nil)

		sut := &exposedPortHandler{serviceInterface: serviceInterfaceMock, ingressController: ingressControllerMock, namespace: testNamespace}

		changedExposedService := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        scmServiceName,
				Namespace:   testNamespace,
				Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "[{\"port\": 2222, \"targetPort\": 1111, \"protocol\": \"TCP\"}]"},
			},
			Spec: v1.ServiceSpec{Ports: []v1.ServicePort{
				{
					Port:       2222,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 1111},
					Protocol:   v1.ProtocolTCP,
				},
			}},
		}

		// when
		err := sut.UpsertCesLoadbalancerService(testCtx, changedExposedService)

		// then
		require.NoError(t, err)
	})

	t.Run("should return an error on service update error", func(t *testing.T) {
		// given
		serviceInterfaceMock := newMockServiceInterface(t)
		ingressControllerMock := newMockIngressController(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(lbService, nil)
		serviceInterfaceMock.EXPECT().Update(testCtx, lbService, metav1.UpdateOptions{}).Return(nil, assert.AnError)
		changeExposedPorts := util.ExposedPorts{{Port: 2222, TargetPort: 1111, Protocol: v1.ProtocolTCP}}
		ingressControllerMock.EXPECT().ExposeOrUpdateExposedPorts(testCtx, testNamespace, scmServiceName, changeExposedPorts).Return(nil)

		sut := &exposedPortHandler{serviceInterface: serviceInterfaceMock, ingressController: ingressControllerMock, namespace: testNamespace}

		changedExposedService := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        scmServiceName,
				Namespace:   testNamespace,
				Annotations: map[string]string{"k8s-dogu-operator.cloudogu.com/ces-exposed-ports": "[{\"port\": 2222, \"targetPort\": 1111, \"protocol\": \"TCP\"}]"},
			},
			Spec: v1.ServiceSpec{Ports: []v1.ServicePort{
				{
					Port:       2222,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 1111},
					Protocol:   v1.ProtocolTCP,
				},
			}},
		}

		// when
		err := sut.UpsertCesLoadbalancerService(testCtx, changedExposedService)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to update ces-loadbalancer service")
		assert.ErrorIs(t, err, assert.AnError)
	})
}

func Test_doguExposedPortHandler_RemoveExposedPorts(t *testing.T) {
	/*t.Run("should do nothing if the dogu has no exposed ports", func(t *testing.T) {
		// given
		sut := &exposedPortHandler{}

		// when
		err := sut.RemoveExposedPorts(testCtx, &v1.Service{})

		// then
		require.NoError(t, err)
	})*/

	t.Run("should return error on tcp/udp exposure error", func(t *testing.T) {
		// given
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().DeleteExposedPorts(testCtx, testNamespace, scmServiceName).Return(assert.AnError)
		sut := &exposedPortHandler{ingressController: ingressControllerMock, namespace: testNamespace}

		// when
		err := sut.RemoveExposedPorts(testCtx, scmServiceName)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to delete entries from expose configmap")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should do nothing if no loadbalancer service exists", func(t *testing.T) {
		// given
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().DeleteExposedPorts(testCtx, testNamespace, scmServiceName).Return(nil)
		serviceInterfaceMock := newMockServiceInterface(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		sut := &exposedPortHandler{ingressController: ingressControllerMock, namespace: testNamespace, serviceInterface: serviceInterfaceMock}

		// when
		err := sut.RemoveExposedPorts(testCtx, scmServiceName)

		// then
		require.NoError(t, err)
	})

	t.Run("should return an error on service get error", func(t *testing.T) {
		// given
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().DeleteExposedPorts(testCtx, testNamespace, scmServiceName).Return(nil)
		serviceInterfaceMock := newMockServiceInterface(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(nil, assert.AnError)
		sut := &exposedPortHandler{ingressController: ingressControllerMock, namespace: testNamespace, serviceInterface: serviceInterfaceMock}

		// when
		err := sut.RemoveExposedPorts(testCtx, scmServiceName)

		// then
		require.Error(t, err)
		assert.ErrorContains(t, err, "failed to get service ces-loadbalancer")
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("should update the loadbalancer service ports if others are existent", func(t *testing.T) {
		// given
		existingLB := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "ces-loadbalancer", Namespace: testNamespace},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeLoadBalancer,
				Ports: []v1.ServicePort{{Name: "scm-2222", Port: 2222, TargetPort: intstr.IntOrString{IntVal: 2222}, Protocol: v1.ProtocolTCP},
					{Name: "nginx-ingress-80", Port: 80, TargetPort: intstr.IntOrString{IntVal: 80}, Protocol: v1.ProtocolTCP},
					{Name: "nginx-ingress-443", Port: 443, TargetPort: intstr.IntOrString{IntVal: 443}, Protocol: v1.ProtocolTCP}},
			},
		}
		expectedLB := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "ces-loadbalancer", Namespace: testNamespace},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeLoadBalancer,
				Ports: []v1.ServicePort{
					{Name: "nginx-ingress-80", Port: 80, TargetPort: intstr.IntOrString{IntVal: 80}, Protocol: v1.ProtocolTCP},
					{Name: "nginx-ingress-443", Port: 443, TargetPort: intstr.IntOrString{IntVal: 443}, Protocol: v1.ProtocolTCP},
				},
			}}

		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().DeleteExposedPorts(testCtx, testNamespace, scmServiceName).Return(nil)
		serviceInterfaceMock := newMockServiceInterface(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(existingLB, nil)
		serviceInterfaceMock.EXPECT().Update(testCtx, expectedLB, metav1.UpdateOptions{}).Return(nil, nil)
		sut := &exposedPortHandler{ingressController: ingressControllerMock, namespace: testNamespace, serviceInterface: serviceInterfaceMock}

		// when
		err := sut.RemoveExposedPorts(testCtx, scmServiceName)

		// then
		require.NoError(t, err)
	})

	t.Run("should not delete the service if the dogu ports are the only ones (avoid getting new ip)", func(t *testing.T) {
		// given
		existingLB := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "ces-loadbalancer", Namespace: "ecosystem"},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeLoadBalancer,
				Ports: []v1.ServicePort{
					{Name: "scm-2222", Port: 2222, TargetPort: intstr.IntOrString{IntVal: 2222}, Protocol: v1.ProtocolTCP},
				},
			},
		}

		emptyLB := &v1.Service{
			ObjectMeta: metav1.ObjectMeta{Name: "ces-loadbalancer", Namespace: "ecosystem"},
			Spec: v1.ServiceSpec{
				Type: v1.ServiceTypeLoadBalancer,
			},
		}

		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().DeleteExposedPorts(testCtx, testNamespace, scmServiceName).Return(nil)
		serviceInterfaceMock := newMockServiceInterface(t)
		serviceInterfaceMock.EXPECT().Get(testCtx, "ces-loadbalancer", metav1.GetOptions{}).Return(existingLB, nil)
		serviceInterfaceMock.EXPECT().Update(testCtx, emptyLB, metav1.UpdateOptions{}).Return(nil, nil)
		sut := &exposedPortHandler{ingressController: ingressControllerMock, namespace: testNamespace, serviceInterface: serviceInterfaceMock}

		// when
		err := sut.RemoveExposedPorts(testCtx, scmServiceName)

		// then
		require.NoError(t, err)
	})
}
