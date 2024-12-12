package expose

import (
	"context"
	"encoding/json"
	"fmt"
	doguv2 "github.com/cloudogu/k8s-dogu-operator/v2/api/v2"
	"github.com/cloudogu/k8s-service-discovery/controllers/util"
	"github.com/cloudogu/retry-lib/retry"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"maps"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

const (
	// Name is limited to 63 characters (dns prefix 253).
	// With this name prefix `service-port-mapping-` the service name length can be max 45.
	// In those rare case the name will be truncated.
	mappingAnnotationServicePortDNSKeyPrefix  = "k8s.cloudogu.com"
	mappingAnnotationServicePortNameKeyPrefix = "ces-exposed-ports-"
	maxLengthAnnotationName                   = 63
)

type networkPolicyHandler struct {
	ingressController      ingressController
	networkPolicyInterface networkPolicyInterface
	allowedCIDR            string
}

func NewNetworkPolicyHandler(policyInterface networkPolicyInterface, controller ingressController, allowedCIDR string) *networkPolicyHandler {
	return &networkPolicyHandler{
		networkPolicyInterface: policyInterface,
		ingressController:      controller,
		allowedCIDR:            allowedCIDR,
	}
}

func (nph *networkPolicyHandler) UpsertNetworkPoliciesForService(ctx context.Context, service *corev1.Service) error {
	logger := log.FromContext(ctx)
	cesServiceExposedPorts, err := parseExposedPortsFromService(service)
	if err != nil {
		return err
	}

	logger.Info(fmt.Sprintf("create or update ingress networkpolicy for ingress controller %s with service %s exposed ports %q", nph.ingressController.GetName(), service.Name, cesServiceExposedPorts))
	_, err = nph.getNetworkPolicy(ctx)
	if err != nil && errors.IsNotFound(err) {
		return nph.createNetworkPolicy(ctx, service.Name, nph.ingressController.GetName(), cesServiceExposedPorts, service.Namespace)
	}

	if err != nil {
		return fmt.Errorf("failed to get networkpolicy %s: %w", getExposedNetworkPolicyName(nph.ingressController.GetName()), err)
	}

	err = nph.updateNetworkPolicy(ctx, service.Name, cesServiceExposedPorts)
	if err != nil {
		return fmt.Errorf("failed to update networkpolicy %s: %w", getExposedNetworkPolicyName(nph.ingressController.GetName()), err)
	}

	return nil
}

func (nph *networkPolicyHandler) getNetworkPolicy(ctx context.Context) (*v1.NetworkPolicy, error) {
	ingressControllerName := nph.ingressController.GetName()
	networkPolicyName := getExposedNetworkPolicyName(ingressControllerName)
	return nph.networkPolicyInterface.Get(ctx, networkPolicyName, metav1.GetOptions{})
}

func (nph *networkPolicyHandler) createNetworkPolicy(ctx context.Context, serviceName string, ingressControllerName string, ports util.ExposedPorts, namespace string) error {
	logger := log.FromContext(ctx)
	if len(ports) == 0 {
		logger.Info(fmt.Sprintf("skip creating networkpolicy for service %s because there are not exposed ports", serviceName))
		return nil
	}

	networkPolicyIngressRulePorts := getNetworkPolicyPortsFromExposedPorts(ports)

	mappingAnnotations, err := createMappingAnnotation(serviceName, ports)
	if err != nil {
		return err
	}

	policyName := getExposedNetworkPolicyName(ingressControllerName)
	networkPolicy := &v1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        policyName,
			Namespace:   namespace,
			Labels:      util.K8sCesServiceDiscoveryLabels,
			Annotations: mappingAnnotations,
		},
		Spec: v1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{doguv2.DoguLabelName: ingressControllerName}},
			PolicyTypes: []v1.PolicyType{v1.PolicyTypeIngress},
			Ingress: []v1.NetworkPolicyIngressRule{
				{
					Ports: networkPolicyIngressRulePorts,
					From: []v1.NetworkPolicyPeer{
						{
							IPBlock: &v1.IPBlock{
								CIDR: nph.allowedCIDR,
							},
						},
					},
				},
			},
		},
	}

	_, err = nph.networkPolicyInterface.Create(ctx, networkPolicy, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create networkpolicy %s: %w", policyName, err)
	}

	return nil
}

