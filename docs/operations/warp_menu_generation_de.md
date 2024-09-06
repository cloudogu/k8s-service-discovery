# Generierung der Einträge des Warp-Menüs

Die `k8s-service-discovery` ist zuständig für die Generierung der `menu.json` des Warp-Menüs.
Dazu implementiert sie, ähnlich wie es `ces-confd` gemacht hat, einen Watch auf bestimmte Pfade in der globalen Konfiguration und lokalen Dogu-Registry.
Bei einer Änderung z.B. einer Dogu-Installation generiert die `k8s-service-discovery` neue Einträge
und schreibt diese in die Configmap `k8s-ces-menu-json`. Diese Configmap wird von dem `nginx-ingress`
eingebunden und verwendet.

## Konfiguration

### Quellen

Es ist möglich 3 Arten von Quellen für den Watch anzugeben.

#### Dogus
```yaml
sources:
  - path: /dogu
    type: dogus
    tag: warp
```

#### Externe Links
```yaml
sources:
  - path: externals
    type: externals
```

Externe Links müssen folgender Struktur (YAML-String) in der globalen Konfiguration entsprechen:

```yaml
cloudogu: |
  DisplayName: Cloudogu
  Description: Beschreibungstext für Cloudogu Webseite
  Category: External Links
  URL: https://www.cloudogu.com
```

#### Konfiguration für Support-Einträge in der globalen Konfiguration
Die Konfiguration der Support-Einträge erfolgt direkt in der globalen Konfiguration mithilfe der folgenden drei Schlüssel:
  - block_warpmenu_support_category
  - allowed_warpmenu_support_entries
  - disabled_warpmenu_support_entries

##### Alle Einträge ausblenden
Wenn alle Support-Einträge des warp-menu nicht angezeigt werden sollen, kann dies über die globale Konfiguration `block_warpmenu_support_category` konfiguriert werden.
```shell
# alle Einträge ausblenden
kubectl edit configmap global-config --namespace ecosystem
```
Edit:
```yaml
data:
  config.yaml:
    block_warpmenu_support_category: "true"
```
# keine Einträge ausblenden
```shell
# alle Einträge ausblenden
kubectl edit configmap global-config --namespace ecosystem
```
Edit:
```yaml
data:
  config.yaml:
    block_warpmenu_support_category: "false"
```

##### Nur einzelne Einträge anzeigen
Wenn alle Support-Einträge des warp-menu ausgeblendet sind, aber trotzen einzelne davon angezeigt werden sollen, kann dies über die globale Konfiguration `allowed_warpmenu_support_entries` konfiguriert werden.
Dort muss ein JSON-Array, mit den anzuzeigenden Einträgen angegeben werden.
```yaml
allowed_warpmenu_support_entries: '["platform", "aboutCloudoguToken"]'
```

> Diese Konfiguration ist nur wirksam, wenn **alle** Einträge ausgeblendet sind (siehe [oben](#alle-einträge-ausblenden)).

##### Einzelne Einträge ausblenden
Wenn einzelne Einträge im warp-menu nicht gerendert werden sollen, kann dies über die globale Konfiguration `disabled_warpmenu_support_entries` konfiguriert werden.
Dort muss ein JSON-Array, mit den auszublendenden Einträgen angegeben werden.

```yaml
disabled_warpmenu_support_entries: '["docsCloudoguComUrl", "aboutCloudoguToken"]'
```

> Diese Konfiguration ist nur wirksam, wenn **nicht** alle Einträge ausgeblendet sind (siehe [oben](#alle-einträge-ausblenden)).

### Order
Mit der Kategorie `order` lassen sich die bestimmten Dogu-Kategorien aus der `dogu.json` im Warp-Menü sortieren.
Ein höherer Wert wird im Warp-Menü weiter oben angezeigt.

### Support
Support Links stellen feste Links, welche im unteren Teil des Warp-Menüs angezeigt werden, dar.

```yaml
support:
  - identifier: docsCloudoguComUrl
    external: true
    href: https://docs.cloudogu.com/
```

### Standardkonfiguration
```yaml
sources:
  - path: /dogu
    type: dogus
    tag: warp
  - path: externals
    type: externals
target: /var/www/html/warp/menu.json
order:
  Development Apps: 100
support:
  - identifier: docsCloudoguComUrl
    external: true
    href: https://docs.cloudogu.com/
  - identifier: aboutCloudoguToken
    external: false
    href: /info/about
  - identifier: platform
    external: true
    href: https://platform.cloudogu.com
```