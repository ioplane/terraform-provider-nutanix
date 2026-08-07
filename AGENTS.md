# Repository Agent Contract

This file is the repository-specific contract for contributors and coding agents. It supplements
the global instructions supplied by the execution environment. The repository is a pre-release,
hand-written Terraform provider for the Nutanix Cloud Platform; implementation claims remain
provisional until the product corpus and authorized acceptance gate are complete.

## Integration and merge policy

- `dev` is the integration branch. Every feature, bug-fix, documentation, dependency, and tooling
  pull request targets `dev`.
- Do not merge feature work directly into `main`, and do not push directly to `main`. `main` is the
  stable and release line; promotion from `dev` to `main` is a separate, explicitly authorized
  release operation.
- Create working branches from the current `dev` tip and keep one concern per branch. Use a
  dedicated worktree when another checkout is active.
- Before deleting a branch, verify its upstream state, merge or PR state, commit ancestry, and
  worktree cleanliness. Never force-delete an unmerged branch merely because its remote ref is gone.
- A squash-merged branch can have divergent Git ancestry while its tree is already present in the
  target. Compare the merged PR commit tree and the branch tree before attempting any replay or
  merge.
- Pull requests are the publication boundary. Require the Foundation check and repository review
  policy before merging. Do not bypass protected-branch rules as a routine workflow.

## Source of truth before work

1. Run `bd prime`, inspect the relevant issue with `bd show`, and claim or create a Beads task
   before editing.
2. Read this file, the current `README.md`, `Taskfile.yml`, the applicable `docs/` standards, and
   the default branch history before planning or changing the repository.
3. Inspect `git status --short --branch`, upstream tracking, active worktrees, and the target branch
   before making a claim about repository state.
4. For API work, use the locked Developer Portal artifacts under `specs/nutanix/manifest.json` and
   `.cache/nutanix/artifacts/`, then require exact operation evidence and Nutanix MCP corroboration
   where the contract demands it. The secondary `ioplane/nutanix-api` index is discovery evidence,
   not a wire-contract source.
5. Treat empty or unexpectedly clean tool output as a diagnostic result: confirm that the tool saw
   the intended files and configuration before calling the check green.

## Product and runtime stack

| Layer | Contract |
| --- | --- |
| Provider | Terraform provider `ioplane/nutanix`, type `nutanix` |
| Framework | Terraform Plugin Framework `v1.19.0`, Protocol 6 |
| Language | Go module `github.com/ioplane/terraform-provider-nutanix`, `go 1.26.0` |
| Build image | `docker.io/library/golang:1.26-trixie@sha256:4ee9ffa999b4583ce281939cdff828763083610292f252279a0cee77473bd9a7` |
| Toolbox | Podman Compose development container, launched by `./dev` |
| IaC test client | Terraform `1.15.8` |
| Task and tracking | Task `3.52.0`; Beads `1.1.2` |
| Go quality | `golangci-lint 2.12.2`, `govulncheck 1.6.0`, `gopls 0.23.0` |
| Documentation | `tfplugindocs 0.25.0`, `rumdl`, `yamllint` |
| Packaging | GoReleaser `2.17.1`, Syft `1.50.0` |
| Python launcher | `uv 0.12.1`; project dependencies are frozen by `uv.lock` |

The direct Go modules are pinned in `go.mod`; Nutanix SDKs, generated API clients, Terraform
SDKv2, generic REST abstractions, and runtime code generation are prohibited. OpenAPI, Postman,
error references, official SDK examples, and MCP results are evidence inputs only.

## Architecture and ownership

- `cmd/terraform-provider-nutanix` owns the provider executable and Protocol 6 server boundary.
- `internal/provider` owns provider schema, configuration, composition, registration, and consumer
  capability interfaces.
- `internal/auth` owns Basic and API-key authorization application.
- `internal/transport` owns the origin-bound HTTPS client, TLS, retries, response limits, typed HTTP
  errors, pagination primitives, request IDs, ETags, and allowlisted logging.
- `internal/task` owns the context-bound, fail-closed task state machine.
- `internal/capability` owns bounded positive and negative capability caching.
- `internal/nutanix/<namespace>` owns versioned paths, operation policies, DTOs, and response
  validation for one Nutanix API namespace.
- `internal/service/<domain>` owns Terraform schemas, state models, diagnostics, lifecycle mapping,
  and the smallest consumer-defined interfaces. Services do not own raw transport or auth.
- `docs/` and `specs/nutanix/` own public contract and artifact evidence; generated provider pages
  under `docs/data-sources/` and `docs/resources/` are rendered from the provider schema.

Keep production code under `cmd/` and `internal/`. Avoid catch-all packages such as `api`,
`common`, `core`, `types`, or `util`.

## Go and security rules

- Run Go commands inside the pinned Podman environment through `./dev`; host toolchains are not
  completion evidence.
- Use `context.Context` as the first argument for request-bound work. Do not store contexts.
- Preserve causes with `%w`; error text starts lower-case and has no terminal punctuation.
- Bound every HTTP response body, close every response body, preserve cancellation, and use explicit
  timeouts. Redirects are rejected.
- Credentials, authorization headers, sensitive query values, response bodies, object names, and
  remote identifiers do not enter logs, diagnostics, or persistent Terraform state unless the
  public contract explicitly requires a reviewed non-secret identity.
- Every goroutine has an owner, cancellation path, bounded lifetime, and collected result.
- Mutations require an exact operation contract, reviewed idempotence, request identity, ETag policy,
  task validation where applicable, and an independently verified rollback or cleanup path.
- Do not reconnect hardware, hunt for passwords, or mutate a live Nutanix target without explicit
  scope and authorization.

## Verification commands

The default implementation gate is:

```bash
./dev up
./dev task versions
./dev task all
```

Use the focused gates when the change requires them:

```bash
./dev task go:test -- ./...
./dev task go:test:race
./dev task go:test:fuzz
./dev task go:test:protocol
./dev task docs:check
./dev task package:test
./dev down
```

`./dev task all` covers repository and artifact checks, Python formatting/lint/type checking,
Go formatting, vet, production lint, vulnerability scanning, documentation, OCI, pin/tool
versions, release configuration, and the provider build. Product acceptance and live tests are
separate gates; a deferred, skipped, unavailable, or partially cleaned live gate is not green.

Generated documentation must be regenerated through `./dev task docs:generate`; never hand-edit a
generated page. `go.mod` and `go.sum` changes require exact version review, `go mod tidy`, the
complete Podman gate, and vulnerability verification.

## Documentation and Beads

- Repository documentation is written in English; conversation and handoff reports may be in
  technical Russian.
- Use present-tense, operational wording. Put investigation narration and evidence in the report,
  not in reference documentation.
- Use Beads for durable task state. Do not create markdown TODO lists or ad-hoc memory files.
- Before implementation: `bd prime`, `bd show <id>`, `bd update <id> --claim`.
- Before reporting completion: verify the worktree and gates, then run `bd close <id> --reason=...`.
- Record durable discoveries with `bd remember`; do not rewrite shared memory files manually.

## Completion standard

Completion requires a clean, evidence-backed result: intended files are reviewed, `git diff --check`
is clean, required Podman gates pass, generated files have no drift, Beads reflects the real state,
and the final report names any deferred product or publication gate. Never promote a plan, subset
test, or unverified source artifact to complete compatibility.
