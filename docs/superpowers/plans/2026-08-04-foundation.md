# Terraform Provider Nutanix Foundation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a reproducible M0 repository with locked Nutanix Developer Portal artifacts, a Podman-only development workflow, Beads governance, and an empty Terraform Plugin Framework provider that negotiates protocol 6.

**Architecture:** Build one Go binary as a modular monolith: Terraform-facing code stays in `internal/provider` and future `internal/service` packages, while hand-written Nutanix clients will live in namespace-specific `internal/nutanix` packages over focused transport packages. The host only orchestrates Podman through a locked Python launcher; every build, test, linter, artifact check, package check, and Terraform handshake runs in the `golang:1.26-trixie` toolbox.

**Tech Stack:** Go 1.26.5, Terraform Plugin Framework 1.19.0, Terraform Plugin Protocol 6, Terraform 1.15.8, Podman 5, podman-py 5.8.0, podman-compose 1.6.0, uv 0.12.1, Python 3.13, Task 3.52.0, Beads 1.1.2, GitHub CLI 2.97.0, Pytest 9.1.1, Ruff 0.16.1, ty 0.0.66, GoReleaser 2.17.1.

---

## Fixed references and execution rules

- Design specification: `docs/superpowers/specs/2026-08-04-foundation-design.md`.
- Official API registry: `https://developers.nutanix.com/api/v1/namespaces/`.
- Go sources: `https://go.dev/doc/go1.26`, `https://go.dev/doc/modules/layout`, `https://go.dev/wiki/CodeReviewComments`, and `https://go.dev/doc/security/best-practices`.
- Framework source: `https://developer.hashicorp.com/terraform/plugin/framework`.
- Work only in `/opt/projects/repositories/.worktrees/terraform-provider-nutanix/m0-foundation` on `sprint/m0-foundation`.
- Write production behavior with red-green-refactor. Configuration and prose files are the TDD exception; checks for their normative content are still added before M0 closes.
- Do not run Go, Terraform, Task, Beads, Python tests, linters, generators, or package tools on the host. The only host tools used by the workflow are Git, GitHub CLI, Podman, and uv for the locked launcher environment.
- Do not use Nutanix SDKs as dependencies or generate Terraform schemas/DTOs from OpenAPI.
- Do not use a live PE/PC in M0.
- Each task ends in a focused commit. Do not combine later-task work into an earlier commit.

## Planned file ownership

| Area | Files | Responsibility |
| --- | --- | --- |
| Repository contract | `AGENTS.md`, `CODEX.md`, `CLAUDE.md`, `README.md`, `CONTRIBUTING.md`, `SECURITY.md`, `CHANGELOG.md`, `VERSION` | Human and agent operating contract |
| Architecture contract | `docs/architecture.md`, `docs/contract.md`, `docs/standards/*.md`, `docs/adr/*.md` | Stable naming, API-source, state, and architecture decisions |
| Host orchestration | `dev`, `scripts/automation/*.py` | Bounded podman-compose orchestration and podman-py inspection |
| Toolbox | `deployments/containers/Containerfile.dev`, `deployments/compose/compose.dev.yml`, `pyproject.toml`, `uv.lock`, `Taskfile.yml` | Exact development environment and command interface |
| Artifact pipeline | `scripts/artifacts/nutanix.py`, `specs/nutanix/manifest.json` | Discover, lock, download, validate, and verify official artifacts |
| Tracker | `.beads/*`, `scripts/checks/tracker.py`, `scripts/generate/roadmap.py`, `docs/roadmap.md` | Canonical dependency graph and generated human view |
| Provider | `go.mod`, `go.sum`, `cmd/terraform-provider-nutanix/main.go`, `internal/provider/*.go` | Empty Framework provider and protocol 6 entrypoint |
| Delivery | `scripts/checks/*.py`, `scripts/package/provider.py`, `.goreleaser.yml`, `.github/*` | Quality gates, deterministic package, CI, and dependency automation |

### Task 1: Repository and engineering contracts

**Files:**

