# Maintenance Mode

This document explains the maintenance mode and how to control it for the Cloudogu EcoSystem MultiNode.

Maintenance mode is a system state of the Ecosystem where external access to the EcoSystem is disabled. The mode is
required when system critical processes are running. While maintenance mode is activated, a maintenance page is
returned for each access to a Dogus.

# Activate Maintenance Mode

Maintenance mode is controlled by the ConfigMap `maintenance` in the namespace of the EcoSystem:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: maintenance
  namespace: ecosystem
data:
  active: "true"
  holder: "k8s-backup-operator"
  text: "Backup in progress"
  title: "Service temporary unavailable"
```

* `active`: maintenance mode is active as soon as this value is `"true"`. Any other value or a missing ConfigMap means
  maintenance mode is inactive.
* `holder`: the component that activated maintenance mode. Another component refuses to activate or deactivate
  maintenance mode while it is held by someone else (unless it forces the change).
* `title` and `text` are displayed on the maintenance page.

Every request to a Dogu is then answered with the maintenance page until `active` is set to `"false"` or the ConfigMap
is deleted.

# How it works

The service discovery watches the `maintenance` ConfigMap. When maintenance mode is activated, it rewrites the HTTP
routes of all Ingresses to the static content backend (`k8s-ces-assets`) and adds the Traefik middleware annotation

```
traefik.ingress.kubernetes.io/router.middlewares: <namespace>-maintenance-mode@kubernetescrd
```

to them. The middleware `maintenance-mode` replaces the request path with the path of the maintenance page
(`/errors/503.html`). When maintenance mode is deactivated, the Ingresses are restored to the services of the Dogus.

**Note:** Enabling and disabling maintenance mode only changes Ingresses and middlewares. No Dogu is restarted, and
the change takes effect as soon as Traefik has picked up the changed Ingresses.
