# Terraform Provider for Nutanix

> **Development status:** the M0 foundation is in progress. No Terraform resources, data sources, or actions are implemented. This repository is not ready for provider use.

This project targets a greenfield Terraform provider for the Nutanix Cloud Platform. The M0 foundation establishes a handwritten modular monolith built with Terraform Plugin Framework and intended to use Terraform Plugin Protocol 6. It does not use a Nutanix SDK as a runtime dependency or generate implementation code from OpenAPI.

| Property | Value |
| --- | --- |
| Go module | `github.com/ioplane/terraform-provider-nutanix` |
| Terraform Registry address | `ioplane/nutanix` |
| Provider type | `nutanix` |
| Protocol | Terraform Plugin Protocol 6 |

## Foundation workflow

Use the repository launcher for development. It bootstraps the exact locked
Python environment through `uv` 0.12.1; the host does not need the repository's
Go, Terraform, Task, Beads, or Python tools installed.

```text
./dev up
./dev status
./dev task versions
./dev task python:test
./dev beads --version
./dev shell
./dev down
```

`./dev task` and `./dev beads` preserve every following argument as a direct
container argument. Build, test, lint, generation, packaging, Terraform, Task,
Beads, and Python quality work run inside Podman. The launcher derives a stable
Compose project name per Git worktree, mounts that worktree at `/workspace`,
and mounts its Git common directory at `/git-common`. Explicit `GIT_DIR`,
`GIT_COMMON_DIR`, and `GIT_WORK_TREE` values keep Git commands such as
`git status` functional in linked worktrees.

The development container does not receive the Podman socket. Lifecycle,
readiness, status, and noninteractive commands are bounded host operations;
only `./dev shell` uses an interactive host `podman exec`. GitHub tokens are
removed from ordinary container operations and are injected only for
`./dev beads dolt push` or `./dev beads dolt pull`, with output redaction.

## Project documents

- [Approved foundation design](docs/superpowers/specs/2026-08-04-foundation-design.md)
- [Approved foundation implementation plan](docs/superpowers/plans/2026-08-04-foundation.md)
- [Provider architecture](docs/architecture.md)
- [Terraform provider contract](docs/contract.md)
- [Go 1.26 engineering standard](docs/standards/go-1.26.md)
- [Naming standard](docs/standards/naming.md)
- [Nutanix artifact standard](docs/standards/nutanix-artifacts.md)
- [Testing standard](docs/standards/testing.md)
- [ADR 0001: Modular Monolith](docs/adr/0001-modular-monolith.md)
- [ADR 0002: Hand-Written Nutanix Client](docs/adr/0002-hand-written-nutanix-client.md)
- [ADR 0003: Podman Development Boundary](docs/adr/0003-podman-development-boundary.md)
- [ADR 0004: Beads Tracker](docs/adr/0004-beads-tracker.md)
- [ADR 0005: Nutanix Artifact Lock](docs/adr/0005-nutanix-artifact-lock.md)
- [Contribution guide](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## License

Licensed under the [Apache License 2.0](LICENSE).
