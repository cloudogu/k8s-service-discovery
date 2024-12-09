package expose

import (
	"context"
	"encoding/json"
	"fmt"
	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

const (
	cesLoadbalancerName       = "ces-loadbalancer"
	cesExposedPortsAnnotation = "k8s-dogu-operator.cloudogu.com/ces-exposed-ports"
)

type exposedPortHandler struct {
	serviceInterface  serviceInterface
	ingressController ingressController
	namespace         string
}

// NewExposedPortHandler creates a new instance of exposedPortHandler.
func NewExposedPortHandler(serviceInterface serviceInterface, ingressController ingressController, namespace string) *exposedPortHandler {
	return &exposedPortHandler{
		serviceInterface:  serviceInterface,
		ingressController: ingressController,
		namespace:         namespace,
	}
}

// UpsertCesLoadbalancerService updates the loadbalancer service "ces-loadbalancer" with the dogu exposed ports.
// If the service is not existent in cluster, it will be created.
// If the dogu has no exposed ports, this method returns an empty service object and nil.
func (deph *exposedPortHandler) UpsertCesLoadbalancerService(ctx context.Context, service *corev1.Service) error {
	logger := log.FromContext(ctx)
	cesServiceExposedPorts, err := parseExposedPortsFromService(service)
	if err != nil {
		return err
	}
	targetServiceName := service.Name

	if len(cesServiceExposedPorts) == 0 {
		logger.Info(fmt.Sprintf("Skipping loadbalancer creation because the are no exposed ports for target service %s...", targetServiceName))
		return nil
	}

	lbService, err := deph.getCesLoadBalancerService(ctx)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get loadbalancer service %s: %w", cesLoadbalancerName, err)
	} else if err != nil && apierrors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("Loadbalancer service %s does not exist. Create a new one...", cesLoadbalancerName))
		_, createErr := deph.createCesLoadbalancerService(ctx, targetServiceName, cesServiceExposedPorts)
		if createErr != nil {
			return fmt.Errorf("failed to create %s loadbalancer service: %w", cesLoadbalancerName, createErr)
		}

		err = deph.ingressController.ExposeOrUpdateExposedPorts(ctx, deph.namespace, targetServiceName, cesServiceExposedPorts)
		if err != nil {
			return fmt.Errorf("failed to expose ces-services %q: %w", cesServiceExposedPorts, err)
		}

		return nil
	}

	logger.Info(fmt.Sprintf("Update loadbalancer service %s...", cesLoadbalancerName))
	lbService = updateCesLoadbalancerService(targetServiceName, lbService, cesServiceExposedPorts)

	err = deph.ingressController.ExposeOrUpdateExposedPorts(ctx, deph.namespace, targetServiceName, cesServiceExposedPorts)
	if err != nil {
		return fmt.Errorf("failed to expose ces-services %q: %w", cesServiceExposedPorts, err)
	}

	// TODO retry
	err = deph.updateService(ctx, lbService)
	if err != nil {
		return fmt.Errorf("failed to update loadbalancer service %s: %w", cesLoadbalancerName, err)
	}

	return nil
}

func parseExposedPortsFromService(service *corev1.Service) (util.ExposedPorts, error) {
	cesExposedPortsStr, ok := service.Annotations[cesExposedPortsAnnotation]
	if !ok {
		return util.ExposedPorts{}, nil
	}

	cesExposedPorts := &util.ExposedPorts{}

	err := json.Unmarshal([]byte(cesExposedPortsStr), cesExposedPorts)
	if err != nil {
		return util.ExposedPorts{}, fmt.Errorf("failed to unmarshal ces exposed ports annotation %q from service %q: %w", cesExposedPortsAnnotation, service.Name, err)
	}

	// Validate: Ports should be in Service Spec
	for _, port := range *cesExposedPorts {
		found := false
		for _, servicePort := range service.Spec.Ports {
			if servicePort.Port == int32(port.Port) {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid service annotation %q. port %q is not defined in service ports", cesExposedPortsAnnotation, port.Port)
		}
	}

	return *cesExposedPorts, nil
}

func (deph *exposedPortHandler) getCesLoadBalancerService(ctx context.Context) (*corev1.Service, error) {
	return deph.serviceInterface.Get(ctx, cesLoadbalancerName, metav1.GetOptions{})
}

func (deph *exposedPortHandler) createCesLoadbalancerService(ctx context.Context, targetServiceName string, exposedPorts util.ExposedPorts) (*corev1.Service, error) {
	ipSingleStackPolicy := corev1.IPFamilyPolicySingleStack
	exposedService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cesLoadbalancerName,
			Namespace: deph.namespace,
			Labels:    util.GetAppLabel(),
		},
		Spec: corev1.ServiceSpec{
			Type:           corev1.ServiceTypeLoadBalancer,
			IPFamilyPolicy: &ipSingleStackPolicy,
			IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol},
			Selector: map[string]string{
				k8sv2.DoguLabelName: deph.ingressController.GetName(),
			},
		},
	}

	var servicePorts []corev1.ServicePort
	for _, port := range exposedPorts {
		servicePorts = append(servicePorts, getServicePortFromExposedPort(targetServiceName, port))
	}
	exposedService.Spec.Ports = servicePorts

	_, err := deph.serviceInterface.Create(ctx, exposedService, metav1.CreateOptions{})
	if err != nil {
		return exposedService, fmt.Errorf("failed to create %s service: %w", cesLoadbalancerName, err)
	}

	return exposedService, nil
}

