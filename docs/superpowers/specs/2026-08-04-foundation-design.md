# Terraform Provider Nutanix Foundation Design

**Status:** approved in conversation on 2026-08-04; repository re-review passed

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
internal/service/<domain>/<terraform-type>
internal/nutanix/<api-namespace>
internal/transport
internal/auth
internal/task
internal/capability
internal/testserver
```

Dependency direction is:

```text
cmd -> provider -> service -> nutanix/<namespace> -> transport
```

Rules:

- Terraform schemas and lifecycle logic live in `internal/service`.
- Nutanix namespace and transport packages do not import Terraform Plugin
  Framework.
- Shared HTTP behavior lives in focused packages such as `transport`, `auth`,
  `task`, and `capability`; catch-all `api`, `core`, `common`, `types`, and
  `util` Go packages are forbidden.
- Production transport and DTOs are hand-written.
- Nutanix SDKs are not runtime dependencies.
- OpenAPI snapshots are pinned evidence and test input, not generators of
  public Terraform schemas.
- Existing downstream type names, nested schema, import grammar, remote
  identity, and state upgrade paths are compatibility contracts.

The foundation sprint registers no resource, data source, action, function, or
ephemeral resource. It proves only provider startup, schema, configuration
validation, protocol 6 negotiation, packaging, and delivery controls.

### 3.1 Official Nutanix artifact contract

The Nutanix Developer Portal is the primary machine-readable API source. The
foundation consumes its public registry and artifact endpoints:

```text
GET https://developers.nutanix.com/api/v1/namespaces/
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/<version>/yaml
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/<version>/postman-collection
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/<version>/locale/en_US/error
```

The 2026-08-04 registry exposes 19 namespaces. There is no single global API
version: each namespace advances independently. Selection prefers the newest
GA version matching `v<major>.<minor>`. If a namespace has no GA release, its
newest preview may be selected only with an explicit `preview` stability flag.

M0 locks exactly one selected version for every namespace returned by the
registry, not a subset. The initial selection is:

| Namespace | Version | Stability |
| --- | --- | --- |
| `aiops` | `v4.0` | GA |
| `clustermgmt` | `v4.2` | GA |
| `datapolicies` | `v4.2` | GA |
| `dataprotection` | `v4.3` | GA |
| `files` | `v4.0` | GA |
| `iam` | `v4.0` | GA |
| `licensing` | `v4.3` | GA |
| `lifecycle` | `v4.2` | GA |
| `microseg` | `v4.2` | GA |
| `monitoring` | `v4.2` | GA |
| `multidomain` | `v4.3` | GA |
| `networking` | `v4.3` | GA |
| `objects` | `v4.0` | GA |
| `opsmgmt` | `v4.0` | GA |
| `prism` | `v4.3` | GA |
| `security` | `v4.1` | GA |
| `storage` | `v4.0.a3` | preview; no GA published |
| `vmm` | `v4.2` | GA |
| `volumes` | `v4.2` | GA |

`specs/nutanix/manifest.json` is the repository lock. For every selected
namespace it records the version, stability, OpenAPI URL, Postman URL when
published, English error-reference URL when published, and each artifact's
byte size and SHA-256 digest. The downloaded vendor artifacts live under the
repository-ignored `.cache/nutanix/artifacts/` tree. They are inputs to design,
contract tests, request fixtures, and drift reports; they are never fetched by
the provider at runtime. Vendor artifacts are not committed until their
redistribution terms have been reviewed separately.

Artifact precedence is:

1. selected GA OpenAPI document;
2. selected version's error reference;
3. selected version's Postman collection;
4. official SDK documentation and examples as comparison evidence only;
5. a live PE or PC observation only for an explicitly recorded documentation
   gap.

OpenAPI or Postman disagreements are recorded as contract risks and are not
silently resolved. SDKs are neither runtime dependencies nor code-generation
inputs. Terraform schemas, state models, lifecycle logic, transport, and DTOs
remain hand-written. Every product implementation task names the exact locked
namespace, version, operations, and schemas it uses.

Artifact updates are reviewable maintenance changes: `task artifacts:update`
refreshes the cache and proposed lock, while `task artifacts:verify` downloads
with GET, validates content type and OpenAPI/Postman shape, and checks every
locked OpenAPI, Postman, and error-reference digest. It also proves the locked
namespace set equals the live registry set and that GA-first selection is
correct. The update command never changes application code.

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

### 4.1 Go 1.26 engineering standard

The module declares `go 1.26.0`; the container supplies the exact patched
toolchain Go 1.26.5 and sets `GOTOOLCHAIN=local` so validation cannot download
or switch toolchains. Normal release builds use `CGO_ENABLED=0`. Race tests run
in a separate target with cgo and the container compiler enabled.

Repository rules follow the Go 1.26 release notes, Effective Go, Go Code
Review Comments, the official module-layout guide, and Go security guidance:

- production Go code stays under `cmd` and `internal`; no public library API is
  promised;
- packages are small, lower-case, single words with a concrete purpose;
- interfaces are declared by consumers at the point of use, not pre-created
  beside implementations for mocking;
- `context.Context` is the first argument for request-bound work, is propagated
  to every HTTP call, and is never stored in a struct;
- errors are handled once, wrapped with operation context and `%w`, and never
  used as unstructured control flow; panic is not normal error handling;
- goroutine ownership and termination are explicit; unbounded background work
  is forbidden;
- HTTP clients have explicit timeouts, close response bodies, redact secrets,
  and preserve cancellation;
- dependencies remain minimal and standard-library facilities are preferred.

Go naming is part of the contract:

- initialisms use canonical case: `API`, `HTTP`, `ID`, `JSON`, `TLS`, `URL`,
  `UUID`, and `ETag`, never `Api`, `Http`, `Id`, or `Url`;
- receivers use one consistent one- or two-letter abbreviation per type and
  never `this`, `self`, or `me`;
- exported declarations have complete doc comments beginning with the name;
- error strings start lower-case and have no terminal punctuation;
- sentinel errors use `ErrName` only when callers need identity; structured
  failures use `NameError`;
- Go source files use descriptive lower snake case, with `_test.go` for tests
  and `_acc_test.go` only for live acceptance tests.

Terraform names use `nutanix_<domain>_<noun>` in lower snake case. A public
type name does not contain an API version merely because its current transport
does; version suffixes exist only when they are required compatibility names.
Provider environment variables use the `NUTANIX_` prefix. Nutanix API version
and namespace names stay inside artifact, transport, and compatibility code.

The normal Go gate is formatting, `go vet ./...`, unit tests, race tests,
linting, and `govulncheck ./...`. Parsers, pagination, filter construction,
state upgrade, and remote error decoding gain native fuzz targets as they are
introduced; bounded fuzz smoke runs in CI and longer fuzzing runs on schedule.
Go 1.26 `go fix` is an explicit reviewed modernization tool, never an
automatic mutating CI step. Experimental `GOEXPERIMENT` features are excluded
from the supported build.

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
| M2 Read-only product | cluster/category/image/subnet/role/operation data sources |
| M3 Foundation resources | import/category reconciliation, categories, projects, subnets, storage containers/policies, image placement |
| M4 Compute and block storage | VM, volume groups and affinity |
| M5 IAM | directory, users/groups, roles, policies and user keys |
| M6 Objects compatibility | official create-only Object Store lifecycle |
| M7 Compatibility release | full current 20-resource/10-data-source downstream surface and state fixtures |
| M8 Segmented Objects | separate draft plus precheck/deploy actions |
| M9 Product expansion | supported stable v4 product namespaces |
| M10 External planes | Foundation, Foundation Central, NDB, Self-Service, NC2, NKP, NDK, NAI, Move, Beam, and Flow Security Central adapters; deprecated NKE compatibility boundary |

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

The immutable source anchors used for that comparison are:

- the predecessor patch line,
  [`dantte-lp/powerdns-upstream-patches@051c654277ee4cc4e6102b5a392854b4004ad392`](https://github.com/dantte-lp/powerdns-upstream-patches/tree/051c654277ee4cc4e6102b5a392854b4004ad392),
  including its `Taskfile.yml`, development Containerfile and Compose files,
  automation scripts, CI workflow, and release shape;
- the successor implementation cross-check,
  [`ioplane/terraform-provider-powerdns@9ef6fb0ba8ed4449c839639d1ba659771a812d8d`](https://github.com/ioplane/terraform-provider-powerdns/tree/9ef6fb0ba8ed4449c839639d1ba659771a812d8d),
  including its controlled launcher, role-oriented automation, containerized
  gates, package checks, and release workflow.

Both repository identities and commits were read back through the GitHub API
on 2026-08-06. These anchors record provenance; Nutanix retains its own
contracts and does not inherit PowerDNS policy wholesale.

Corrections relative to the PowerDNS repository:

- host Task is not required; the launcher enters the pinned container;
- podman-compose is locked by `uv.lock`, not whatever happens to be installed;
- tracker state is a dependency graph, not a manually expanded plan;
- no automation target contains unconditional recursive deletion;
- M0 does not create a fake lab before there is a bounded Nutanix acceptance
  design.

### Current contract reconciliation

The bullets above record the capabilities studied during the original M0
design. The current repository contract deliberately narrows the active gate
while the main Nutanix product corpus is being implemented:

- Python automation receives ruff and ty analysis, but no new automation test
  suite; existing non-product tests are frozen and excluded from delivery;
- `task verify` is intentionally an exact alias for the lightweight `task all`
  gate until the corresponding product corpus and product acceptance contract
  exist;
- `package:test` remains a separate packaging-shape command and is excluded
  from the implementation gate until the packaging and release evidence phase;
- the static pin checker validates the immutable allowlist and reference
  syntax; successful `./dev up` builds and successful GitHub Actions runs are
  the runtime evidence that the pinned base image and Actions resolve;
- release publication is intentionally deferred. `.goreleaser.yml` records the
  packaging shape, but no release publication workflow is required in the
  current M2 implementation sprint.

This reconciliation supersedes the older gate-strength statements for current
work without rewriting the historical design intent.

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
.gitignore
go.mod
go.sum
cmd/terraform-provider-nutanix/main.go
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
specs/nutanix/manifest.json
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
| Artifacts | all locked Developer Portal artifacts validate and match SHA-256 |
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
