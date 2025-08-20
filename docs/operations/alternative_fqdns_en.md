# Configuration of alternative FQDNs

In addition to the primary FQDN, alternative FQDNs can be configured via the global configuration.
If a user calls such an FQDN, they will be redirected to the primary FQDN.

These FQDNs are specified in the global configuration under the key `alternativeFQDNs` with a comma-separated list of entries.
Optionally, a separate TLS certificate can be referenced for each FQDN via its name. For this purpose, the FQDN entry and the TLS certificate are separated by a `:`.
If no TLS certificate is specified, the default certificate of the instance is used.

The following points should be considered when configuring alternative FQDNs:

- Each entry is a valid hostname (without schema/port), e.g., `alt.example.com`
- The referenced TLS certificate is a Kubernetes secret of type `kubernetes.io/tls` and is located in the same namespace
- Spaces around entries are tolerated (e.g., after the comma)
- Do not use wildcards (*.example.com) unless explicitly supported
- Avoid duplicate FQDNs
- Secret must contain the keys `tls.crt` and `tls.key`
- Always enclose strings in quotation marks so that commas are correctly interpreted as separators

## Example

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

In the example above, the alternative FQDNs `bmf.example.com,k008.example.com,alt.example.com` are redirected to the primary FQDN `cloudogu.example.com`.
The FQDNs `bmf.example.com,k008.example.com` use the instance's default certificate, while `alt.example.com` uses the certificate `new-certificate`.