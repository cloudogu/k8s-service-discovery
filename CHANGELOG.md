# k8s-service-discovery Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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