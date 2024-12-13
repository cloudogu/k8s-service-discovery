package expose

import (
	"context"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"

	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

var testCtx = context.Background()

func TestNewIngressClassCreator(t *testing.T) {
	namespace := "test"
	clientSetMock := newMockClientSetInterface(t)
	appsv1Mock := newMockAppsv1Interface(t)
	appsv1Mock.EXPECT().Deployments(namespace).Return(newMockDeploymentInterface(t))
	clientSetMock.EXPECT().AppsV1().Return(appsv1Mock)
	netv1Mock := newMockNetInterface(t)
	netv1Mock.EXPECT().IngressClasses().Return(newMockIngressClassInterface(t))
	clientSetMock.EXPECT().NetworkingV1().Return(netv1Mock)
	creator := NewIngressClassCreator(clientSetMock, "my-ingress-class", namespace, newMockEventRecorder(t), nil)

	require.NotNil(t, creator)
}

func TestIngressClassCreator_CreateIngressClass(t *testing.T) {
	namespace := "test"
	className := "my-ingress-class"
	selfDeployment := &v1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "k8s-service-discovery-controller-manager", Namespace: namespace}}
	ingressClass := &networking.IngressClass{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:   className,
			Labels: util.K8sCesServiceDiscoveryLabels,
		},
		Spec: networking.IngressClassSpec{
			Controller: "k8s.io/nginx-ingress",
		},
	}

	t.Run("failed to get deployment", func(t *testing.T) {
		t.Parallel()

		// given
		ingressClassMock := newMockIngressClassInterface(t)
		ingressClassMock.EXPECT().Get(testCtx, className, metav1.GetOptions{}).Return(ingressClass, nil)
		deploymentInterfaceMock := newMockDeploymentInterface(t)
		deploymentInterfaceMock.EXPECT().Get(testCtx, "k8s-service-discovery-controller-manager", metav1.GetOptions{}).Return(nil, assert.AnError)
		ingressControllerMock := newMockIngressController(t)

		sut := ingressClassCreator{
			className:             className,
			ingressClassInterface: ingressClassMock,
			deploymentInterface:   deploymentInterfaceMock,
			ingressController:     ingressControllerMock,
		}

		// when
		err := sut.CreateIngressClass(testCtx)

		// then
		require.ErrorContains(t, err, "create ingress class: failed to get deployment [k8s-service-discovery-controller-manager]")
	})

	t.Run("ingress class does not exists and is begin created", func(t *testing.T) {
		t.Parallel()

		// given
		ingressClassMock := newMockIngressClassInterface(t)
		ingressClassMock.EXPECT().Get(testCtx, className, metav1.GetOptions{}).Return(nil, errors.NewNotFound(schema.GroupResource{}, "not found"))
		ingressClassMock.EXPECT().Create(testCtx, ingressClass, metav1.CreateOptions{}).Return(nil, nil)
		deploymentInterfaceMock := newMockDeploymentInterface(t)
		deploymentInterfaceMock.EXPECT().Get(testCtx, "k8s-service-discovery-controller-manager", metav1.GetOptions{}).Return(selfDeployment, nil)
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(selfDeployment), "Normal", "IngressClassCreation",
			"Ingress class [%s] created.", className)
		ingressControllerMock := newMockIngressController(t)
		ingressControllerMock.EXPECT().GetControllerSpec().Return("k8s.io/nginx-ingress")

		sut := ingressClassCreator{
			className:             className,
			ingressClassInterface: ingressClassMock,
			deploymentInterface:   deploymentInterfaceMock,
			eventRecorder:         recorderMock,
			ingressController:     ingressControllerMock,
		}

		// when
		err := sut.CreateIngressClass(context.Background())

		// then
		require.NoError(t, err)
	})

	t.Run("ingress class does already exist and is not begin created", func(t *testing.T) {
		t.Parallel()

		// given
		ingressClassMock := newMockIngressClassInterface(t)
		ingressClassMock.EXPECT().Get(testCtx, className, metav1.GetOptions{}).Return(ingressClass, nil)
		deploymentInterfaceMock := newMockDeploymentInterface(t)
		deploymentInterfaceMock.EXPECT().Get(testCtx, "k8s-service-discovery-controller-manager", metav1.GetOptions{}).Return(selfDeployment, nil)
		recorderMock := newMockEventRecorder(t)
		recorderMock.EXPECT().Eventf(mock.IsType(selfDeployment), "Normal", "IngressClassCreation",
			"Ingress class [%s] already exists.", className)

		sut := ingressClassCreator{
			className:             className,
			ingressClassInterface: ingressClassMock,
			deploymentInterface:   deploymentInterfaceMock,
			eventRecorder:         recorderMock,
		}

		// when
		err := sut.CreateIngressClass(context.Background())

		// then
		require.NoError(t, err)
	})
}
