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

External links must match the following structure (YAML-String) in the configuration:

```yaml
cloudogu: |
  DisplayName: Cloudogu
  Description: Beschreibungstext für Cloudogu Webseite
  Category: External Links
  URL: https://www.cloudogu.com
```

#### Configuration of Support-Entries
```yaml
sources:
  - path: /config/_global/block_warpmenu_support_category
    type: support_entry_config
  - path: /config/_global/allowed_warpmenu_support_entries
    type: support_entry_config
  - path: /config/_global/disabled_warpmenu_support_entries
    type: support_entry_config
```

##### Hide all entries
If all support entries of the warp-menu are not to be displayed, this can be configured via the etcd key `/config/_global/block_warpmenu_support_category`.
```shell
# hide all entries
etcdctl set /config/_global/block_warpmenu_support_category true

# do not hide any entries
etcdctl set /config/_global/block_warpmenu_support_category false
```

##### Show only individual entries
If all support entries of the warp-menu are hidden, but individual entries should still be displayed, this can be configured via the etcd key `/config/_global/allowed_warpmenu_support_entries`.
A JSON array with the entries to be displayed must be specified there.

```shell
etcdctl set /config/_global/allowed_warpmenu_support_entries '["platform", "aboutCloudoguToken"]'
```

> This configuration is only effective if **all** entries are hidden (see [above](#hide-all-entries)).

##### Hide individual entries
If individual entries in the warp-menu are not to be rendered, this can be configured via the etcd key `/config/_global/disabled_warpmenu_support_entries`.
A JSON array with the entries to be hidden must be specified there.

```shell
etcdctl set /config/_global/disabled_warpmenu_support_entries '["docsCloudoguComUrl", "aboutCloudoguToken"]'
```

> This configuration is only effective if **not** all entries are hidden (see [above](#hide-all-entries)).

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
  - path: /config/_global/block_warpmenu_support_category
    type: support_entry_config
  - path: /config/_global/allowed_warpmenu_support_entries
    type: support_entry_config
  - path: /config/_global/disabled_warpmenu_support_entries
    type: support_entry_config
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
