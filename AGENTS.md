# K8up - Kubernetes Backup Operator

K8up is a CNCF sandbox Kubernetes backup operator based on [Restic](https://restic.readthedocs.io).
Built with [Kubebuilder](https://github.com/kubernetes-sigs/kubebuilder), it handles PVC and application backups on Kubernetes/OpenShift clusters.

- Project website: [k8up.io](https://k8up.io/)
- Documentation: [docs.k8up.io](https://docs.k8up.io/)

## Project Structure

```
api/v1/          # CRD type definitions (Backup, Restore, Schedule, Check, Prune, Archive, Snapshot, PreBackupPod, PodConfig)
operator/        # Kubernetes operator controllers and reconcilers
  *controller/   # Per-resource controllers (backup, restore, schedule, check, prune, archive)
  executor/      # Job execution logic
  job/           # Kubernetes Job creation
  scheduler/     # Cron scheduling
  monitoring/    # Prometheus metrics
restic/          # Restic CLI wrapper (backup, restore, check, prune operations)
cli/             # CLI restore functionality
cmd/             # Entrypoints (k8up binary, operator subcommand, restic subcommand)
common/          # Shared utilities
config/          # Kubernetes manifests and CRD definitions
  crd/           # Generated CRD YAMLs
  samples/       # Example CR configurations
charts/k8up/     # Helm chart (templates, CRDs, values)
docs/            # AsciiDoc documentation (Antora)
e2e/             # End-to-end tests using BATS/DETIK and KIND
envtest/         # Integration tests using envtest
```

## Language & Build

- **Go** project (see `go.mod` for version)
- Build: `make build` (binary output: `./k8up`)
- Main entrypoint: `cmd/k8up/main.go`
- Linter: `golangci-lint` (config: `.golangci.yml`)
- Formatting: `gofmt` with tabs — always run `go fmt ./...` before committing

## Key Commands

```
make test              # Run unit tests
make build             # Build binary (runs generate, fmt, vet first)
make generate          # Generate CRD code (deepcopy, manifests)
make run-operator      # Run operator locally against ~/.kube/config
make run-restic        # Run restic module locally
make install           # Install CRDs into KIND cluster
make deploy            # Deploy operator to KIND cluster
make e2e-test          # Run E2E tests (requires node/npm, uses KIND + BATS/DETIK)
make docs-preview      # Build and preview Antora docs
make chart-docs        # Generate Helm chart documentation
```

## Code Conventions

- CRD types live in `api/v1/` — after modifying, run `make generate`
- Controller logic follows the Kubebuilder reconciler pattern
- Each CRD resource has its own controller package under `operator/`
- Tabs for indentation in Go, Makefiles, and bats/bash files
- Spaces (2) for YAML, JSON, CSS, JS (see `.editorconfig`)

## PR and Contribution Rules

- **Never mix code changes with chart changes** in the same PR — this breaks the release process
- Code PRs must have the `area:operator` label
- Chart PRs must have the `area:chart` label and the chart label (e.g., `chart:k8up`)
- Commits must be [signed off](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits) (DCO)
- PRs should be labeled with one of: `bug`, `enhancement`, `documentation`, `change`, `breaking`, `dependency`
- Helm chart values must be documented in the format required by [helm-docs](https://github.com/norwoodj/helm-docs#valuesyaml-metadata)
- If changing chart, bump Chart version if immediate release is planned, and run `make chart-docs`
- When closing issues as already-implemented, add documentation if the feature wasn't documented
- Always reference related issues in PR descriptions (`Fixes #N` or `Relates-to: #N`)
- Always review `git diff` before committing — verify only intended changes are staged
- Create PRs as draft when changes should be double-checked by the user before review

## Release

- Releases use GoReleaser (`.goreleaser.yml`)
- Multi-arch builds: linux/darwin/windows, amd64/arm64
- Container images published to `ghcr.io/k8up-io/k8up` and `quay.io/k8up-io/k8up`
- Helm chart index is published on the `gh-pages` branch (GitHub Pages)

## Dependencies

- Renovate manages dependency updates (config: `renovate.json`)
- Renovate PRs also need area labels (`area:operator` or `area:chart`)
- Dependency PRs get the `dependency` label automatically

## Testing

- **Use test-driven development**: write a failing test first, then implement the fix or feature
- **Bug fixes must include a test** that reproduces the bug before fixing it
- **Unit tests**: `make test` (standard `go test ./...` with coverage)
- **Integration tests**: envtest-based, in `envtest/` directory
- **E2E tests**: BATS/DETIK framework, runs against KIND cluster, in `e2e/` directory
  - Requires `node` and `npm`
  - Run specific test: `make e2e-test -e BATS_FILES=test-02-deployment.bats`

## Documentation

- Documentation source lives in `docs/` as AsciiDoc files, structured for [Antora](https://antora.org/)
- Pushing to this repo triggers a webhook to a VSHN GitLab pipeline that builds and deploys to [docs.k8up.io](https://docs.k8up.io/)
- K8up shares the same Antora build pipeline as other VSHN documentation repos
- Preview docs locally with `make docs-preview`
- Example YAML files in `docs/modules/ROOT/examples/` are included via `include::` directives — keep them valid and up to date

## Community

- CNCF Slack: `#k8up` channel in [CNCF workspace](https://slack.cncf.io/)
- Roadmap: [GitHub Projects](https://github.com/k8up-io/k8up/projects/2)
