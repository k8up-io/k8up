# Backup as a service

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Ability to set file extension for stdin jobs via annotation

## [v0.1.3] - 2018-12-09
### Changed
- Don't quote backup commands
- Observe jobs instead of pods
### Fixed
- Fix owner reference creation
- Choose the right target for archival jobs

## [v0.1.2] - 2018-11-08
### Changed
- Removed a lot of duplicated code
### Added
- Locking management, jobs that disturb each other shouldn't run concurrently anymore

## [v0.1.1] - 2018-11-02
### Fixed
- Cleanup of the CRDs


## [v0.1.0] - 2018-11-01
With this release it is possible to trigger every action on demand. But there's also a schedule CRD that can put everything on its own schedule. These changes are the plumbing for further improvements especially with the locking problem.

**ATTENTION**: All the CRD definitions have changed slightly. So this is not backwards compatible with the old CRDs.

Also some known issues in this version:
- No lock management yet, only the plumbing, so if there's a shared repository it can and will sometimes fail due to stale locks
- If a job still fails after it reached the back-off limit the operator doesn't notice it. That's due to the k8s job controller cleaning up the pod. This can be resolved by observing the job states.

### Added
- Archive function
- Check function
- Prune function
- Schedule CRD to schedule all functions
### Changed
- Group name of the API, it's now backup.appuio.ch
- New CRDs please consult manifest/examples for how to use them

## [v0.0.10] - 2018-09-28
### Added
- Pass arbitrary URL to wrestic, which is used to post additional backup information.
- When setting the backup annotation on PVC it will be backed up even if RWO
### Fixed
- Broken locking when using global backend

## [v0.0.9] - 2018-09-18
### Added
- Global S3 endpoint, S3 bucket and keepJobs. Using env vars, global default values can be specified. The defaults can be overwritten by a backup spec.

## [v0.0.8] - 2018-09-14
### Added
- Push images to Docker Hub
- Divide installation manifests into own files

## [v0.0.7] - 2018-09-13
### Added
- Global repo password. The operator can use a global password for Restic repositories for backups which don't specify an own password.

## [v0.0.6] - 2018-09-12
### Added
- Manage service accounts, roles and rolebindings in namespaces for backup commands. Please see the README chapter `Installation changes` about the new permissions necessary for the operator service account.
- Global cluster state. The operator now knows when a prune job is running to a shared repository and waits before starting the next backup/prune.
- Global S3 credentials. The operator can use global S3 credentials to be used with backups which don't specify own credentials.
### Fixed
- Ignore if the service account, roles or rilebindings already exist in the namespace.

## [v0.0.5] - 2018-08-15
### Added
- Consistent backups -> it is now possible to set backup commands on pods. This changes the prerequisites, please consult the README chapter `Installation changes` for more infos.
### Fixed
- Crash when a running job got deleted
- Skip backup if there are no backup commands and no PVCs

## [v0.0.4] - 2018-08-06
### Changes
- Skip backup if no suitable PVCs are found
- Minor refactoring
- When the backup definition is deleted it will also cleanup all jobs and pods

## [v0.0.3] - 2018-07-27
### Changed
- CRD status, start and end are now timestamps
- Print namespace wenn a backup is created

## [v0.0.2] - 2018-07-27
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

[unreleased]: https://git.vshn.net/vshn/baas/compare/v0.1.2...master
[v0.1.2]: https://git.vshn.net/vshn/baas/compare/v0.0.1.1...v0.1.2
[v0.1.1]: https://git.vshn.net/vshn/baas/compare/v0.0.1.0...v0.1.1
[v0.1.0]: https://git.vshn.net/vshn/baas/compare/v0.0.10...v0.1.0
[v0.0.10]: https://git.vshn.net/vshn/baas/compare/v0.0.9...v0.0.10
[v0.0.9]: https://git.vshn.net/vshn/baas/compare/v0.0.8...v0.0.9
[v0.0.8]: https://git.vshn.net/vshn/baas/compare/v0.0.7...v0.0.8
[v0.0.7]: https://git.vshn.net/vshn/baas/compare/v0.0.6...v0.0.7
[v0.0.6]: https://git.vshn.net/vshn/baas/compare/v0.0.5...v0.0.6
[v0.0.5]: https://git.vshn.net/vshn/baas/compare/v0.0.4...v0.0.5
[v0.0.4]: https://git.vshn.net/vshn/baas/compare/v0.0.3...v0.0.4
[v0.0.3]: https://git.vshn.net/vshn/baas/compare/v0.0.2...v0.0.3
[v0.0.2]: https://git.vshn.net/vshn/baas/compare/v0.0.1...v0.0.2
[v0.0.1]: https://git.vshn.net/vshn/baas/tree/v0.0.1
