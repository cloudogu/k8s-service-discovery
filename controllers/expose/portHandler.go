package expose

import (
	"context"
	"encoding/json"
	"fmt"
	k8sv2 "github.com/cloudogu/k8s-dogu-operator/v3/api/v2"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	"github.com/cloudogu/retry-lib/retry"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

const (
	cesLoadbalancerName = "ces-loadbalancer"
	// CesExposedPortsAnnotation can be appended to service with information of exposed ports from dogu descriptors.
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
func (eph *exposedPortHandler) UpsertCesLoadbalancerService(ctx context.Context, service *corev1.Service) error {
	logger := log.FromContext(ctx)
	cesServiceExposedPorts, parseErr := parseExposedPortsFromService(service)
	if parseErr != nil {
		return parseErr
	}
	targetServiceName := service.Name

	if len(cesServiceExposedPorts) == 0 {
		logger.Info(fmt.Sprintf("Skipping loadbalancer creation because the are no exposed ports for target service %s...", targetServiceName))
		return nil
	}

	retryErr := retry.OnConflict(func() error {
		lbService, err := eph.getCesLoadBalancerService(ctx)
		if err != nil && !apierrors.IsNotFound(err) {
			return errorGetLoadBalancerService(err)
		} else if err != nil && apierrors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("Loadbalancer service %s does not exist. Create a new one...", cesLoadbalancerName))
			_, createErr := eph.createCesLoadbalancerService(ctx, targetServiceName, cesServiceExposedPorts)
			if createErr != nil {
				return fmt.Errorf("failed to create %s loadbalancer service: %w", cesLoadbalancerName, createErr)
			}

			err = eph.ingressController.ExposeOrUpdateExposedPorts(ctx, eph.namespace, targetServiceName, cesServiceExposedPorts)
			if err != nil {
				return fmt.Errorf("failed to expose ces-services %q: %w", cesServiceExposedPorts, err)
			}

			return nil
		}

		err = eph.ingressController.ExposeOrUpdateExposedPorts(ctx, eph.namespace, targetServiceName, cesServiceExposedPorts)
		if err != nil {
			return fmt.Errorf("failed to expose ces-services %q: %w", cesServiceExposedPorts, err)
		}

		lbService, changed := updateCesLoadbalancerService(targetServiceName, lbService, cesServiceExposedPorts)
		if !changed {
			logger.Info(fmt.Sprintf("no loadbalancer service %s update required for service %s...", cesLoadbalancerName, targetServiceName))
			return nil
		}

		logger.Info(fmt.Sprintf("Update loadbalancer service %s...", cesLoadbalancerName))
		err = eph.updateService(ctx, lbService)
		if err != nil {
			return fmt.Errorf("failed to update loadbalancer service %s: %w", cesLoadbalancerName, err)
		}

		return nil
	})

	if retryErr != nil {
		return fmt.Errorf("failed to upsert loadbalancer service %s: %w", cesLoadbalancerName, retryErr)
	}

	return nil
}

func errorGetLoadBalancerService(err error) error {
	return fmt.Errorf("failed to get loadbalancer service %s: %w", cesLoadbalancerName, err)
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
			if equalsServicePortExposedPort(servicePort, port) {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid service annotation %q. port %q is not defined in service ports", cesExposedPortsAnnotation, port.Port)
		}
	}

	return *cesExposedPorts, nil
}

func (eph *exposedPortHandler) getCesLoadBalancerService(ctx context.Context) (*corev1.Service, error) {
	return eph.serviceInterface.Get(ctx, cesLoadbalancerName, metav1.GetOptions{})
}

func (eph *exposedPortHandler) createCesLoadbalancerService(ctx context.Context, targetServiceName string, exposedPorts util.ExposedPorts) (*corev1.Service, error) {
	ipSingleStackPolicy := corev1.IPFamilyPolicySingleStack
	exposedService := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cesLoadbalancerName,
			Namespace: eph.namespace,
			Labels:    util.GetAppLabel(),
		},
		Spec: corev1.ServiceSpec{
			Type:           corev1.ServiceTypeLoadBalancer,
			IPFamilyPolicy: &ipSingleStackPolicy,
			IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol},
			Selector: map[string]string{
				k8sv2.DoguLabelName: eph.ingressController.GetName(),
			},
		},
	}

	var servicePorts []corev1.ServicePort
	for _, port := range exposedPorts {
		servicePorts = append(servicePorts, getServicePortFromExposedPort(targetServiceName, port))
	}
	exposedService.Spec.Ports = servicePorts

	_, err := eph.serviceInterface.Create(ctx, exposedService, metav1.CreateOptions{})
	if err != nil {
		return exposedService, fmt.Errorf("failed to create %s service: %w", cesLoadbalancerName, err)
	}

	return exposedService, nil
}