- Create: `.editorconfig`
- Create: `.gitattributes`
- Create: `.gitignore`
- Create: `AGENTS.md`
- Create: `CODEX.md`
- Create: `CLAUDE.md`
- Modify: `README.md`
- Create: `CONTRIBUTING.md`
- Create: `SECURITY.md`
- Create: `CHANGELOG.md`
- Create: `VERSION`
- Create: `docs/architecture.md`
- Create: `docs/contract.md`
- Create: `docs/standards/go-1.26.md`
- Create: `docs/standards/naming.md`
- Create: `docs/standards/nutanix-artifacts.md`
- Create: `docs/standards/testing.md`
- Create: `docs/adr/0001-modular-monolith.md`
- Create: `docs/adr/0002-hand-written-nutanix-client.md`
- Create: `docs/adr/0003-podman-development-boundary.md`
- Create: `docs/adr/0004-beads-tracker.md`
- Create: `docs/adr/0005-nutanix-artifact-lock.md`

- [ ] **Step 1: Establish repository hygiene before any generated/downloaded data exists**

  Add LF/UTF-8 defaults. Ignore `.venv/`, `.cache/`, Go outputs, Terraform local data, coverage files, IDE files, and Beads' local Dolt database while keeping `.beads/config.yaml` and `.beads/issues.jsonl` reviewable. Prove the exact vendor cache path is ignored with `git check-ignore .cache/nutanix/artifacts/example/openapi.yaml`.

  Expected: the path is printed and exit status is 0.

- [ ] **Step 2: Write the repository operating contract**

  `AGENTS.md` must require: design-before-implementation for Terraform types; official locked artifacts; hand-written transport/DTO/schema; TDD; all validation inside Podman; Beads as canonical state; one sprint/branch/worktree/PR; no live mutation without an acceptance contract; evidence before closure. `CODEX.md` and `CLAUDE.md` contain only a pointer to `AGENTS.md`.

- [ ] **Step 3: Write stable architecture, public contract, and naming documents**

  Split the approved design into focused documents. Include the exact dependency direction, no catch-all Go packages, initialism rules (`API`, `HTTP`, `ID`, `JSON`, `TLS`, `URL`, `UUID`, `ETag`), Terraform naming and compatibility rules, API artifact precedence, GA/preview selection, context/error/concurrency rules, and red-green-refactor requirements. Each decision document links its primary source URL and the approved design spec.

- [ ] **Step 4: Write contribution, security, release-history, and README entry points**

  README must state M0 status without claiming implemented resources, show only containerized commands, link contracts, and identify `ioplane/nutanix`. SECURITY forbids public issue disclosure of vulnerabilities and explains the GitHub private reporting path. CHANGELOG starts with an Unreleased foundation section. VERSION is `0.0.0-dev`.

- [ ] **Step 5: Check and commit the contract-only change**

  Run:

  ```bash
  git diff --check
  git status --short
  ```

  Expected: `git diff --check` is clean. The normative naming scan is added to
  the in-container repository gate after the toolbox exists.

  Commit:

  ```bash
  git add .editorconfig .gitattributes .gitignore AGENTS.md CODEX.md CLAUDE.md README.md CONTRIBUTING.md SECURITY.md CHANGELOG.md VERSION docs
  git commit -m "docs: establish provider engineering contracts"
  ```

### Task 2: Reproducible Podman toolbox and tested host launcher

**Files:**

- Create: `deployments/containers/Containerfile.dev`
- Create: `deployments/compose/compose.dev.yml`
- Create: `pyproject.toml`
- Create: `uv.lock`
- Create: `Taskfile.yml`
- Create: `dev`
- Create: `scripts/__init__.py`
- Create: `scripts/automation/__init__.py`
- Create: `scripts/automation/project.py`
- Create: `scripts/automation/process.py`
- Create: `scripts/automation/podman_api.py`
- Create: `scripts/automation/cli.py`
- Create: `tests/automation/test_project.py`
- Create: `tests/automation/test_process.py`
- Create: `tests/automation/test_podman_api.py`
- Create: `tests/automation/test_cli.py`

