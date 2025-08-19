package util

import "strings"

type AlternativeFQDN struct {
	FQDN                  string
	CertificateSecretName string
}

func (a AlternativeFQDN) HasCertificate() bool {
	return a.CertificateSecretName != ""
}

func ParseAlternativeFQDNsFromConfigString(configString string) []AlternativeFQDN {
	altFQDNs := make([]AlternativeFQDN, 0)

	altFQDNCerts := strings.Split(configString, ",")
	for _, altFQDNCert := range altFQDNCerts {
		fqdnCertTupel := strings.Split(altFQDNCert, ":")
		if len(fqdnCertTupel) == 2 && fqdnCertTupel[0] != "" {
			altFQDNs = append(altFQDNs, AlternativeFQDN{strings.TrimSpace(fqdnCertTupel[0]), strings.TrimSpace(fqdnCertTupel[1])})
		} else if len(fqdnCertTupel) == 1 && fqdnCertTupel[0] != "" {
			altFQDNs = append(altFQDNs, AlternativeFQDN{strings.TrimSpace(fqdnCertTupel[0]), ""})
		}
	}

	return altFQDNs
}
