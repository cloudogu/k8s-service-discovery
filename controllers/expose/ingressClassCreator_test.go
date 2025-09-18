package expose

import (
	"context"
	"testing"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/require"
)

var testCtx = context.Background()

func TestNewIngressClassCreator(t *testing.T) {
	ingressClassMock := newMockIngressClassInterface(t)
	deploymentMock := newMockDeploymentInterface(t)
	ingressControllerMock := newMockIngressController(t)

	creator := NewIngressClassCreator(ingressClassMock, deploymentMock, "my-ingress-class", newMockEventRecorder(t), ingressControllerMock)

	require.NotNil(t, creator)
	require.NotNil(t, creator.ingressClassInterface)
	require.NotNil(t, creator.deploymentInterface)
	require.NotNil(t, creator.eventRecorder)
	require.NotNil(t, creator.ingressController)

	require.Equal(t, "my-ingress-class", creator.className)
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
