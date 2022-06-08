# Generation of the entries of the warp menu

The `k8s-service-discovery` is responsible for generating the `menu.json` of the warp menu.
For this it implements, similar to what `ces-confd` did, a watch on certain paths in the etcd.
When changing e.g. a Dogu installation the `k8s-service-discovery` generates new entries
and writes them to the configmap `k8s-ces-menu-json`. This configmap is included and used by the `nginx-ingress`.

## Configuration

### Sources

It is possible to specify 3 types of sources for the etcd-watch.

#### Dogus
```yaml
sources:
- path: /dogu
  type: dogus
  tag: warp
```

#### External links
```yaml
sources:
  - path: /config/nginx/externals
    type: externals
```

External links must match the following structure (JSON-String) in the etcd:

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

#### Entries to be hidden
```yaml
sources:
  - path: /config/_global/disabled_warpmenu_support_entries
    type: disabled_support_entries
```

The configuration also includes support links, but these can be hidden with the `disabled_support_entries` type.
To do this, a string array in JSON format must be stored in the specified path. The entries correspond to
the keys of the links. For example:
`'["docsCloudoguComUrl", "aboutCloudoguToken"]'`

### Order
The `order` category can be used to sort the specific Dogu categories from the `dogu.json` in the warp menu.
A higher value will be displayed higher up in the warp menu.

### Support
Support links represent fixed links that are displayed in the lower part of the warp menu.

```yaml
support:
- identifier: docsCloudoguComUrl
  external: true
  href: https://docs.cloudogu.com/
```

### Default configuration
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
