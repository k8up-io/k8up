# Backup as a service

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [unreleased]
### Added
- Vscode launch config
- Read credentials and password from OpenShift secrets
- Specifiy default Prometheus push gateway for the operator
- Read the Prometheus push gateway url from the CRD, has precedence over the operator settings
### Changed
- Use alpine for operator image
- Moved the Restic wrapper to its own project: https://git.vshn.net/vshn/wrestic

## [v0.0.1]
### Changed
- Initial version

[unreleased]: https://git.vshn.net/vshn/baas/compare/v0.0.1...master
[v0.0.1]: https://git.vshn.net/vshn/baas/tree/v0.0.1
