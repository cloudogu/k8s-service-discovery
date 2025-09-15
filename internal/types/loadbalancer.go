package types

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	loadbalancerConfigKey  = "config.yaml"
	LoadBalancerConfigName = "ces-loadbalancer-config"
	LoadbalancerName       = "ces-loadbalancer"

	configManagedAnnotationKey          = "k8s-service-discovery.cloudogu.com/configManagedKeys"
	configManagedAnnotationKeySeparator = ";"
)

var (
	defaultExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
	defaultInternalTrafficPolicy = ptr.To(corev1.ServiceInternalTrafficPolicyCluster)
)

type LoadbalancerConfig struct {
	Annotations           map[string]string                   `yaml:"annotations"`
	InternalTrafficPolicy corev1.ServiceInternalTrafficPolicy `yaml:"internalTrafficPolicy"`
	ExternalTrafficPolicy corev1.ServiceExternalTrafficPolicy `yaml:"externalTrafficPolicy"`
}

func ParseLoadbalancerConfig(cm *corev1.ConfigMap) (LoadbalancerConfig, error) {
	var lbConfig LoadbalancerConfig
	if err := yaml.Unmarshal([]byte(cm.Data[loadbalancerConfigKey]), &lbConfig); err != nil {
		return LoadbalancerConfig{}, fmt.Errorf("failed to unmarshal loadbalancer from config map: %w", err)
	}

	if lbConfig.ExternalTrafficPolicy == "" {
		lbConfig.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
	}

	if lbConfig.InternalTrafficPolicy == "" {
		lbConfig.InternalTrafficPolicy = corev1.ServiceInternalTrafficPolicyCluster
	}

	switch lbConfig.InternalTrafficPolicy {
	case corev1.ServiceInternalTrafficPolicyCluster, corev1.ServiceInternalTrafficPolicyLocal:
	default:
		return LoadbalancerConfig{}, fmt.Errorf("internalTrafficPolicy has invalid type %s", lbConfig.InternalTrafficPolicy)
	}

	switch lbConfig.ExternalTrafficPolicy {
	case corev1.ServiceExternalTrafficPolicyCluster, corev1.ServiceExternalTrafficPolicyLocal:
	default:
		return LoadbalancerConfig{}, fmt.Errorf("externalTrafficPolicy has invalid type %s", lbConfig.InternalTrafficPolicy)
	}

	return lbConfig, nil
}

type LoadBalancer corev1.Service

func (lb *LoadBalancer) ApplyConfig(cfg LoadbalancerConfig) {
	lbAnnotations := lb.GetAnnotations()

	// delete old config annotations
	for _, k := range getConfigAnnotationKeys(lbAnnotations) {
		delete(lbAnnotations, k)
	}

	newCfgMap := createConfigAnnotations(cfg.Annotations)
	maps.Insert(lbAnnotations, maps.All(newCfgMap))

	lb.SetAnnotations(lbAnnotations)

	lb.Spec.ExternalTrafficPolicy = cfg.ExternalTrafficPolicy
	lb.Spec.InternalTrafficPolicy = &cfg.InternalTrafficPolicy
}

func (lb *LoadBalancer) UpdateExposedPorts(ports ExposedPorts) {
	ports.SetNodePorts(lb.Spec.Ports)
	lb.Spec.Ports = ports.ToServicePorts()
}

func (lb *LoadBalancer) ToK8sService() *corev1.Service {
	if lb == nil {
		return nil
	}

	svc := corev1.Service(*lb)
	return &svc
}

func (lb *LoadBalancer) Equals(o LoadBalancer) bool {
	if lb.Name != o.Name {
		return false
	}

	if !lb.equalAnnotations(o.GetAnnotations()) {
		return false
	}

	if lb.Spec.ExternalTrafficPolicy != o.Spec.ExternalTrafficPolicy ||
		lb.Spec.InternalTrafficPolicy != o.Spec.InternalTrafficPolicy {
		return false
	}

	return lb.equalPorts(o.Spec.Ports)
}

