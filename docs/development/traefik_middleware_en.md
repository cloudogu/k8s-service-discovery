# Traefik Middlewares

With the switch from nginx to traefik, some functions of k8s-service-discovery were migrated to traefik middlewares.

##  Static Rewrites
If a Dogu is installed but not healthy, the "Dogu is starting" page is displayed. For this purpose, a [middleware](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/replacepath/) is created,
which replaces the path with the path of a static page in k8s-ces-assets.
When maintenance mode is enabled, middleware is created that performs a rewrite to a static page in k8s-ces-assets.

The use of these middlewares is defined by an annotation on the respective ingress.
```
traefik.ingress.kubernetes.io/router.middlewares: ecosystem-dogu-starting@kubernetescrd
```
or
```
traefik.ingress.kubernetes.io/router.middlewares: maintenance-mode@kubernetescrd
```

## Exposed Ports
Some Dogus require certain ports that must be accessible from the outside via Traefik, e.g., SCM. For this purpose, an ``IngressRouteTCP`` or an ``IngressRouteUDP`` is created dynamically
depending on the desired technology. Unfortunately, when starting
Traefik, all ports that may be exposed must be specified, because although Traefik can dynamically create the above-mentioned
resources, it cannot then release these ports.
In ``k8s-ces-gateway``, these ports are statically exposed in values.yaml.

## Dogu Rewrites
Individual Dogus require static rewrites, e.g., the Nexus Docker Repository. For this purpose, middleware is dynamically created during the installation
of the Dogu, which performs a ``Replace Path Rewrite`` for the service of a Dogu. This middleware
is created if the service of a Dogu has defined additional ``ces-services ``. 

## Alternative FQDNs
Alternative FQDNs for the ecosystem can be defined in global-config. If this configuration exists,
a ``Redirect`` middleware is dynamically created, which redirects from the alternative FQDNs to the primary FQDN using a regex.