# Terraform Provider Nutanix Foundation Design

**Status:** approved in conversation on 2026-08-04; written specification
awaiting repository review

## 1. Purpose

Create `ioplane/terraform-provider-nutanix` as a new Terraform provider for the
Nutanix Cloud Platform. The provider is greenfield code: it does not fork or
reuse implementation code from `nutanix/terraform-provider-nutanix`.

The foundation sprint establishes the repository, governance, local tracking,
containerized development environment, automation, CI gate, and an empty
Terraform Plugin Framework provider served over protocol 6. Product resources
and live Nutanix mutations are intentionally outside this sprint.

## 2. Repository identity

| Property | Decision |
| --- | --- |
| GitHub repository | `ioplane/terraform-provider-nutanix` |
| Visibility | public |
| Licence | Apache-2.0 |
| Go module | `github.com/ioplane/terraform-provider-nutanix` |
| Terraform Registry address | `ioplane/nutanix` |
| Provider type name | `nutanix` |
| Default branch | `main` |
| Merge policy | pull request, squash only, delete branch after merge |

The repository is not a vendor fork. The official provider is evidence for
compatibility and state migration, not a code base or module dependency.

## 3. Product architecture

The provider is one binary implemented as a modular monolith:

```text
cmd/terraform-provider-nutanix
internal/provider
internal/core
internal/api/<control-plane-or-namespace>
internal/service/<domain>/<terraform-type>
internal/testserver
```

Dependency direction is:

```text
cmd -> provider -> service -> api -> core
```

Rules:

- Terraform schemas and lifecycle logic live in `internal/service`.
- API packages do not import Terraform Plugin Framework.
- Shared transport, authentication, retry, task polling, ETag handling,
  pagination, diagnostics, and capability discovery live in `internal/core`.
- Production transport and DTOs are hand-written.
- Nutanix SDKs are not runtime dependencies.
- OpenAPI snapshots are pinned evidence and test input, not generators of
  public Terraform schemas.
- Existing downstream type names, nested schema, import grammar, remote
  identity, and state upgrade paths are compatibility contracts.

The foundation sprint registers no resource, data source, action, function, or
ephemeral resource. It proves only provider startup, schema, configuration
validation, protocol 6 negotiation, packaging, and delivery controls.

## 4. Terraform baseline

The foundation pins:

- Go 1.26.5 from the approved `golang:1.26-trixie` image;
- `terraform-plugin-framework v1.19.0`;
- `terraform-plugin-testing v1.16.0`;
- `terraform-plugin-docs v0.25.0`;
- Terraform CLI 1.15.8;
- Terraform Plugin Protocol 6.

The entrypoint uses `providerserver.Serve` and explicitly selects protocol 6.
The provider implements current Framework `Metadata`, `Schema`, `Configure`,
`Resources`, and `DataSources` contracts. Actions are not registered in M0.

Dependency pins are exact. `go.sum` is committed. Upgrades are isolated
`build(deps)` pull requests with the full gate.

## 5. Delivery methodology

Use gated-iterative delivery:

- macro phases have explicit entry and exit gates;
- implementation is divided into two-week sprints;
- one sprint uses one branch, one worktree, and one pull request;
- `main` remains releasable;
- every factual completion claim includes command or runtime evidence;
- status changes occur when evidence exists, never retrospectively.

Roles are separate responsibilities even when one person performs several:

| Role | Responsibility |
| --- | --- |
| PM | scope, priorities, phase gates, risk, release decisions |
| ARC | architecture, provider identity, Terraform schema/state contract, ADRs |
| DEV | implementation, unit tests, generated documentation |
| QA | independent tests, negative cases, drift and acceptance evidence |
| OPS | Podman environment, pins, CI, packaging and repository controls |

DEV does not approve its own schema contract. A Terraform type cannot enter an
implementation sprint until ARC has signed its schema, state, identity, import,
and test contract. The author of a lifecycle implementation does not provide
the only acceptance review for it.

### 5.1 Macro phases

| Phase | Outcome |
| --- | --- |
| M0 Foundation | repository, standards, Beads, Podman environment, automation, CI, empty protocol 6 provider |
| M1 Kernel | config, TLS/auth, diagnostics, transport, retry, pagination, ETag, task polling, capabilities |
| M2 Read-only canary | cluster/category/image/subnet/role/operation reads and import canary |
| M3 Foundation resources | categories, projects, subnets, storage containers/policies, image placement |
| M4 Compute and block storage | VM, volume groups and affinity |
| M5 IAM | directory, users/groups, roles, policies and user keys |
| M6 Objects compatibility | official create-only Object Store lifecycle |
| M7 Compatibility release | full current 20-resource/10-data-source downstream surface and state fixtures |
| M8 Segmented Objects | separate draft plus precheck/deploy actions |
| M9 Product expansion | supported stable v4 product namespaces |
| M10 External planes | Foundation, NDB, Self-Service, NC2, NKP, NDK and NAI adapters |

