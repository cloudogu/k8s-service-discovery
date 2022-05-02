# k8s-service-discovery Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Changed
- Update makefiles to version 5.1.0

## [v0.1.0] - 2022-04-20
### Added
- Automatically creates the ingress class `k8s-ecosystem-ces-service` in the current namespace.
- Parses the annotation `k8s-dogu-operator.cloudogu.com/ces-services` for every created service 
and automatically creates respective ingress objects for each CES-service.