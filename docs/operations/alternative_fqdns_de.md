# Konfiguration alternativer FQDNs

Über die globale Konfiguration können neben der primären FQDN alternative FQDNs konfiguriert werden.
Ruft ein Nutzer solch eine FQDN auf, wird er auf die primäre FQDN umgeleitet.

Diese FQDNs werden in der globalen Konfiguration unter dem Schlüssel `alternativeFQDNs` mit einer kommagetrennten Liste von Einträgen angegeben.
Optional kann für jede FQDN ein eigenes TLS-Zertifikat über dessen Namen referenziert werden. Hierfür wird der FQDN-Eintrag und das TLS-Zertifikat über ein `:` getrennt.
Wird kein TLS-Zertifikat angegeben, wird das Standard-Zertifikat der Instanz verwendet.

Für die Konfiguration der alternativen FQDNs sollten folgende Punkte beachtet werden:

- jeder Eintrag ist ein gültiger Hostname (ohne Schema/Port), z. B. `alt.example.com`
- das referenzierte TLS-Zertifikat ist ein Kubernetes-Secret vom Typ `kubernetes.io/tls` und befindet sich im gleichen Namespace
- Leerzeichen rund um Einträge werden toleriert (z. B. nach dem Komma)
- keine Wildcards (*.example.com) verwenden, sofern nicht explizit unterstützt
- doppelte FQDNs vermeiden
- Secret muss die Schlüssel `tls.crt` und `tls.key` enthalten
- Strings immer in Anführungszeichen setzen, damit Kommas korrekt als Trennzeichen interpretiert werden

## Beispiel

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app: ces
    k8s.cloudogu.com/type: global-config
  name: global-config
  namespace: ecosystem
data:
  config.yaml: |
    alternativeFQDNs: "bmf.example.com,k008.example.com,alt.example.com:new-certificate",
    fqdn: "cloudogu.example.com"
```

Im genannten Beispiel werden die alternativen FQDNs `bmf.example.com,k008.example.com,alt.example.com` auf die primäre FQDN `cloudogu.example.com` umgeleitet.
Die FQDNs `bmf.example.com,k008.example.com` nutzen das Standard-Zertifikat der Instanz, während `alt.example.com` das Zertifikat `new-certificate` nutzt.