M0 does not claim downstream compatibility. Real legacy state inventory and
`UpgradeState` work remain M7 gates.

## 6. Canonical local tracker: Beads

Use Beads `v1.1.2` as the canonical task and dependency tracker. Markdown is
not authoritative task state.

Why Beads:

- dependency graph and automatic ready queue;
- structured JSON output for automation;
- atomic claim transition;
- hierarchical epics and tasks;
- appendable audit history;
- local embedded Dolt database with synchronization under `refs/dolt/data`;
- no 1,000-line manually maintained status document.

The alternatives were rejected:

- a living `docs/plan.md` is easy to start but becomes unqueryable narrative;
- a custom JSON registry would reimplement dependency, locking, ready queue,
  merge, and history semantics.

### 6.1 Beads operating contract

- The `bd` binary is installed only in the development image.
- The release archive and checksum are pinned for supported architectures.
- Initialization uses embedded Dolt and `--skip-agents`; repository-owned
  `AGENTS.md` remains authoritative.
- The Git origin is also the Dolt remote. Cross-checkout synchronization uses
  `bd dolt pull`, `bd dolt push`, and `bd bootstrap`.
- `.beads/config.yaml` and the generated JSONL export are committed for
  discovery and review. Dolt remains the source of truth.
- `docs/roadmap.md` is generated from `bd --json`; it is a human view, not a
  writable tracker.
- Exactly one critical-path task may be `in_progress`.
- Auxiliary tasks may run only when they are dependency-disjoint and do not
  alter the same files or external state.
- A task is closed only after its acceptance evidence is attached.
- Every branch, commit, and pull request references its Beads task ID.

The initial graph contains one epic per macro phase. `M0-FOUNDATION` is the
only current critical-path task. Later phases remain blocked by their explicit
predecessor gates.

## 7. Host and container boundary

The host is a control plane, not a development environment.

Allowed on the host:

- Git and GitHub operations;
- Podman engine access;
- `uv` for the pinned bootstrap environment;
- `podman-compose` to create or remove the development container;
- `podman-py` to inspect readiness and execute a command in the container.

Required inside the development container:

- all Go commands;
- Terraform/OpenTofu/Terragrunt commands;
- Task;
- Beads;
- Python lint, type check, and tests;
- documentation generation and linting;
- security and dependency checks;
- packaging and release dry-runs.

No host Go, Terraform, Task, linter, generator, or test runner is used as
completion evidence.

### 7.1 Operator interface

A repository launcher named `dev` is the only host entrypoint:

```text
./dev up
./dev down
./dev status
./dev shell
./dev task <task> [arguments]
./dev beads <bd arguments>
```

The launcher runs a locked Python environment. It invokes `podman-compose`
with argument arrays and bounded deadlines, then uses `PodmanClient.from_env()`
as a context manager for readiness, inspect, and exec. Typed Podman API errors
are reported without parsing CLI output. Diagnostic status may fall back to
the Podman CLI when the REST socket is unavailable; mutating operations do not
silently fall back.

The Podman socket is not mounted inside the development container. This avoids
granting the tool container control over unrelated host containers.

## 8. OCI development environment

### 8.1 Pinned baseline

```text
docker.io/library/golang:1.26-trixie
sha256:4ee9ffa999b4583ce281939cdff828763083610292f252279a0cee77473bd9a7
```

The image reports `GOLANG_VERSION=1.26.5`.

Other foundation pins include:

- podman-py 5.8.0;
- podman-compose 1.6.0;
- Task 3.52.0;
- Beads 1.1.2;
- golangci-lint 2.12.2;
- GoReleaser 2.17.1.

Every image uses a registry-qualified digest. GitHub Actions use full commit
SHAs. Python dependencies use exact versions and a committed `uv.lock`.

### 8.2 Compose topology

M0 has one Compose service, `dev`, built from
`deployments/containers/Containerfile.dev`.

- The checkout is mounted at `/workspace:z`.
- Go module and build caches are named volumes.
- The container runs as a toolbox with `sleep infinity`.
- A health check verifies the toolbox process.
- The Compose project and container name include a stable worktree suffix.
- Parallel worktrees never share a container or writable build cache namespace.
- The Compose file follows the living Compose Specification and has no
  top-level `version` key.

The image carries OCI source, revision, version, created, licence, title,
description, base name, and base digest annotations. Values that depend on the
build are supplied as build arguments.

## 9. Automation borrowed and corrected from PowerDNS

Reuse the following patterns, adapted rather than copied blindly:

