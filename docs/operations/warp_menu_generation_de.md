# Generierung der Einträge des Warp-Menüs

Die `k8s-service-discovery` ist zuständig für die Generierung der `menu.json` des Warp-Menüs.
Dazu implementiert sie, ähnlich wie es `ces-confd` gemacht hat, einen Watch auf bestimmte Pfade im Etcd.
Bei einer Änderung z.B. einer Dogu-Installation generiert die `k8s-service-discovery` neue Einträge
und schreibt diese in die Configmap `k8s-ces-menu-json`. Diese Configmap wird von dem `nginx-ingress`
eingebunden und verwendet.

## Konfiguration

### Quellen

Es ist möglich 3 Arten von Quellen für den Etcd-Watch anzugeben.

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
  - path: /config/nginx/externals
    type: externals
```

Externe Links müssen folgender Struktur (JSON-String) im Etcd entsprechen:

```
{
  "cloudogu": "{
  \"DisplayName\": \"Cloudogu\",
  \"Description\": \"Beschreibungstext für Cloudogu Webseite\",
  \"Category\": \"External Links\",
  \"URL\": \"https://www.cloudogu.com/\"
}"
}
```

#### Einträge, die ausgeblendet werden sollen
```yaml
sources:
  - path: /config/_global/disabled_warpmenu_support_entries
    type: disabled_support_entries
```

Die Konfiguration beinhaltet außerdem Support-Links, die jedoch mit dem Typ `disabled_support_entries` ausgeblendet
werden können. Dazu muss im angegebenen Pfad ein String-Array im JSON-Format abgelegt werden. Die Einträge entsprechen
den Keys der Links. Zum Beispiel:
`'["docsCloudoguComUrl", "aboutCloudoguToken"]'`

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
  - path: /config/nginx/externals
    type: externals
  - path: /config/_global/disabled_warpmenu_support_entries
    type: disabled_support_entries
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
  - identifier: myCloudogu
    external: true
    href: https://my.cloudogu.com/
```