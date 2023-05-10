# Verwendung eines selbst signierten SSL Zertifikats

## Ablage

Das SSL-Zertifikat befindet sich in der Registry unter den folgenden Pfaden:
- `config/_global/certificate/key`
- `config/_global/certificate/server.crt`
- `config/_global/certificate/server.key`

## Ein selbst-signiertes SSL-Zertifikat erneuern

Das `k8s-ces-setup` konfiguriert initial das Zertifikat für das Cloudogu Ecosystem.
Die `k8s-service-discovery` bietet einen Endpunkt an, um das selbst-signiertes Zertifikat zu erneuern:

```bash
curl -I --request POST --url http://fqdn:9090/api/v1/ssl?days=<days> 
```

> Wenn die FQDN geändert wird und ein selbst-signiertes SSL-Zertifikat verwendet wird, wird dieses automatisch neu generiert und angewendet. 
> Bei FQDN-Änderungen müssen zusätzlich auch die Dogus neu gestartet werden, damit sie diese Änderung erhalten.