func (lb *LoadBalancer) equalAnnotations(oAnn map[string]string) bool {
	lbConfigKeys := getConfigAnnotationKeys(lb.GetAnnotations())
	oConfigKeys := getConfigAnnotationKeys(oAnn)

	slices.Sort(lbConfigKeys)
	slices.Sort(oConfigKeys)

	if !slices.Equal(lbConfigKeys, oConfigKeys) {
		return false
	}

	for _, k := range lbConfigKeys {
		lbValue, lbOk := lb.Annotations[k]
		oValue, oOk := oAnn[k]

		if lbOk != oOk {
			return false
		}

		if lbValue != oValue {
			return false
		}
	}

	return true
}

func (lb *LoadBalancer) equalPorts(oPorts []corev1.ServicePort) bool {
	lbIndexMap := make(map[indexKey]struct{}, len(lb.Spec.Ports))
	oIndexMap := make(map[indexKey]struct{}, len(oPorts))

	for _, p := range lb.Spec.Ports {
		lbIndexMap[indexKeyOfServicePort(p)] = struct{}{}
	}

	for _, p := range oPorts {
		oIndexMap[indexKeyOfServicePort(p)] = struct{}{}
	}

	return maps.Equal(lbIndexMap, oIndexMap)
}

func (lb *LoadBalancer) GetOwnerReference(scheme *runtime.Scheme) (*metav1.OwnerReference, error) {
	gvk, err := apiutil.GVKForObject(lb.ToK8sService(), scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to get GroupVersionKind for loadbalancer: %w", err)
	}

	return &metav1.OwnerReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       lb.Name,
		UID:        lb.UID,
		Controller: ptr.To(false),
	}, nil
}

func ParseLoadBalancer(obj metav1.Object) (LoadBalancer, bool) {
	if obj.GetName() != LoadbalancerName {
		return LoadBalancer{}, false
	}

	lbService, ok := obj.(*corev1.Service)
	if !ok {
		return LoadBalancer{}, false
	}

	if lbService.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return LoadBalancer{}, false
	}

	// Ensure Annotations are set
	ann := lbService.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
		lbService.SetAnnotations(ann)
	}

	return LoadBalancer(*lbService), true
}

func CreateLoadBalancer(namespace string, cfg LoadbalancerConfig, exposedPorts ExposedPorts, selector map[string]string) LoadBalancer {
	ipSingleStackPolicy := corev1.IPFamilyPolicySingleStack
	loadbalancerService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LoadbalancerName,
			Namespace: namespace,
			Labels:    util.GetAppLabel(),
		},
		Spec: corev1.ServiceSpec{
			Type:           corev1.ServiceTypeLoadBalancer,
			IPFamilyPolicy: &ipSingleStackPolicy,
			IPFamilies:     []corev1.IPFamily{corev1.IPv4Protocol},
			Selector:       selector,
		},
	}

	// apply config
	loadbalancerService.Spec.ExternalTrafficPolicy = cfg.ExternalTrafficPolicy
	loadbalancerService.Spec.InternalTrafficPolicy = &cfg.InternalTrafficPolicy

	// apply loadbalancer
	loadbalancerService.SetAnnotations(createConfigAnnotations(cfg.Annotations))

	exposedServicePorts := make([]corev1.ServicePort, 0, len(exposedPorts))

	for _, ePort := range exposedPorts {
		exposedServicePorts = append(exposedServicePorts, ePort.ToServicePort())
	}

	loadbalancerService.Spec.Ports = exposedServicePorts

	return LoadBalancer(loadbalancerService)
}

func createConfigAnnotations(cfgAnnotations map[string]string) map[string]string {
	ann := make(map[string]string)
	annKeys := make([]string, 0, len(cfgAnnotations))

	for k, v := range cfgAnnotations {
		ann[k] = v
		annKeys = append(annKeys, k)
	}

	slices.Sort(annKeys)

	ann[configManagedAnnotationKey] = strings.Join(annKeys, configManagedAnnotationKeySeparator)

	return ann
}

func getConfigAnnotationKeys(lbAnnotations map[string]string) []string {
	keys := make([]string, 0, len(lbAnnotations))

	keysStr, ok := lbAnnotations[configManagedAnnotationKey]
	if !ok {
		return keys
	}

	return strings.Split(keysStr, configManagedAnnotationKeySeparator)
}
