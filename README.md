# Terraform Provider for Nutanix

> **Development status:** the M0 foundation and M1 hand-written kernel are complete; the M2 read-only Nutanix product corpus is active. Provider configuration, authentication, TLS, origin-bound HTTP, bounded response/error handling, redaction, structured logging, request IDs, fail-closed retry, pagination, guarded ETags, Prism task handling, and the capability registry are implemented. No runtime capability probe, Terraform resource, data source, or action is registered yet. Product tests follow the corresponding product implementation. This repository is not ready for provider use.

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
./dev task all
./dev task verify
./dev beads init --skip-agents
./dev beads --version
./dev shell
./dev down
```

`all` is the lightweight implementation gate. It validates repository safety
and the locked official API artifacts, checks the role-oriented Python CLI and
Go sources with formatting/static/security tools, verifies documentation and
tracker consistency, and builds the provider. It does not run Python tooling
tests, Go unit tests, fuzzing, race tests, Protocol acceptance, or package
acceptance. Product tests are added and run only after the corresponding
Nutanix product corpus is implemented. `verify` is an exact alias for `all`.

`./dev task` and `./dev beads` preserve every following argument as a direct
container argument. Build, test, lint, generation, packaging, Terraform, Task,
Beads, and Python quality work run inside Podman. The launcher derives a stable
Compose project name per Git worktree, mounts that worktree at `/workspace`,
and mounts its Git common directory at `/git-metadata/.git`. Explicit
`GIT_DIR`, `GIT_COMMON_DIR`, and `GIT_WORK_TREE` values keep Git commands such
as `git status` functional in linked worktrees.

Beads is anchored explicitly at `BEADS_DIR=/workspace/.beads`. Every
`./dev beads` execution receives the anchor directly, and `./dev shell`
inherits it from the Compose service. Mounting the Git common directory with a
real `.git` basename makes Beads 1.1.2 derive any automatic fallback as the
sibling `/git-metadata/.beads`, which is container-local and not host-backed.
An existing host fallback remains physically reachable through the required
Git bind at `/git-metadata/.git/.beads`, but the pinned Beads resolver and the
launcher never select it as canonical state or mutate it automatically. No
nested mount is created, and no fallback database is deleted or migrated.
Database subtrees under `.beads/` remain ignored; only `.beads/config.yaml`
and `.beads/issues.jsonl` are tracked projections.
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
`./dev beads bootstrap`, `./dev beads dolt push`, or `./dev beads dolt pull`.
Bootstrap remains behind the fail-closed worktree marker guard described
above; only exact first-argument `init` bypasses that guard. For those three
remote Beads operations only, the launcher runs `bd` through the
exact argument-array prefix `/usr/bin/env -u GIT_COMMON_DIR -u GIT_DIR -u
GIT_WORK_TREE`. This prevents the container's explicit Git worktree variables
from conflicting with Beads' internal Git subprocesses. The exec still
receives only `BEADS_DIR` plus the short-lived `GH_TOKEN`; the preceding
`gh auth setup-git` exec remains token-only. No shell is involved, and ordinary
Beads, Task, and interactive-shell commands retain their existing environment.
Pinned Beads 1.1.2 rewrites an existing `sync.remote` entry without a final LF
during an executing bootstrap. The launcher first records, read-only, whether
the regular source config ended in LF. Only if that canonical source later
survives a successful execution-mode bootstrap does it run the repository
normalizer inside the same toolbox with only `BEADS_DIR`. Help, version, and
dry-run invocations never normalize; a source already lacking LF remains
visible as drift. The normalizer appends one LF only when missing and refuses
empty, non-regular, or symlinked configs. This keeps a tracked project config
byte-stable without exposing credentials or hiding other Beads changes.

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
- [Go dependency policy](docs/standards/dependencies.md)
- [ADR 0001: Modular Monolith](docs/adr/0001-modular-monolith.md)
- [ADR 0002: Hand-Written Nutanix Client](docs/adr/0002-hand-written-nutanix-client.md)
- [ADR 0003: Podman Development Boundary](docs/adr/0003-podman-development-boundary.md)
- [ADR 0004: Beads Tracker](docs/adr/0004-beads-tracker.md)
- [ADR 0005: Nutanix Artifact Lock](docs/adr/0005-nutanix-artifact-lock.md)
- [ADR 0006: M1 Hand-Written Kernel Contract](docs/adr/0006-m1-kernel-contract.md)
- [M1 kernel design](docs/superpowers/specs/2026-08-05-m1-kernel-design.md)
- [M1 kernel implementation plan](docs/superpowers/plans/2026-08-05-m1-kernel.md)
- [Contribution guide](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## License

Licensed under the [Apache License 2.0](LICENSE).