- [ ] **Step 1: Add configuration needed to create the test environment**

  Pin the base image exactly:

  ```text
  docker.io/library/golang:1.26-trixie@sha256:4ee9ffa999b4583ce281939cdff828763083610292f252279a0cee77473bd9a7
  ```

  Containerfile arguments pin Task 3.52.0, Beads 1.1.2, Terraform 1.15.8, golangci-lint 2.12.2, GoReleaser 2.17.1, terraform-plugin-docs 0.25.0, govulncheck 1.6.0, GitHub CLI 2.97.0, and uv 0.12.1. Download archives only from official release locations, verify published SHA-256 data, install for both amd64 and arm64 where available, set `GOTOOLCHAIN=local`, and attach complete OCI annotations.

  Compose has one `dev` service, no top-level `version`, no Podman socket mount, `/workspace:z`, worktree-specific named caches, and a health check. `pyproject.toml` pins podman 5.8.0, podman-compose 1.6.0, pytest 9.1.1, ruff 0.16.1, ty 0.0.66, and PyYAML 6.0.3. Generate `uv.lock` with host uv, which is an allowed bootstrap control-plane operation:

  ```bash
  uv lock
  ```

- [ ] **Step 2: Write failing project/process tests**

  Tests require:

  ```python
  def test_project_name_is_stable_and_worktree_specific(tmp_path): ...
  def test_run_uses_argument_array_without_shell(): ...
  def test_timeout_terminates_the_process_group(): ...
  def test_nonzero_exit_preserves_stdout_and_stderr(): ...
  ```

  Build the configuration-only image and run the tests in it:

  ```bash
  BOOTSTRAP_PROJECT="nutanix-m0-bootstrap-$(git hash-object --stdin <<<"$PWD")"
  uv run --frozen podman-compose -p "${BOOTSTRAP_PROJECT}" -f deployments/compose/compose.dev.yml build dev
  uv run --frozen podman-compose -p "${BOOTSTRAP_PROJECT}" -f deployments/compose/compose.dev.yml run --rm dev uv run --frozen pytest -q tests/automation/test_project.py tests/automation/test_process.py
  ```

  Expected: FAIL because `scripts.automation.project` and `process` behavior is absent.

- [ ] **Step 3: Implement the minimal project and bounded process primitives**

  `project.py` resolves the Git common directory and worktree path, then builds a DNS-safe SHA-256-derived suffix. `process.py` always calls `subprocess.Popen` with an argument sequence, `shell=False`, `start_new_session=True`, captured text streams, and a timeout; on timeout it sends TERM then KILL to the process group and raises a typed `CommandTimeoutError`.

  Run the same test command. Expected: PASS.

- [ ] **Step 4: Write failing Podman API and CLI tests**

  Use fake Unix-socket paths and injected client/runner factories. Assert that the API wrapper sets `CONTAINER_HOST`, enters `PodmanClient.from_env()` as a context manager, handles typed Podman exceptions, waits for the exact Compose container health state, and never mutates via a CLI fallback. Assert exact forwarding for `up`, `down`, `status`, `shell`, `task`, and `beads`. For remote Beads operations, assert the launcher resolves an existing `GH_TOKEN`/`GITHUB_TOKEN` or the host `gh auth token`, injects it only into the relevant exec environment, initializes the in-container GitHub credential helper, and never includes the token in arguments, output, exceptions, or files.

  Run:

  ```bash
  BOOTSTRAP_PROJECT="nutanix-m0-bootstrap-$(git hash-object --stdin <<<"$PWD")"
  uv run --frozen podman-compose -p "${BOOTSTRAP_PROJECT}" -f deployments/compose/compose.dev.yml run --rm dev uv run --frozen pytest -q tests/automation/test_podman_api.py tests/automation/test_cli.py
  ```

  Expected: FAIL because the wrapper/CLI behavior is absent.

- [ ] **Step 5: Implement the minimal Podman wrapper and launcher**

  The launcher must call locked `podman-compose` using argument arrays and bounded deadlines, then inspect readiness and exec using podman-py. Socket discovery accepts explicit `CONTAINER_HOST`, then `/run/user/<uid>/podman/podman.sock`, then `/run/podman/podman.sock`; it exports the chosen URI before `PodmanClient.from_env()`. Status may report a diagnostic CLI fallback, but `up`, `down`, and exec never do. Remote Beads sync uses the pinned in-container `gh` credential helper with a token injected for that exec only; no host credential directory or token file is mounted. The shell `dev` file only runs `uv run --frozen python -m scripts.automation.cli "$@"`.

  Run the Podman/CLI tests. Expected: PASS.

