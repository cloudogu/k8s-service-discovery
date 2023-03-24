# Use of a Self-Signed SSL Certificate

## Location

The SSL certificate is located in the registry under the following paths:
- `config/_global/certificate/key`
- `config/_global/certificate/server.crt`
- `config/_global/certificate/server.key`

## Renew an SSL certificate

The `k8s-ces-setup` initially creates the certificate for the Cloudogu Ecosystem.
The `k8s-service-discovery` provides an endpoint to renew the selfsigned certificate:

```bash
curl -I --request POST --url http://fqdn:9090/api/v1/ssl?days=<days> 
```
