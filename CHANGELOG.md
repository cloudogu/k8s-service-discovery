# k8s-service-discovery Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- The certificate for the ingress-nginx will be automatically updated.
A watch recognizes changes in the registry for the certificate and updates the ssl secret [#5].

## [v0.2.0] - 2022-06-08
### Added
- Warp menu generation
  - Add runnable to the controller which observes keys in the etcd specified in a configmap `k8s-ces-warp-config`
  and creates warp menu entries in `k8s-ces-menu-json` for the nginx-ingress dogu [#3].

### Changed
- Update makefiles to version 6.0.1 [#3]

## [v0.1.0] - 2022-04-20
### Added
- Automatically creates the ingress class `k8s-ecosystem-ces-service` in the current namespace.
- Parses the annotation `k8s-dogu-operator.cloudogu.com/ces-services` for every created service 
and automatically creates respective ingress objects for each CES-service.