- [ ] **Step 6: Add the initial Task interface and prove the real container path**

  Initial tasks are `versions`, `python:format`, `python:lint`, `python:typecheck`, and `python:test`. Every task checks `/.dockerenv` or `/run/.containerenv` and `/workspace`. Then run:

  ```bash
  ./dev up
  ./dev status
  ./dev task versions
  ./dev task python:test
  ./dev task python:lint
  ./dev task python:typecheck
  ```

  Expected: healthy worktree-specific container; exact pinned versions; all Python tests and checks pass inside it.

- [ ] **Step 7: Commit the toolbox and launcher**

  ```bash
  git add deployments pyproject.toml uv.lock Taskfile.yml dev scripts/automation scripts/__init__.py tests/automation
  git commit -m "build: add reproducible Podman development toolbox"
  ```

### Task 3: Canonical Beads graph and generated roadmap

**Files:**

- Create: `.beads/config.yaml`
- Create: `.beads/issues.jsonl`
- Create: `scripts/checks/tracker.py`
- Create: `scripts/generate/roadmap.py`
- Create: `tests/checks/test_tracker.py`
- Create: `tests/generate/test_roadmap.py`
- Create: `docs/roadmap.md`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Initialize embedded Dolt without agent-file generation**

  Run only through the container:

  ```bash
  ./dev beads init --skip-agents
  ./dev beads bootstrap
  ```

  Expected: `.beads` is initialized, origin is recognized as the Dolt remote, and repository-owned `AGENTS.md` is unchanged.

- [ ] **Step 2: Build the macro and M0 dependency graph**

  Create explicit epics `ntnx-m0` through `ntnx-m10`, with each later macro phase depending on the previous phase. Under `ntnx-m0`, create tasks `ntnx-m0.1` through `ntnx-m0.8` matching this implementation plan and add sequential blocking dependencies. Every issue includes description, acceptance criteria, priority, labels, and the design/plan path. Mark `.1` and `.2` closed with commit evidence, claim `.3`, and leave all later work open:

  ```bash
  ./dev beads update ntnx-m0.3 --claim
  ./dev beads ready --json
  ./dev beads list --status=in_progress --json
  ```

  Expected: exactly `ntnx-m0.3` is in progress and no blocked task is reported ready.

- [ ] **Step 3: Write failing tracker and roadmap tests**

  Use JSON fixtures rather than a fake database. Test one-current-critical-task, no dependency cycle, required M0-M10 epic set, ready/front consistency, stable roadmap ordering, and rejection of Markdown-authored status.

  Run:

  ```bash
  ./dev task python:test -- tests/checks/test_tracker.py tests/generate/test_roadmap.py
  ```

  Expected: FAIL because the checker/generator is absent.

- [ ] **Step 4: Implement the checker and generator**

  `tracker.py` invokes `bd ... --json` through an injected runner and validates structure. `roadmap.py` renders a deterministic macro-phase table and current-task section from JSON. No Markdown parsing is used as tracker input.

  Run the focused tests. Expected: PASS.

- [ ] **Step 5: Export, generate, commit Dolt state, and synchronize**

  Export deterministically with `./dev beads export --output .beads/issues.jsonl`; generate `docs/roadmap.md`; run the tracker gate; commit Beads' database history and push the separate Dolt ref:

  ```bash
  ./dev beads vc commit -m "Initialize provider delivery graph"
  ./dev beads dolt push
  ```

- [ ] **Step 6: Commit repository-visible tracker state**

  ```bash
  git add .beads/config.yaml .beads/issues.jsonl scripts/checks/tracker.py scripts/generate/roadmap.py tests/checks/test_tracker.py tests/generate/test_roadmap.py docs/roadmap.md Taskfile.yml
  git commit -m "chore: initialize Beads delivery graph"
  git push origin sprint/m0-foundation
  ```

- [ ] **Step 7: Validate the committed Git and Dolt refs from a fresh clone**

  Set `VERIFY_DIR="../terraform-provider-nutanix-beads-$(git rev-parse --short HEAD)"`, clone `sprint/m0-foundation` from the GitHub origin into that new path, and verify the same origin exposes `refs/dolt/data`. Then run `./dev beads bootstrap`, `./dev beads dolt pull`, `./dev task tracker:check`, and generated-roadmap no-diff from the clone. Fail closed if the target already exists and do not delete it automatically.

