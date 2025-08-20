package nginx

import (
	"context"
	"fmt"

	"github.com/cloudogu/k8s-service-discovery/v2/controllers/util"
	"github.com/cloudogu/k8s-service-discovery/v2/internal/types"
	networking "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	redirectAnnotation   = "nginx.ingress.kubernetes.io/server-snippet"
	redirectIngressPath  = "/"
	redirectPathType     = networking.PathTypePrefix
	redirectEndpointName = "ces-loadbalancer"
	redirectEndpointPort = 443
)

type IngressRedirector struct {
	ingressClassName string
	ingressInterface ingressInterface
}

func (i IngressRedirector) RedirectAlternativeFQDN(ctx context.Context, namespace string, redirectObjectName string, fqdn string, altFQDNList []types.AlternativeFQDN, setOwner func(targetObject metav1.Object) error) error {
	logger := log.FromContext(ctx)

	if len(altFQDNList) == 0 {
		if dErr := i.ingressInterface.Delete(ctx, redirectObjectName, metav1.DeleteOptions{}); dErr != nil && !apierrors.IsNotFound(dErr) {
			return fmt.Errorf("failed to delete redirect ingress: %w", dErr)
		}

		logger.Info("no alternative FQDN configured, cleared redirect ingress")

		return nil
	}

	redirectIngress := i.createRedirectIngress(namespace, redirectObjectName, fqdn, groupFQDNsBySecretName(altFQDNList))

	if oErr := setOwner(redirectIngress); oErr != nil {
		return fmt.Errorf("failed to set owner for redirect ingress: %w", oErr)
	}

	if uErr := i.upsertIngress(ctx, redirectIngress); uErr != nil {
		return fmt.Errorf("failed to upsert redirect ingress: %w", uErr)
	}

	logger.Info("applied new redirect ingress")

	return nil
}

func (i IngressRedirector) createRedirectIngress(namespace string, objectName string, fqdn string, altFQDNMap map[string][]string) *networking.Ingress {
	annotations := map[string]string{
		redirectAnnotation: fmt.Sprintf("return 308 https://%s$request_uri;", fqdn),
	}

	fdns := make([]string, 0, len(altFQDNMap))
	tlsList := make([]networking.IngressTLS, 0, len(altFQDNMap))
	for certificateName, fqdnList := range altFQDNMap {
		fdns = append(fdns, fqdnList...)

		tlsIngress := networking.IngressTLS{
			Hosts:      fqdnList,
			SecretName: certificateName,
		}

		tlsList = append(tlsList, tlsIngress)
	}

	return &networking.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        objectName,
			Namespace:   namespace,
			Annotations: annotations,
			Labels:      util.K8sCesServiceDiscoveryLabels,
		},
		Spec: networking.IngressSpec{
			IngressClassName: &i.ingressClassName,
			TLS:              tlsList,
			Rules:            createIngressRules(fdns),
		},
	}
}

func (i IngressRedirector) upsertIngress(ctx context.Context, ingress *networking.Ingress) error {
	_, cErr := i.ingressInterface.Create(ctx, ingress, metav1.CreateOptions{})
	if cErr == nil {
		return nil
	}

	if !apierrors.IsAlreadyExists(cErr) {
		return fmt.Errorf("failed to create redirect ingress: %w", cErr)
	}

	_, uErr := i.ingressInterface.Update(ctx, ingress, metav1.UpdateOptions{})
	if uErr != nil {
		return fmt.Errorf("failed to update redirect ingress: %w", uErr)
	}

	return nil
}

func createIngressRules(hostList []string) []networking.IngressRule {
	pathTypePrefix := redirectPathType
	rules := make([]networking.IngressRule, 0, len(hostList))

	for _, host := range hostList {
		ingressRule := networking.IngressRule{
			Host: host,
			IngressRuleValue: networking.IngressRuleValue{
				HTTP: &networking.HTTPIngressRuleValue{
					Paths: []networking.HTTPIngressPath{
						{
							Path:     redirectIngressPath,
							PathType: &pathTypePrefix,
							Backend: networking.IngressBackend{
								Service: &networking.IngressServiceBackend{
									Name: redirectEndpointName,
									Port: networking.ServiceBackendPort{
										Number: redirectEndpointPort,
									},
								},
								Resource: nil,
							},
						},
					},
				},
			},
		}

		rules = append(rules, ingressRule)
	}

	return rules
}

// groupFQDNsBySecretName maps a list of alternative FQDNs with a secret name to a map of secret names to a list of FQDNs.
// *
// Result example:
//
//	{
//	  "secret1": ["fqdn1", "fqdn2"],
//	  "secret2": ["fqdn3"]
//	}
func groupFQDNsBySecretName(altFQDNList []types.AlternativeFQDN) map[string][]string {
	result := make(map[string][]string)

	for _, altFQDN := range altFQDNList {
		result[altFQDN.CertificateSecretName] = append(result[altFQDN.CertificateSecretName], altFQDN.FQDN)
	}

	return result
}
