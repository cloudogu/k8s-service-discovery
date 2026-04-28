package expose

import (
	"context"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var testCtx = context.Background()

const (
	testNamespace        = "my-Namespace"
	testIngressClassName = "my-ingress-class-name"
)

func getTestIngress(ingressName string, path string, service corev1.Service, targetServiceName string, targetPort int32, annotations map[string]string) *v1.Ingress {
	pathType := v1.PathTypePrefix
	ingressClassName := testIngressClassName
	return &v1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   testNamespace,
			Annotations: annotations,
			Labels:      util.K8sCesServiceDiscoveryLabels,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: service.APIVersion,
				Kind:       service.Kind,
				Name:       service.Name,
				UID:        service.UID,
			}},
		},
		Spec: v1.IngressSpec{
			IngressClassName: &ingressClassName,
			Rules: []v1.IngressRule{
				{
					IngressRuleValue: v1.IngressRuleValue{
						HTTP: &v1.HTTPIngressRuleValue{
							Paths: []v1.HTTPIngressPath{
								{
									Path:     path,
									PathType: &pathType,
									Backend: v1.IngressBackend{
										Service: &v1.IngressServiceBackend{
											Name: targetServiceName,
											Port: v1.ServiceBackendPort{
												Number: targetPort,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