### Task 4: Nutanix Developer Portal artifact lock

**Files:**

- Create: `scripts/artifacts/__init__.py`
- Create: `scripts/artifacts/nutanix.py`
- Create: `tests/artifacts/test_nutanix.py`
- Create: `tests/fixtures/nutanix/registry.json`
- Create: `tests/fixtures/nutanix/versions.json`
- Create: `tests/fixtures/nutanix/openapi.yaml`
- Create: `tests/fixtures/nutanix/postman.json`
- Create: `tests/fixtures/nutanix/errors.json`
- Create: `scripts/checks/staged.py`
- Create: `tests/checks/test_staged.py`
- Create: `specs/nutanix/manifest.json`
- Modify: `Taskfile.yml`
- Modify: `docs/standards/nutanix-artifacts.md`

- [ ] **Step 1: Write failing selection and validation tests**

  Serve fixtures from a real local `http.server.ThreadingHTTPServer`. Tests require: GET only; all registry namespaces selected; newest GA preferred over newer preview; preview selected only when no GA exists; accepted and rejected HTTP `Content-Type` values for every artifact kind; OpenAPI begins with 3.0/3.1 metadata; Postman parses as an object with `item`; error reference parses as a JSON array; safe namespace/version paths; byte count and SHA-256 lock; temp-file-plus-rename writes; digest mismatch failure; manifest namespace count output; and actionable HTTP/JSON/YAML diagnostics. `test_staged.py` uses a temporary Git repository and proves the staged-file checker rejects `.cache/nutanix/artifacts/**` after `git add -f` while accepting the manifest and ordinary source.

  Run:

  ```bash
  ./dev task python:test -- tests/artifacts/test_nutanix.py tests/checks/test_staged.py
  ```

  Expected: FAIL because the artifact pipeline is absent.

- [ ] **Step 2: Implement minimal discover/update/verify commands**

  Use only `urllib.request`, `hashlib`, `json`, `pathlib`, `tempfile`, and PyYAML. Public functions receive base URL, manifest path, cache path, and opener dependencies. Validate response `Content-Type` before parsing or locking: JSON for registry, version, Postman, and error-reference responses; YAML or plain text containing OpenAPI YAML for specifications. Record the observed media type in each artifact lock entry. CLI subcommands:

  ```text
  discover  print registry/version selection JSON without writes
  update    download selected artifacts and atomically write proposed lock
  verify    download and verify live registry set, selection, shape, size, digest
  count     print the locked namespace count as a decimal integer
  ```

  `staged.py` reads the actual index with `git diff --cached --name-only` and rejects vendor-cache paths. Never log response bodies, credentials, authorization headers, or environment variables. Never follow a URL outside `https://developers.nutanix.com/api/v1/` unless a test base URL was explicitly injected. Add Task targets for all four artifact subcommands and `repo:staged-cache-check` in this step.

  Run the focused tests. Expected: PASS.

- [ ] **Step 3: Create and independently verify the real 19-namespace lock**

  Use the Task targets added in Step 2. Run:

  ```bash
  ./dev task artifacts:discover
  ./dev task artifacts:update
  ./dev task artifacts:verify
  ./dev task artifacts:count
  git check-ignore .cache/nutanix/artifacts/aiops/v4.0/openapi.yaml
  ```

  Expected: 19 namespaces; 18 GA and one explicitly preview `storage v4.0.a3`; every available OpenAPI, Postman, and English error reference has nonzero size and SHA-256; cache path is ignored. Compare the result to the exact selection table in the design spec.

- [ ] **Step 4: Record source behavior and commit only the lock**

  Update the standard with the verified observation that artifact bodies are retrieved with GET and HEAD is not a supported probe. Confirm no vendor artifact body is staged:

  ```bash
  git status --short
  git add scripts/artifacts scripts/checks/staged.py tests/artifacts tests/checks/test_staged.py tests/fixtures/nutanix specs/nutanix/manifest.json Taskfile.yml docs/standards/nutanix-artifacts.md
  ./dev task repo:staged-cache-check
  git commit -m "feat: lock official Nutanix API artifacts"
  ```