func getServicePortFromExposedPort(targetServiceName string, exposedPort util.ExposedPort) corev1.ServicePort {
	return corev1.ServicePort{
		Name:       fmt.Sprintf("%s%d", getTargetServicePortNamePrefix(targetServiceName), exposedPort.Port),
		Protocol:   corev1.Protocol(strings.ToUpper(string(exposedPort.Protocol))),
		Port:       exposedPort.Port,
		TargetPort: intstr.FromInt32(exposedPort.TargetPort),
	}
}

func getTargetServicePortNamePrefix(targetServiceName string) string {
	return fmt.Sprintf("%s-", targetServiceName)
}

func updateCesLoadbalancerService(targetServiceName string, lbService *corev1.Service, exposedPorts util.ExposedPorts) (*corev1.Service, bool) {
	var found bool
	lbService.Spec.Ports, found = filterTargetServicePorts(targetServiceName, lbService)

	if !found && len(exposedPorts) == 0 {
		return lbService, false
	}

	for _, port := range exposedPorts {
		lbService.Spec.Ports = append(lbService.Spec.Ports, getServicePortFromExposedPort(targetServiceName, port))
	}

	return lbService, true
}

// filterTargetServicePorts returns all ports from the service filtered by the service name prefix.
// If the service has no ports in the service it additionally returns false. Otherwise, true.
func filterTargetServicePorts(targetServiceName string, lbService *corev1.Service) ([]corev1.ServicePort, bool) {
	var servicePorts []corev1.ServicePort
	found := false

	for _, servicePort := range lbService.Spec.Ports {
		servicePortName := servicePort.Name
		servicePrefix := getTargetServicePortNamePrefix(targetServiceName)
		f := strings.HasPrefix(servicePortName, servicePrefix)
		if !f {
			servicePorts = append(servicePorts, servicePort)
		} else {
			found = true
		}
	}

	return servicePorts, found
}

func (eph *exposedPortHandler) updateService(ctx context.Context, exposedService *corev1.Service) error {
	_, err := eph.serviceInterface.Update(ctx, exposedService, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update %s service: %w", cesLoadbalancerName, err)
	}
	return nil
}

// RemoveExposedPorts removes given dogu exposed ports from the loadbalancer service.
// If these ports are the only ones, the service will be deleted.
// If the dogu has no exposed ports, the method returns nil.
func (eph *exposedPortHandler) RemoveExposedPorts(ctx context.Context, serviceName string) error {
	logger := log.FromContext(ctx)

	logger.Info("Delete exposed tcp and upd ports...")
	err := eph.ingressController.DeleteExposedPorts(ctx, eph.namespace, serviceName)
	if err != nil {
		return fmt.Errorf("failed to delete entries from expose configmap: %w", err)
	}

	retryErr := retry.OnConflict(func() error {
		exposedService, err := eph.getCesLoadBalancerService(ctx)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to get service %s: %w", cesLoadbalancerName, err)
			} else {
				return nil
			}
		}

		ports, found := filterTargetServicePorts(serviceName, exposedService)
		if !found {
			logger.Info(fmt.Sprintf("found no exposed ports for service %s in loadbalancer service %s", serviceName, cesLoadbalancerName))
			return nil
		}

		logger.Info("Update loadbalancer service...")
		// Do not delete the loadbalancer service even it has no ports because this could result in a new ip if it gets recreated.
		exposedService.Spec.Ports = ports
		return eph.updateService(ctx, exposedService)
	})

	if retryErr != nil {
		return fmt.Errorf("failed to remove exposed ports from loadbalancer service %s: %w", cesLoadbalancerName, retryErr)
	}

	return nil
}

func equalsServicePortExposedPort(servicePort corev1.ServicePort, exposedPort util.ExposedPort) bool {
	if strings.ToUpper(string(servicePort.Protocol)) != strings.ToUpper(string(exposedPort.Protocol)) {
		return false
	}

	if servicePort.Port != exposedPort.Port {
		return false
	}

	return true
}