- `Taskfile.yml` is the in-container command index.
- Dev-container and worktree preconditions fail with an actionable message.
- Worktree suffixes isolate Compose projects.
- Subprocesses run without a shell, in a new process group, with bounded
  deadlines that kill descendants.
- The Containerfile is the canonical tool-version registry.
- Automated checks compare CI version markers with Containerfile arguments.
- Checks prove image digests and GitHub Action pins resolve.
- Python automation is covered by ruff, ty, and pytest.
- `task all` is the no-live-system pre-PR gate.
- `task verify` extends `all` with the strongest available integration test.
- Generated provider docs are checked for drift.
- Package tests install the exact ZIP/checksum used by consumers.

Corrections relative to the PowerDNS repository:

- host Task is not required; the launcher enters the pinned container;
- podman-compose is locked by `uv.lock`, not whatever happens to be installed;
- tracker state is a dependency graph, not a manually expanded plan;
- no automation target contains unconditional recursive deletion;
- M0 does not create a fake lab before there is a bounded Nutanix acceptance
  design.

## 10. Repository files in M0

```text
AGENTS.md
CODEX.md
CLAUDE.md
README.md
CHANGELOG.md
CONTRIBUTING.md
SECURITY.md
VERSION
Taskfile.yml
dev
go.mod
go.sum
main.go
internal/provider/provider.go
internal/provider/provider_test.go
deployments/containers/Containerfile.dev
deployments/compose/compose.dev.yml
scripts/automation/dev.py
scripts/automation/run.py
scripts/checks/pins.py
scripts/checks/tool_versions.py
test/scripts/
pyproject.toml
uv.lock
.github/workflows/ci.yml
docs/adr/
docs/standards/
docs/superpowers/specs/
docs/superpowers/plans/
.beads/config.yaml
.beads/issues.jsonl
```

Pointer files `CODEX.md` and `CLAUDE.md` direct readers to `AGENTS.md`; no
tool-specific contract is allowed to drift from it.

## 11. Quality and test contract

M0 gates:

| Gate | Required proof |
| --- | --- |
| Bootstrap | `./dev up`, health status, and `./dev shell` work from a clean checkout |
| Versions | `./dev task versions` reports exact pinned tools |
| Build | provider binary builds in the dev container |
| Protocol | Terraform loads the provider through protocol 6 |
| Unit | provider metadata/schema/config tests pass with race detection |
| Python | ruff, ty, and pytest pass inside the dev container |
| OCI | Containerfile lint, labels, base digest, and Compose validation pass |
| Pins | all container digests and GitHub Action SHAs resolve |
| Tracker | graph has one current critical task and no dependency cycle |
| Docs | Markdown/YAML lint and internal links pass |
| Package | deterministic ZIP and checksum install into a temporary filesystem mirror |
| CI | GitHub Actions runs the same containerized gates |

`task all` contains every non-live-system gate. `task verify` is equal to
`task all` in M0 and gains live acceptance only when M1 defines an isolated,
non-production fixture.

## 12. GitHub delivery controls

The repository uses:

- issues and projects enabled;
- wiki disabled;
- squash merge only;
- merge commits and rebase merges disabled;
- automatic source-branch deletion;
- pull-request review workflow;
- branch protection after the first CI workflow has produced its named checks;
- required CI checks and conversation resolution;
- no direct push to protected `main` after bootstrap;
- Dependabot for Go modules, Python dependencies, containers, and Actions.

The GitHub repository is created before the first sprint branch. Initial
README/licence creation is the only bootstrap commit made by GitHub directly on
`main`. Foundation implementation lands through `sprint/m0-foundation`.

## 13. Security boundaries

- No Nutanix credentials, endpoints, state, fixtures containing secrets, or
  environment identifiers are committed.
- TLS verification defaults on. Insecure mode is explicit and warned later.
- No production PE, PC, or state is used in M0.
- Provider logs and diagnostics cannot contain provider configuration values.
- The Podman socket remains on the host control plane.
- Dependency, container, and Action references are immutable.
- Release work eventually emits checksums, SBOMs, and provenance; M0 proves the
  packaging shape without publishing a release.

## 14. Foundation exit criteria

M0 is complete only when:

1. the public repository identity and settings match this specification;
2. Beads is initialized, synchronized to the Git origin, and has exactly one
   active critical-path task;
3. a clean checkout can bootstrap through `./dev up` without host Go or Task;
4. every build, test, linter, generator, and package check runs in Podman;
5. the empty provider completes a protocol 6 handshake with Terraform 1.15.8;
6. the local full gate passes from the packaged development image;
7. GitHub Actions passes the same gate from the sprint commit;
8. a pull request contains the evidence and has no unresolved blocking review;
9. branch protection requires the actual CI check names after merge;
10. the generated roadmap and Beads graph agree.

No Nutanix resource implementation, live mutation, downstream module change,
provider source migration, or Terraform state change is part of M0.