### Task 5: Empty Framework provider and protocol 6 handshake

**Files:**

- Create: `go.mod`
- Create: `go.sum`
- Create: `cmd/terraform-provider-nutanix/main.go`
- Create: `cmd/terraform-provider-nutanix/main_test.go`
- Create: `internal/provider/provider.go`
- Create: `internal/provider/provider_test.go`
- Create: `internal/provider/provider_acc_test.go`
- Create: `examples/provider/provider.tf`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Initialize the exact module inside the container**

  Run:

  ```bash
  ./dev task go:mod:init
  ```

  The task creates module `github.com/ioplane/terraform-provider-nutanix`, sets `go 1.26.0`, and adds exact Framework 1.19.0 plus plugin-testing 1.16.0 requirements using `GOTOOLCHAIN=local`. It runs `go mod download`, not `go mod tidy`, because no importing source exists yet. Verify both exact modules, no `toolchain` directive, and no Nutanix SDK module.

- [ ] **Step 2: Write failing provider contract tests**

  Tests assert compile-time implementation of `provider.Provider`, metadata type name `nutanix`, supplied build version, nonempty provider description, an intentionally empty M0 attribute map, no diagnostics for empty configuration, and zero resources/data sources. Use real Framework request/response values, no mock Framework.

  After the test source imports both pinned modules, run
  `./dev task go:mod:tidy` and verify both requirements remain before running
  the red test.

  Run:

  ```bash
  ./dev task go:test -- ./internal/provider
  ```

  Expected: FAIL because `provider.New` and its methods do not exist.

- [ ] **Step 3: Implement the smallest provider that passes**

  `provider.New(version string) func() provider.Provider` returns a concrete provider. Implement only `Metadata`, `Schema`, `Configure`, `Resources`, and `DataSources`; do not add connection attributes, clients, resources, or speculative interfaces. Add doc comments and canonical initialisms.

  Run the focused unit test. Expected: PASS.

- [ ] **Step 4: Write failing entrypoint and protocol 6 tests**

  `cmd/terraform-provider-nutanix/main_test.go` calls an unexported
  `serveOptions` helper and requires registry address
  `registry.terraform.io/ioplane/nutanix` plus explicit protocol version 6.

  Use `ProtoV6ProviderFactories` from plugin-testing and Terraform 1.15.8 with:

  ```hcl
  terraform {
    required_providers {
      nutanix = { source = "ioplane/nutanix" }
    }
  }

  provider "nutanix" {}
  ```

  The test is guarded by `TF_ACC=1` and has no Nutanix endpoint. Run:

  ```bash
  ./dev task go:test:protocol
  ```

  Expected: FAIL because `serveOptions` and the protocol-6 entrypoint are absent,
  not because the Terraform binary is missing.

- [ ] **Step 5: Implement the protocol 6 entrypoint**

  `serveOptions` returns `providerserver.ServeOpts` with registry address
  `registry.terraform.io/ioplane/nutanix`, the requested debug value, and
  explicit `ProtocolVersion: 6`. `main.go` calls `providerserver.Serve` with
  those options and the link-time version. Do not select protocol 5 or
  automatic negotiation.

  Run:

  ```bash
  ./dev task go:format
  ./dev task go:vet
  ./dev task go:test
  ./dev task go:test:race
  ./dev task go:test:protocol
  ./dev task go:build
  ```

  Expected: all pass; the produced binary is named `terraform-provider-nutanix` and `go version -m` reports the expected module/dependencies.

- [ ] **Step 6: Commit the provider foundation**

  ```bash
  git add go.mod go.sum cmd internal examples Taskfile.yml
  git commit -m "feat: serve empty provider over protocol 6"
  ```

### Task 6: Repository gates, generated docs, and deterministic package

**Files:**

