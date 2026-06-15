# Exposition Custom Resource

Eine Exposition (`exp`) ist eine Kubernetes-Custom-Resource, die definiert, wie ein Service von außerhalb des Clusters erreichbar gemacht wird.
Sie unterstützt HTTP-Routen (Layer 7) sowie rohe TCP- und UDP-Ports (Layer 4).
Für jede Exposition erstellt der Operator die notwendigen Ingress-Objekte und weist für TCP/UDP-Einträge einen Port am LoadBalancer-Service des Clusters zu.

## HTTP-Routen (`spec.http`)

Jeder Eintrag in `spec.http` macht einen Kubernetes-Service unter einem bestimmten URL-Pfad erreichbar.

- `name` — eindeutiger Bezeichner dieser Route innerhalb der Exposition (Kleinbuchstaben, Ziffern und Bindestriche, z. B. `ui`)
- `service` — Name des Kubernetes-Services, an den der Traffic weitergeleitet wird
- `port` — Port-Nummer am Ziel-Service
- `path` — URL-Pfad, unter dem die Anwendung erreichbar ist; muss mit `/` beginnen
- `rewrite.stripPrefix` *(optional)* — Präfix, der vor der Weiterleitung an den Service vom Anfragepfad entfernt wird
- `rewrite.regex.pattern` *(optional)* — Regulärer Ausdruck, der auf den Anfragepfad angewendet wird
- `rewrite.regex.replacement` *(optional)* — Ersetzungszeichenkette für den gefundenen Treffer (Capture-Gruppen über `$1`, `$2`, …); erforderlich, wenn `pattern` gesetzt ist

Pro Route sollte nur eines von `stripPrefix` oder `regex` gesetzt werden.

## TCP-Routen (`spec.tcp`)

Jeder Eintrag in `spec.tcp` macht einen Kubernetes-Service über einen rohen TCP-Port am LoadBalancer erreichbar.

- `name` — eindeutiger Bezeichner dieser Route (Kleinbuchstaben, Ziffern und Bindestriche)
- `service` — Name des Ziel-Kubernetes-Services
- `port` — Port-Nummer am Ziel-Service
- `requestedExternalPort` *(optional)* — gewünschte externe Port-Nummer
- `protocol` — Freitext-Protokollhinweis für Dokumentations- oder Firewall-Zwecke (z. B. `"ssh"`, `"ldap"`)

## UDP-Routen (`spec.udp`)

`spec.udp` hat dieselbe Struktur wie `spec.tcp`.

## Status

Nach der Reconciliation aktualisiert der Operator `status.conditions`:

- `Valid` — ob die Exposition-Spezifikation akzeptiert und erfolgreich verarbeitet wurde
- `IngressesReady` — ob die HTTP-Ingress-Objekte erstellt wurden; der Reason spiegelt den aktuellen Zustand wider (`Created`, `DoguStopped`, `DoguStarting`, `MaintenanceMode`)
- `IngressTCPRoutesCreated` — ob die TCP-IngressRoute-Objekte erstellt wurden
- `IngressUDPRoutesCreated` — ob die UDP-IngressRoute-Objekte erstellt wurden
- `LoadBalancerPortsAllocated` — ob alle angeforderten TCP/UDP-Ports am LoadBalancer erfolgreich zugewiesen wurden

Das Feld `status.allocatedPorts` listet den tatsächlich zugewiesenen externen Port für jeden TCP- und UDP-Eintrag nach der Zuweisung auf.

## Lifecycle-Verhalten

Der Operator passt das HTTP-Routing automatisch an den Zustand des zugehörigen Dogus und den globalen Wartungsmodus an:

- **Dogu startet** — HTTP-Routen liefern vorübergehend eine Startseite, bis der Dogu bereit ist
- **Dogu gestoppt** — HTTP-Routen sind deaktiviert; kein Traffic wird an die Anwendung weitergeleitet
- **Wartungsmodus aktiv** — alle HTTP-Routen aller Expositionen liefern die globale Wartungsseite; dieser Zustand überschreibt den Dogu-Zustand
- **TCP- und UDP-Routen** sind vom Dogu-Lifecycle und vom Wartungsmodus nicht betroffen

Alle vom Operator erstellten Ressourcen (Ingress-Objekte, Middleware, IngressRouteTCP, IngressRouteUDP) gehören der Exposition-CR.
Das Löschen einer Exposition entfernt automatisch alle von ihr erstellten Ressourcen.

## Port-Konflikte

Wenn zwei oder mehr Exposition-CRs denselben externen Port und dasselbe Protokoll (TCP oder UDP) anfordern, werden alle betroffenen Expositionen vom LoadBalancer ausgeschlossen, bis der Konflikt aufgelöst ist.
Die Condition `LoadBalancerPortsAllocated` jeder betroffenen Exposition wird auf `False` gesetzt mit Reason `PortCollision` und einer Nachricht, die die konfliktierenden Port/Protokoll-Kombinationen auflistet.

## Beispiele

### Minimale HTTP-Exposition

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

### HTTP-Exposition mit Pfad-Rewrite

Das folgende Beispiel entfernt das Präfix `/myapp` vor der Weiterleitung an den Service, sodass eine Anfrage an `/myapp/api/v1` den Service unter `/api/v1` erreicht.

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

### TCP-Exposition mit explizitem externem Port

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

### Kombinierte HTTP- und TCP-Exposition

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
