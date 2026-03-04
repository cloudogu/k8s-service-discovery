# Traefik Middlewares

Mit der Umstellung von nginx auf traefik wurden einige Funktionen der k8s-service-dicovery auf traefik Middlewares umgestellt.

##  Statische Rewrites
Wenn ein Dogu installiert aber nicht healthy ist, wird die ``Dogu is starting``-Seite angezeigt. Dafür wird eine [Middleware](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/replacepath/) erstellt,
die einen den Pfad durch den Pfad einer statischen Seite in k8s-ces-assets ersetzt.
Wenn der Maintenance-Modus aktiviert ist, wird eine Middleware erstellt, die einen Rewrite auf eine statische Seite in k8s-ces-assets durchführt.

Die Verwendung dieser Middlewares wird durch eine Annotation am jeweiligen Ingress definiert.
```
traefik.ingress.kubernetes.io/router.middlewares: ecosystem-dogu-starting@kubernetescrd
```
oder
```
traefik.ingress.kubernetes.io/router.middlewares: maintenance-mode@kubernetescrd
```

## Exposed Ports
Einige Dogus benötigen bestimmte Ports, die über Traefik nach außen erreichbar sein müssen, z.B. SCM. Für wird dynamisch
je nach gewünschter Technologie eine ``IngressRouteTCP`` oder eine ``IngressRouteUDP`` erstellt. Leider müssen beim Start
von Traefik bereits alle Ports, die eventuell exposed werden können angegeben werden, da Traefik zwar dynamisch die o.g.
Ressourcen erstellen kann, diese Ports dann aber nicht freigeben kann. 
Im ``k8s-ces-gateway`` werden diese Ports statisch in der values.yaml freigegeben. 

## Dogu Rewrites
Einzelne Dogus benötigen statische Rewrites, z.B. das Nexus Docker Repository. Dafür wird dynamisch bei der Installation
des Dogus eine Middleware erstellt, die für den Service eines Dogus einen ``Replace Path Rewrite`` durchführt. Diese [Middleware](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/replacepathregex/)
wird erstellt, wenn der Service eines Dogus zusätzliche ``ces-services `` definiert hat. 

## Alternative FQDNs
In der global-config können alternative FQDNs für das Ecosystem definiert werden. Wenn diese Konfiguration vorhanden ist, 
wird dynamisch eine ``Redirect`` [Middleware](https://doc.traefik.io/traefik/reference/routing-configuration/http/middlewares/redirectregex/) erstellt, die anhand von einer Regex von den alternativen FQDNs auf die primäre FQDN umleitet.