- Create: `scripts/checks/repository.py`
- Create: `scripts/checks/pins.py`
- Create: `scripts/checks/tool_versions.py`
- Create: `scripts/package/__init__.py`
- Create: `scripts/package/provider.py`
- Create: `tests/checks/test_repository.py`
- Create: `tests/checks/test_pins.py`
- Create: `tests/checks/test_tool_versions.py`
- Create: `tests/package/test_provider.py`
- Create: `.goreleaser.yml`
- Create: `docs/index.md`
- Modify: `Taskfile.yml`
- Modify: `README.md`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Write failing repository/pin/version tests**

  Test: required files and pointer-file exactness; no tracked `.cache`/secret/state files; no Nutanix SDK in `go.mod`; exact base digest and tool args; no floating image/action references; Compose has no `version`/socket mount; CI markers equal Containerfile versions; manifest covers design namespaces; and `versions` output is machine-parseable.

  Run:

  ```bash
  ./dev task python:test -- tests/checks
  ```

  Expected: FAIL because the checks are absent.

- [ ] **Step 2: Implement focused read-only checks and integrate Task targets**

  Checks accept repository root arguments and return structured diagnostics. Add `repo:check`, `pins:check`, `tools:check`, `docs:check`, `go:lint`, and `go:vuln`. `docs:check` generates Framework docs in a temporary directory and compares content without rewriting the checkout.

  Run focused tests. Expected: PASS.

- [ ] **Step 3: Write failing deterministic package tests**

  Tests create two packages from the same fake binary with fixed `SOURCE_DATE_EPOCH`, then assert byte-identical ZIPs, sorted entries, normalized timestamps/modes, SHA256SUMS format, consumer filename `terraform-provider-nutanix_<version>`, and safe rejection of path traversal/version injection.

  Run:

  ```bash
  ./dev task python:test -- tests/package/test_provider.py
  ```

  Expected: FAIL because packaging is absent.

- [ ] **Step 4: Implement minimal deterministic packaging and install smoke**

  Package only the provider binary and LICENSE. Build twice with `CGO_ENABLED=0`, `-trimpath`, and fixed link-time version; use explicit ZIP metadata. Install the exact ZIP selected by its checksum into a temporary Terraform filesystem mirror. Generate a temporary `TF_CLI_CONFIG_FILE` whose `provider_installation` uses only that filesystem mirror, run `terraform init -backend=false` in the temporary configuration, and only then run `terraform providers schema -json`. Both commands must have network discovery disabled and must use the unpacked binary selected by the verified checksum.

  Run:

  ```bash
  ./dev task package:test
  ```

  Expected: two package hashes are identical and Terraform reports provider schema over protocol 6.

- [ ] **Step 5: Assemble the non-live gate**

  `task all` runs, in order: repository checks, artifact lock verification, Python format/lint/type/test, Go format/vet/lint/test/race/vuln, generated docs drift, OCI/Compose/pin checks, tracker check, build, protocol test, and package test. `task verify` aliases `all` in M0. Run:

  ```bash
  ./dev task all
  ./dev task verify
  ```

  Expected: both exit 0 with no warnings hidden or skipped required gate.

- [ ] **Step 6: Commit the complete local gate**

  ```bash
  git add scripts/checks scripts/package tests/checks tests/package .goreleaser.yml docs/index.md Taskfile.yml README.md CHANGELOG.md
  git commit -m "test: add complete foundation quality gate"
  ```

### Task 7: GitHub CI, dependency automation, and repository controls

**Files:**

- Create: `.github/workflows/ci.yml`
- Create: `.github/dependabot.yml`
- Create: `.github/CODEOWNERS`
- Create: `.github/pull_request_template.md`
- Modify: `scripts/checks/pins.py`
- Modify: `tests/checks/test_pins.py`
- Modify: `Taskfile.yml`

- [ ] **Step 1: Write failing CI-policy tests**

  Require immutable full-SHA actions, minimal `contents: read` permissions, pinned `ubuntu-24.04`, concurrency cancellation, no host Go/Terraform/test commands, a Podman socket service for podman-py, `./dev up`, `./dev task all`, and diagnostic `./dev status` on failure. Require Dependabot coverage for gomod, pip/uv, Docker, and Actions.

  Run:

  ```bash
  ./dev task python:test -- tests/checks/test_pins.py
  ```

  Expected: FAIL because workflow files are absent.

- [ ] **Step 2: Implement CI and dependency automation**

  CI uses pinned checkout and setup-uv actions, verifies host Podman, starts `podman system service --time=0` on a private runner socket, exports `CONTAINER_HOST`, and invokes only the repository launcher for development gates. Action comments carry release tags while executable references remain full SHAs.

  Run focused tests, then `./dev task all`. Expected: PASS.

