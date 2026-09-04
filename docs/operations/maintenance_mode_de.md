# Wartungsmodus

Dieses Dokument erklärt den Wartungsmodus und wie man diesen für das Cloudogu EcoSystem MultiNode steuern kann.

Der Wartungsmodus ist ein Systemzustand des Ecosystem, bei dem ein externer Zugriff auf das EcoSystem deaktiviert wird.
Der Modus wird benötigt, wenn systemkritische Prozesse laufen. Während der Wartungsmodus aktiviert ist, wird für jeden
Zugriff auf Dogus eine Wartungsseite angezeigt.

# Wartungsmodus aktivieren

Der Wartungsmodus wird über die ConfigMap `maintenance` im Namespace des EcoSystems gesteuert:

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

* `active`: Der Wartungsmodus ist aktiv, sobald dieser Wert `"true"` ist. Jeder andere Wert oder eine fehlende ConfigMap
  bedeutet, dass der Wartungsmodus inaktiv ist.
* `holder`: Die Komponente, die den Wartungsmodus aktiviert hat. Eine andere Komponente verweigert das Aktivieren bzw.
  Deaktivieren, solange der Wartungsmodus von einer anderen Komponente gehalten wird (es sei denn, sie erzwingt die
  Änderung).
* `title` und `text` werden auf der Wartungsseite angezeigt.

Jede Anfrage an ein Dogu wird dann mit der Wartungsseite beantwortet, bis `active` auf `"false"` gesetzt oder die
ConfigMap gelöscht wird.

# Funktionsweise

Die Service-Discovery überwacht die ConfigMap `maintenance`. Wird der Wartungsmodus aktiviert, schreibt sie die
HTTP-Routen aller Ingresses auf das Static-Content-Backend (`k8s-ces-assets`) um und ergänzt die Traefik-Middleware-
Annotation

```
traefik.ingress.kubernetes.io/router.middlewares: <namespace>-maintenance-mode@kubernetescrd
```

an diesen. Die Middleware `maintenance-mode` ersetzt den Anfragepfad durch den Pfad der Wartungsseite
(`/errors/503.html`). Wird der Wartungsmodus deaktiviert, werden die Ingresses wieder auf die Services der Dogus
zurückgesetzt.

**Hinweis:** Das Aktivieren und Deaktivieren des Wartungsmodus ändert lediglich Ingresses und Middlewares. Es wird kein
Dogu neu gestartet, und die Änderung wird wirksam, sobald Traefik die geänderten Ingresses übernommen hat.
