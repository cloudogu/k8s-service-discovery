# Exposition Custom Resource

An Exposition (`exp`) is a Kubernetes custom resource that defines how a service is made reachable from outside the cluster.
It supports HTTP (Layer 7) routes as well as raw TCP and UDP (Layer 4) ports.
For each Exposition the operator creates the necessary ingress objects and, for TCP/UDP entries, allocates a port on the cluster's LoadBalancer Service.

## HTTP Routes (`spec.http`)

Each entry in `spec.http` exposes a Kubernetes Service at a specific URL path.

- `name` — unique identifier for this route within the Exposition (lowercase alphanumeric and hyphens, e.g. `ui`)
- `service` — name of the Kubernetes Service to route traffic to
- `port` — port number on the target Service
- `path` — URL path under which the application is reachable; must start with `/`
- `rewrite.stripPrefix` *(optional)* — prefix to strip from the request path before forwarding to the Service
- `rewrite.regex.pattern` *(optional)* — regular expression to match against the request path
- `rewrite.regex.replacement` *(optional)* — replacement string for the matched pattern (capture groups via `$1`, `$2`, …); required when `pattern` is set

Only one of `stripPrefix` or `regex` should be set per route.

## TCP Routes (`spec.tcp`)

Each entry in `spec.tcp` exposes a Kubernetes Service on a raw TCP port on the LoadBalancer.

- `name` — unique identifier for this route (lowercase alphanumeric and hyphens)
- `service` — name of the target Kubernetes Service
- `port` — port number on the target Service
- `requestedExternalPort` *(optional)* — desired external port number
- `protocol` *(optional)* — free-text protocol hint for documentation or firewall rules (e.g. `"ssh"`, `"ldap"`)

## UDP Routes (`spec.udp`)

`spec.udp` has the same structure as `spec.tcp`.

## Status

After reconciliation the operator updates `status.conditions`:

- `Valid` — whether the Exposition spec was accepted and processed successfully
- `IngressesReady` — whether the HTTP ingress objects were created; the condition reason reflects the current state (`Created`, `DoguStopped`, `DoguStarting`, `MaintenanceMode`)
- `IngressTCPRoutesCreated` — whether the TCP IngressRoute objects were created
- `IngressUDPRoutesCreated` — whether the UDP IngressRoute objects were created
- `LoadBalancerPortsAllocated` — whether all requested TCP/UDP ports were successfully allocated on the LoadBalancer

The field `status.allocatedPorts` lists the actually assigned external port for each TCP and UDP entry after allocation.

## Lifecycle Behaviour

The operator adjusts HTTP routing automatically based on the state of the associated Dogu and the global maintenance mode:

- **Dogu is starting** — HTTP routes temporarily serve a splash page until the Dogu becomes healthy
- **Dogu is stopped** — HTTP routes are suspended; no traffic is forwarded to the application
- **Maintenance mode active** — all HTTP routes across all Expositions serve the global maintenance page; this overrides the Dogu state
- **TCP and UDP routes** are not affected by Dogu lifecycle or maintenance mode

All resources created by the operator (Ingress objects, Middleware, IngressRouteTCP, IngressRouteUDP) are owned by the Exposition CR.
Deleting an Exposition automatically removes all resources it created.

## Port Conflicts

If two or more Exposition CRs request the same external port and protocol (TCP or UDP), all conflicting Expositions are excluded from the LoadBalancer until the conflict is resolved.
Each affected Exposition's `LoadBalancerPortsAllocated` condition is set to `False` with reason `PortCollision` and a message listing the conflicting port/protocol combinations.

## Examples

### Minimal HTTP exposition

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: Exposition
metadata:
  name: myapp
  namespace: ecosystem
spec:
  http:
    - name: ui
      service: myapp
      port: 8080
      path: /myapp
```

### HTTP exposition with path rewrite

The following example strips the `/myapp` prefix before forwarding the request to the Service, so a request to `/myapp/api/v1` reaches the Service at `/api/v1`.

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: Exposition
metadata:
  name: myapp
  namespace: ecosystem
spec:
  http:
    - name: ui
      service: myapp
      port: 8080
      path: /myapp
      rewrite:
        stripPrefix: /myapp
```

### TCP exposition with explicit external port

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: Exposition
metadata:
  name: ldap
  namespace: ecosystem
spec:
  tcp:
    - name: ldap
      service: ldap
      port: 389
      requestedExternalPort: 2389
      protocol: ldap
```

### Combined HTTP and TCP exposition

```yaml
apiVersion: k8s.cloudogu.com/v1
kind: Exposition
metadata:
  name: ldap
  namespace: ecosystem
spec:
  http:
    - name: ui
      service: ldap-ui
      port: 8080
      path: /ldap
      rewrite:
        stripPrefix: /ldap
  tcp:
    - name: ldap
      service: ldap
      port: 389
      requestedExternalPort: 2389
      protocol: ldap
```