- [ ] **Step 3: Commit, push, and observe the real workflow**

  ```bash
  git add .github scripts/checks/pins.py tests/checks/test_pins.py Taskfile.yml
  git commit -m "ci: run foundation gate in Podman"
  git push origin sprint/m0-foundation
  gh pr checks 1 --watch
  ```

  Expected: every named PR check passes. If CI fails, use systematic debugging and add a reproducing test before the fix.

- [ ] **Step 4: Apply repository settings after named checks exist**

  With `gh api`, set squash merge only, delete branches after merge, disable merge/rebase commits, keep issues/projects enabled, wiki disabled, and add topics `terraform`, `terraform-provider`, `nutanix`, `golang`, `infrastructure-as-code`. Enable private vulnerability reporting and secret scanning where the public repository/account supports them. Protect `main` with pull-request review, conversation resolution, and the exact successful CI check names. Do not merge the draft PR.

  Read back every setting with `gh api` and save command output in the PR evidence, not in tracked secrets.

### Task 8: Final evidence, tracker reconciliation, and independent audit

**Files:**

- Modify: `.beads/issues.jsonl`
- Modify: `docs/roadmap.md`
- Modify: `CHANGELOG.md`
- Modify: PR #1 body (GitHub only)

- [ ] **Step 1: Rebuild from the committed toolbox and run the full gate twice**

  ```bash
  ./dev up
  ./dev task versions
  ./dev task all
  ./dev task all
  git status --short
  ```

  Expected: both gates pass; the second run proves no generated drift; only intentional tracker/evidence changes remain.

- [ ] **Step 2: Validate clean-clone bootstrap without destructive cleanup**

  Set `VERIFY_DIR="../terraform-provider-nutanix-verify-$(git rev-parse --short HEAD)"`, clone the sprint branch from origin into that new path, then run `./dev up`, `./dev beads bootstrap`, `./dev beads dolt pull`, `./dev task tracker:check`, and `./dev task verify`. Fail closed if the target already exists, record the path for the user, and do not remove it automatically.

- [ ] **Step 3: Reconcile Beads from evidence**

  Close completed `ntnx-m0.*` child tasks with commit/check evidence, claim no later task unless work genuinely remains, regenerate `.beads/issues.jsonl` and `docs/roadmap.md`, commit Beads' Dolt history, and run `./dev beads dolt push`. The M0 epic is closed only if every design exit criterion is satisfied; otherwise leave it open with the precise blocker.

- [ ] **Step 4: Run independent final reviews**

  Dispatch one whole-change spec-compliance reviewer and one code-quality reviewer over the base `3fb8b05` through current HEAD. Fix Critical/Important findings with reproducing tests, rerun reviewers, and rerun `./dev task all` after the final code change.

- [ ] **Step 5: Commit/push evidence and update the draft PR**

  ```bash
  git add .beads/issues.jsonl docs/roadmap.md CHANGELOG.md
  git commit -m "chore: record foundation verification evidence"
  git push origin sprint/m0-foundation
  gh pr checks 1 --watch
  ```

  PR body must report actual commands, artifact namespace count, protocol version, package hash reproducibility, Beads/Dolt sync, clean-clone result, repository-setting readback, and unresolved blockers. Keep the PR draft until the user asks to make it ready or merge.

## M0 acceptance checklist

- [ ] Developer Portal lock contains exactly the live registry namespace set and all digests verify.
- [ ] Naming, Go 1.26, public Terraform, architecture, testing, and source contracts are tracked and linked.
- [ ] No Nutanix SDK or generated production code exists.
- [ ] `./dev` is the only host development entrypoint and uses both podman-compose and podman-py.
- [ ] Every completion gate runs inside the pinned Go 1.26.5 toolbox.
- [ ] Beads/Dolt is canonical, synchronized, cycle-free, and reflected by generated roadmap output.
- [ ] Terraform 1.15.8 loads the packaged provider using protocol 6.
- [ ] `task all` and `task verify` pass twice without dirtying the checkout.
- [ ] CI passes the same gate and `main` protection names its actual check.
- [ ] PR #1 remains unmerged and accurately reports any remaining work.
