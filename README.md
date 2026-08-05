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
./dev beads init --skip-agents
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

Beads is anchored explicitly at `BEADS_DIR=/workspace/.beads`. This keeps the
embedded Dolt database local to the mounted worktree instead of letting Beads
derive a fallback database below `/git-common` when that mount does not have a
`.git` basename. Every `./dev beads` execution receives the anchor directly,
and `./dev shell` inherits it from the Compose service. Database subtrees under
`.beads/` remain ignored; only `.beads/config.yaml` and
`.beads/issues.jsonl` are tracked projections. The launcher does not discover,
migrate, or remove a database created at an earlier fallback location.
The Git common-directory bind remains available at `/git-common`, but Compose
masks its `.beads` child with an empty mode-`0700` tmpfs. This makes an ambient
host fallback database invisible to the pinned Beads process while leaving the
host data untouched. The mount uses `notmpcopyup`; without it, Podman would
populate the new tmpfs from the masked host directory. The only usable tracker
state in the toolbox is the worktree-owned `/workspace/.beads`; no fallback
database is deleted or migrated.
The launcher entrypoint first resolves the worktree with exactly three bounded,
token-scrubbed Git queries. After that mandatory discovery, it requires a real
`.beads/` directory and a regular, non-symlink `.beads/config.yaml`; otherwise
it rejects every Beads command before credential lookup, connector entry,
Beads exec, or Podman API use. The sole exception is a command whose first
Beads argument is exactly `init`. That exception lets the pinned binary create
the worktree-local database using the explicit anchor; after initialization,
all Beads commands retain the same exact environment.

The development container does not receive the Podman socket. Lifecycle,
readiness, status, and noninteractive commands are bounded host operations;
only `./dev shell` uses an interactive host `podman exec`. GitHub tokens are
removed from every ordinary host subprocess and container operation. Known
token values are redacted from captured output and are injected only for
`./dev beads dolt push` or `./dev beads dolt pull`.

Every noninteractive container command runs under the pinned GNU `timeout`
binary without a shell. Task commands have a 60-minute limit, ordinary Beads
commands 5 minutes, remote authentication setup 1 minute, and remote Beads
commands 10 minutes. Expiry sends `TERM` to the command process group, then
`KILL` after 2 seconds; the resulting exit code, stdout, and stderr are
preserved.

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