func getServicePortFromExposedPort(targetServiceName string, exposedPort util.ExposedPort) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       fmt.Sprintf("%s%d", getTargetServicePortNamePrefix(targetServiceName), exposedPort.Port),
		Protocol:   exposedPort.Protocol,
		Port:       int32(exposedPort.Port),
		TargetPort: intstr.FromInt(exposedPort.TargetPort),
	}
}

func getTargetServicePortNamePrefix(targetServiceName string) string {
	return fmt.Sprintf("%s-", targetServiceName)
}

func updateCesLoadbalancerService(targetServiceName string, lbService *corev1.Service, exposedPorts util.ExposedPorts) *corev1.Service {
	lbService.Spec.Ports = filterTargetServicePorts(targetServiceName, lbService)

	for _, port := range exposedPorts {
		lbService.Spec.Ports = append(lbService.Spec.Ports, getServicePortFromExposedPort(targetServiceName, port))
	}

	return lbService
}

// filterTargetServicePorts returns all ports from the service filtered by the service name prefix
func filterTargetServicePorts(targetServiceName string, lbService *corev1.Service) []corev1.ServicePort {
	var servicePorts []corev1.ServicePort

	for _, servicePort := range lbService.Spec.Ports {
		servicePortName := servicePort.Name
		servicePrefix := getTargetServicePortNamePrefix(targetServiceName)
		f := strings.HasPrefix(servicePortName, servicePrefix)
		if !f {
			servicePorts = append(servicePorts, servicePort)
		}
	}

	return servicePorts
}

func (deph *exposedPortHandler) updateService(ctx context.Context, exposedService *corev1.Service) error {
	_, err := deph.serviceInterface.Update(ctx, exposedService, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update %s service: %w", cesLoadbalancerName, err)
	}
	return nil
}

// RemoveExposedPorts removes given dogu exposed ports from the loadbalancer service.
// If these ports are the only ones, the service will be deleted.
// If the dogu has no exposed ports, the method returns nil.
func (deph *exposedPortHandler) RemoveExposedPorts(ctx context.Context, service *corev1.Service) error {
	logger := log.FromContext(ctx)

	cesServiceExposedPorts, err := parseExposedPortsFromService(service)
	if err != nil {
		return err
	}
	targetServiceName := service.Name

	if len(cesServiceExposedPorts) == 0 {
		logger.Info(fmt.Sprintf("Skipping deletion from loadbalancer service because the target service %s has no exposed ports...", targetServiceName))
		return nil
	}

	logger.Info("Delete exposed tcp and upd ports...")
	err = deph.ingressController.DeleteExposedPorts(ctx, deph.namespace, targetServiceName, cesServiceExposedPorts)
	if err != nil {
		return fmt.Errorf("failed to delete entries from expose configmap: %w", err)
	}

	exposedService, err := deph.getCesLoadBalancerService(ctx)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to get service %s: %w", cesLoadbalancerName, err)
		} else {
			return nil
		}
	}

	ports := filterTargetServicePorts(targetServiceName, exposedService)
	if len(ports) > 0 {
		logger.Info("Update loadbalancer service...")
		exposedService.Spec.Ports = ports
		// TODO retry
		return deph.updateService(ctx, exposedService)
	}

	logger.Info("Delete loadbalancer service because no ports are remaining...")
	err = deph.serviceInterface.Delete(ctx, exposedService.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete service %s: %w", cesLoadbalancerName, err)
	}

	return nil
}
