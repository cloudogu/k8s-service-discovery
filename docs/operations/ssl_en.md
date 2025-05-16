# Use of a Self-Signed SSL ertificate

The SSL certificate is expected in a secret as follows:
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

The k8s-service-discovery reconciles this secret and writes the public key to the global config as follows:
- `certificate/server.crt`

The secret with the certificate is therefore leading in relation to the global config.

The type of certificate can be found in the global config:
- `certificate/type`

If the FQDN is changed and a self-signed SSL certificate is used, this is automatically regenerated and applied.
In the case of FQDN changes, the Dogus must also be restarted so that they receive this change.