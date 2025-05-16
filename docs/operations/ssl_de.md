# Verwendung eines selbst signierten SSL Zertifikats


Das SSL-Zertifikat wird von der k8s-service-discovery generiert und in dem Secret `ecosystem-certificate` wie folgt abgelegt:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ecosystem-certificate
  namespace: ecosystem
type: Opaque
data:
  tls.crt: <public_key>
  tls.key: <private_key>
```

Das Secret mit dem Zertifikat ist somit führend gegenüber der globalen Config.

Die k8s-service-discovery reconciled dieses Secret und schreibt den Public-Key wie folgt in die globale Config:
- `certificate/server.crt`

> Wenn die FQDN geändert wird und ein selbst-signiertes SSL-Zertifikat verwendet wird, wird dieses automatisch neu generiert und angewendet. 
> Bei FQDN-Änderungen müssen zusätzlich auch die Dogus neu gestartet werden, damit sie diese Änderung erhalten.