func getNetworkPolicyPortsFromExposedPorts(ports util.ExposedPorts) []v1.NetworkPolicyPort {
	var result []v1.NetworkPolicyPort
	for _, port := range ports {
		result = append(result, getNetworkPolicyPort(port))
	}
	return result
}

func (nph *networkPolicyHandler) updateNetworkPolicy(ctx context.Context, serviceName string, exposedPorts util.ExposedPorts) error {
	logger := log.FromContext(ctx)
	retryErr := retry.OnConflict(func() error {
		get, err := nph.getNetworkPolicy(ctx)
		if err != nil {
			return err
		}

		newServicePortMappingAnnotation, err := createMappingAnnotation(serviceName, exposedPorts)
		if err != nil {
			return err
		}

		if get.Annotations == nil {
			get.Annotations = map[string]string{}
		}

		actualPortsStr, ok := get.Annotations[getServicePortMappingAnnotationKey(serviceName)]
		// Service is new and the policy has no ports defined. We can just set the port slice.
		if !ok {
			if len(exposedPorts) == 0 {
				logger.Info(fmt.Sprintf("skip updating networkpolicy for service %s because there are not exposed ports", serviceName))
				return nil
			}

			maps.Copy(get.Annotations, newServicePortMappingAnnotation)

			// There should be only one ingress rule
			get.Spec.Ingress[0].Ports = append(get.Spec.Ingress[0].Ports, getNetworkPolicyPortsFromExposedPorts(exposedPorts)...)
			nph.updateCIDR(get)

			_, updateErr := nph.networkPolicyInterface.Update(ctx, get, metav1.UpdateOptions{})
			return updateErr
		}

		actualPorts, err := unmarshalCesExposedPorts(serviceName, actualPortsStr)
		if err != nil {
			return err
		}

		deletedPortList := deleteIngressPorts(get.Spec.Ingress[0].Ports, getPortsToDelete(actualPorts, exposedPorts))
		get.Spec.Ingress[0].Ports = addIngressPorts(deletedPortList, exposedPorts)
		maps.Copy(get.Annotations, newServicePortMappingAnnotation)
		nph.updateCIDR(get)

		_, updateErr := nph.networkPolicyInterface.Update(ctx, get, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		return fmt.Errorf("failed to update networkpolicy: %w", retryErr)
	}

	return nil
}

func (nph *networkPolicyHandler) updateCIDR(policy *v1.NetworkPolicy) {
	policy.Spec.Ingress[0].From[0].IPBlock.CIDR = nph.allowedCIDR
}

func unmarshalCesExposedPorts(serviceName string, exposedPortStr string) (util.ExposedPorts, error) {
	result := make(util.ExposedPorts, 0)
	err := json.Unmarshal([]byte(exposedPortStr), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal service port mapping %s for service %s: %w", exposedPortStr, serviceName, err)
	}

	return result, nil
}

func addIngressPorts(ports []v1.NetworkPolicyPort, portsToAdd util.ExposedPorts) []v1.NetworkPolicyPort {
	for _, portToAdd := range portsToAdd {
		var found bool
		for _, actualPort := range ports {
			if equalsNetpolPortExposedPort(actualPort, portToAdd) {
				found = true
				break
			}
		}
		if !found {
			ports = append(ports, getNetworkPolicyPort(portToAdd))
		}
	}

	return ports
}

// deleteIngressPorts returns the ports from the given ingress rule which are not equal with the ces exposed ports.
func deleteIngressPorts(ports []v1.NetworkPolicyPort, portsToDelete util.ExposedPorts) []v1.NetworkPolicyPort {
	var result []v1.NetworkPolicyPort

	for _, port := range ports {
		var found bool
		for _, toDelete := range portsToDelete {
			if equalsNetpolPortExposedPort(port, toDelete) {
				found = true
			}
		}
		if !found {
			result = append(result, port)
		}
	}

	return result
}

// equalsNetpolPortExposedPort returns true if the protocol and the exposed port numbers are equal
func equalsNetpolPortExposedPort(netpolPort v1.NetworkPolicyPort, exposedPort util.ExposedPort) bool {
	if !strings.EqualFold(string(*netpolPort.Protocol), string(exposedPort.Protocol)) {
		return false
	}

	// Just check the port because ces exposed ports do not support port ranges.
	if netpolPort.Port.IntValue() != int(exposedPort.Port) {
		return false
	}

	return true
}

// subtractSlice returns a string slice with elements from s1 which are not in s2
func subtractSlice(s1, s2 util.ExposedPorts) util.ExposedPorts {
	var result util.ExposedPorts
	for _, x := range s1 {
		var found bool
		for _, y := range s2 {
			if x == y {
				found = true
			}
		}

		if !found {
			result = append(result, x)
		}
	}

	return result
}

func getPortsToDelete(actual, want util.ExposedPorts) util.ExposedPorts {
	return subtractSlice(actual, want)
}

// This mapping is needed because the ports in the NetworkPolicyIngressRule do not support names like the ports in a regular service.
// To avoid creating a networkpolicy for every service we add the mapping from service to ports in the annotations.
// This information is needed if an exposed port will change and the old has to be deleted.
func createMappingAnnotation(serviceName string, exposedPorts util.ExposedPorts) (map[string]string, error) {
	key := getServicePortMappingAnnotationKey(serviceName)

	out, err := json.Marshal(exposedPorts)
	if err != nil {
		return nil, fmt.Errorf("failed to marschal ports %s for service %s: %w", exposedPorts, serviceName, err)
	}

	return map[string]string{key: string(out)}, nil
}

func getServicePortMappingAnnotationKey(serviceName string) string {
	name := fmt.Sprintf("%s%s", mappingAnnotationServicePortNameKeyPrefix, serviceName)
	if len(name) > maxLengthAnnotationName {
		name = name[0:maxLengthAnnotationName]
	}
	return fmt.Sprintf("%s/%s", mappingAnnotationServicePortDNSKeyPrefix, name)
}

func getNetworkPolicyPort(exposedPort util.ExposedPort) v1.NetworkPolicyPort {
	protocolStr := strings.ToUpper(string(exposedPort.Protocol))
	protocol := corev1.Protocol(protocolStr)
	port := intstr.FromInt32(exposedPort.Port)

	return v1.NetworkPolicyPort{
		Protocol: &protocol,
		Port:     &port,
	}
}

func getExposedNetworkPolicyName(ingressControllerName string) string {
	return fmt.Sprintf("%s-exposed", ingressControllerName)
}

func (nph *networkPolicyHandler) RemoveExposedPorts(ctx context.Context, serviceName string) error {
	logger := log.FromContext(ctx)

	retryErr := retry.OnConflict(func() error {
		get, err := nph.getNetworkPolicy(ctx)
		if err != nil && errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("skip removing networkpolicy exposed ports for service %s because the policy does not exists", serviceName))
			return nil
		}

		if err != nil {
			return fmt.Errorf("failed to get networkpolicy %s: %w", getExposedNetworkPolicyName(nph.ingressController.GetName()), err)
		}

		actualPortsStr, ok := get.Annotations[getServicePortMappingAnnotationKey(serviceName)]
		if !ok {
			logger.Info("skip removing networkpolicy exposed ports for service %s because there are no matching ports", serviceName)
			return nil
		}

		portsToDelete, err := unmarshalCesExposedPorts(serviceName, actualPortsStr)
		if err != nil {
			return err
		}

		get.Spec.Ingress[0].Ports = deleteIngressPorts(get.Spec.Ingress[0].Ports, portsToDelete)
		delete(get.Annotations, getServicePortMappingAnnotationKey(serviceName))
		_, err = nph.networkPolicyInterface.Update(ctx, get, metav1.UpdateOptions{})

		return err
	})

	if retryErr != nil {
		return fmt.Errorf("failed to delete networkpolicy exposed ports for service %s: %w", serviceName, retryErr)
	}

	return nil
}

func (nph *networkPolicyHandler) RemoveNetworkPolicy(ctx context.Context) error {
	logger := log.FromContext(ctx)
	policy, err := nph.getNetworkPolicy(ctx)
	policyName := getExposedNetworkPolicyName(nph.ingressController.GetName())
	if err != nil && errors.IsNotFound(err) {
		logger.Info(fmt.Sprintf("do not delete network policy %s because it does not exists", policyName))
		return nil
	}

	if err != nil {
		return err
	}

	err = nph.networkPolicyInterface.Delete(ctx, policy.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete network policy %s: %w", policyName, err)
	}

	return nil
}
