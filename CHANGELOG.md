# k8s-service-discovery Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v1.2.0] - 2024-12-04
### Changed
- [#67] Minimize RBAC permissions for the operator
  - Separate roles into own files
  - Restrict permissions for roles as much as possible
  - Delete leader-election-role

## [v1.1.0] - 2024-10-28
### Changed
- [#65] Make imagePullSecrets configurable via helm values and use `ces-container-registries` as default.

## [v1.0.0] - 2024-10-18
### Changed
- [#63] Use dogu v2 api

## [v0.15.2] - 2024-09-19
### Changed
- Relicense to AGPL-3.0-only

## [v0.15.1] - 2024-09-06
### Added
- [#58] Use new config interface (configmaps instead of the etcd is now used) to request global configuration.
- [#56] Use new registry interface (configmaps instead of the etcd is now used) to request and watch dogu jsons.
- [#53] New configuration (`/config/_global/block_warpmenu_support_category`) for completely blocking the support entries in the warp menu
- [#53] New configuration (`/config/_global/allowed_warpmenu_support_entries`) for explicitly allowing support entries in the warp menu

### Fixed
- [#53] Create warp menu directly at startup to prevent an empty warp menu

## [v0.15.0] - 2023-12-08
### Added
- [#49] Patch-template for mirroring this component and its images into airgapped environments.
### Changed
- [#50] Remove kustomize and hold the operator yaml files in a single helm chart.

## [v0.14.4] - 2023-10-24
### Changed
- [#46] Update cesapp-lib to 0.12.2

## [v0.14.3] - 2023-10-02
### Fixed
- [#44] Fix a bug where the service discovery only updated one single ingress switching maintenance mode.

## [v0.14.2] - 2023-09-20
### Changed
- [#38] updated go dependencies
- [#38] updated kube-rbac-proxy

### Fixed
- [#38] deprecation warning for argument `logtostderr` in kube-rbac-proxy

### Removed
- [#38] deprecated argument `logtostderr` from kube-rbac-proxy

## [v0.14.1] - 2023-09-15
### Fixed
- [#42] Set default-value for STAGE environment-variable to "production"

## [v0.14.0] - 2023-09-15
### Changed
- [#39] Move component-dependencies to helm-annotations

## [v0.13.1] - 2023-08-31
### Fixed
- [#34] Add label `app: ces` for all generated Kubernetes resources

### Added
- [#36] Add "k8s-etcd" as a dependency to the helm-chart

## [v0.13.0] - 2023-07-07
### Added
- [#32] Add Helm chart release process to project

## [v0.12.1] - 2023-06-01
### Fixed
- [#30] Add appropriate labels to generated ingress resources

## [v0.12.0] - 2023-05-10
### Added
- [#26] Support for service rewrite mechanism
- [#28] Support automatic regeneration of selfsigned certificates on FQDN-change

## [v0.11.0] - 2023-04-06
### Added
- [#24] Apply additional ingress annotations from dogu service to ingress object.

## [v0.10.0] - 2023-03-24
### Added
- [#22] Add ssl api to renew the selfsigned certificate of the Cloudogu Ecosystem.

### Changed
- Update makefiles to 7.5.0

## [v0.9.0] - 2023-02-10
### Changed
- [#17] add `Accept-Encoding: "identity"` header to requests proxied by nginx-ingress

## [v0.8.0] - 2023-01-11
### Changed
- [#16] add/update label for consistent mass deletion of CES K8s resources
  - select any k8s-service-discovery related resources like this: `kubectl get deploy,pod,... -l app=ces,app.kubernetes.io/name=k8s-service-discovery`
  - select all CES components like this: `kubectl get deploy,pod,... -l app=ces`

## [v0.7.0] - 2022-12-05
### Changed
- [#14] Write important events on dogu resources
- Update RBAC permissions to apply only a minimum set of privileges 

## [v0.6.0] - 2022-11-15
### Added
- [#12] All dogus that are not ready are created with a "Dogu is starting"-page ingress object. The ingress object is 
  automatically updated after the dogu becomes ready.

## [v0.5.0] - 2022-08-30
### Added
- [#10] Support for maintenance mode. See [maintenance mode](docs/operations/maintenance_mode_en.md) for more details.

### Changed
- [#10] Update `ces-build-lib` to version `1.56.0`
- [#10] Update `makefiles` to version `7.0.1`

## [v0.4.0] - 2022-08-29
### Added
- [#8] Add implementation for general logger used in the cesapp-lib

## [v0.3.0] - 2022-06-09
### Added
- [#5] The certificate for the ingress-nginx will be automatically updated.
A watch recognizes changes in the registry for the certificate and updates the ssl secret .

## [v0.2.0] - 2022-06-08
### Added
- [#3] Warp menu generation
  - Add runnable to the controller which observes keys in the etcd specified in a configmap `k8s-ces-warp-config`
  and creates warp menu entries in `k8s-ces-menu-json` for the nginx-ingress dogu.

### Changed
- [#3] Update makefiles to version 6.0.1 

## [v0.1.0] - 2022-04-20
### Added
- Automatically creates the ingress class `k8s-ecosystem-ces-service` in the current namespace.
- Parses the annotation `k8s-dogu-operator.cloudogu.com/ces-services` for every created service 
and automatically creates respective ingress objects for each CES-service.