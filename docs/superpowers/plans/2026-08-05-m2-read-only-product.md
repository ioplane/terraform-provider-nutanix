# M2 Read-only Product Implementation Plan

**Goal:** Register the four currently corroborated hand-written Nutanix data
sources, retain the two IAM types as fail-closed work, and complete all six
before starting their product-test phase.

**Beads parent:** `ntnx-m2`

## Delivery order

- [x] Approve the exact API, schema, state, identity, and deferred-test
  contract in `ntnx-m2.1`.
- [x] Implement shared list-query and response primitives with one concrete
  responsibility; add no generic SDK or generated code.
- [x] Query every planned operation independently through `nutanix-mcp` and
  record four exact-path matches, operation-ID indexing coverage, and the two
  fail-closed IAM path gaps.
- [x] Reapprove the revised function interaction and ownership contract.
- [x] Implement the MCP-corroborated `clustermgmt`, `prism`, `vmm`, and
  `networking` read operations and hand-written DTO projections.
- [ ] Implement the two `iam` readers only after exact-operation MCP evidence
  exists and the contract records it.
- [x] Implement and register `nutanix_clusters_v2`,
  `nutanix_categories_v2`, `nutanix_images_v2`, and `nutanix_subnet_v2`.
- [ ] Implement and register `nutanix_roles_v2` and `nutanix_operations_v2`
  only after their exact-operation MCP gate succeeds.
- [x] Update public contract, generated provider documentation, Beads, and the
  roadmap to reflect the implemented surface.
- [x] Run only the lightweight format, static-analysis, documentation, and
  build gate during implementation.
- [ ] After the complete six-type corpus exists, open a separate product-test
  phase limited to the deferred evidence boundary in the approved contract.

## 2026-08-05 consistency audit

- [x] Prove that the upstream Nutanix checkout owns the legacy module identity
  and `go 1.25.3`; keep the `ioplane` module on its independent Go 1.26
  contract.
- [x] Verify the exact Go 1.26.5 container build, current `gopls`, direct
  module versions, module graph, format, vet, lint, and build without running
  tests.
- [x] Upgrade the one outdated direct dependency,
  `terraform-plugin-log`, to `v0.11.0`.
- [x] Replace three duplicated list-query schema and conversion
  implementations with `internal/service/listquery` while retaining
  hand-written product DTO and state mapping.
- [x] Inspect the AOS 7.6 VMM v4.3 extraction, record its source limitations
  and 13-operation delta, and create `ntnx-m4.1` for MCP and schema
  qualification before adoption.
- [x] Close `ntnx-m2.8` only after the complete lightweight gate and tracker
  projections pass.

## 2026-08-05 RE handoff reconciliation

- [x] Pin the unlisted Gist revision and content digest through `gh api` and
  remove all live endpoint and credential-store details from repository
  evidence.
- [x] Verify official-provider module, branch, commit, release, and Go/SDK
  claims through the GitHub API rather than inheriting the handoff wording.
- [x] Compare the handoff version map with live Developer Portal discovery and
  exact MCP searches; retain the per-namespace lock when they differ.
- [x] Reject the proposed upstream-fork and SDK-golden-path changes because
  they conflict with the approved greenfield hand-written architecture.
- [x] Keep Basic and API-key authentication unchanged; defer uncorroborated
  OIDC, service-token, and certificate-header claims to a separate contract.
- [x] Define a deterministic MCP ingestion input and exact-search acceptance
  in the sanitized IAM evidence handoff.
- [x] Identify and pin the version-controlled MCP corpus producer through the
  GitHub API; do not treat the detached local catalog as provenance.
- [x] Prove that the producer's 10,000-character OpenAPI truncation excludes
  both IAM operation IDs even though its path-only endpoint list survives.
- [x] Correct the producer locally to emit operation-aware bounded sections,
  then run an isolated IAM render that retains both exact paths and operation
  IDs; keep the change uncommitted and the live corpus untouched.
- [ ] After explicit commit approval, publish the reviewed producer correction
  and run an isolated catalog ingestion that passes all live MCP acceptance
  queries before closing `ntnx-m2.9`; do not patch the live corpus ad hoc.
- [ ] Keep `ntnx-m2.7` blocked until both IAM paths pass that live MCP gate.
- [x] Track Licensing v4.3 as an explicit M9 qualification item rather than
  inserting a v4.0 custom client into M2.

## 2026-08-06 tracked RE corpus refresh

- [x] Replace mutable local-worktree provenance with the tracked extraction
  commit `740198d8cfb3274695a79411d44e2282b4513e15` from
  `ioplane/nutanix-re` and retain the earlier untracked observation only as
  timeline evidence.
- [x] Record per-document paths, operation counts, placeholder-schema counts,
  hashes, and normalized PC 7.5.1.6 versus PC 7.6 operation parity.
- [x] Compare Licensing v4.0 and Prism v4.0.b1 shipped routes with the current
  Portal contracts; keep v4.3 authoritative and exclude retired or beta-only
  routes.
- [x] Prove that the shipped Lifecycle v4.2 client contains all 29 official
  Portal operations, while keeping its 35 additional routes outside the
  contract until exact Portal and MCP evidence exists.
- [x] Keep preview `lcm` v4.0.a2 separate from `lifecycle` v4.2; do not merge
  the two namespace surfaces.
- [x] Requery current and legacy Licensing, Prism, Lifecycle, and LCM paths
  through MCP and record both exact matches and fail-closed misses.
- [x] Treat the latest decompiled authentication findings as threat-model
  input only; retain Basic and API key as the complete public auth contract.
- [x] Track Prism v4.3 management expansion and Lifecycle v4.2 as separate
  future product tasks behind the M9 phase chain.

## 2026-08-06 live MCP and LSP revalidation

- [x] Requery MCP health and both exact IAM operation/path pairs. The store
  still reports 7,343 documents and 489,531 chunks, with its last ingestion
  `partial` on 2026-05-01.
- [x] Confirm that neither `listRoles GET /iam/v4.0/authz/roles` nor
  `listOperations GET /iam/v4.0/authz/operations` resolves to an IAM API
  document; `list_documents(product="iam")` still returns zero documents.
- [x] Keep `ntnx-m2.9` in progress and `ntnx-m2.7` blocked. Do not interpret
  IAM-related KB or release-note matches as operation-level API evidence.
- [x] Verify the production workspace with Go 1.26.5 and `gopls` 0.23.0:
  workspace load reports zero diagnostics and explicit `gopls check` passes
  for all 38 non-test Go files.
- [x] Use LSP implementation, reference, and call-hierarchy queries to prove
  the current call graph: provider configuration constructs four namespace
  clients; each consumer-side reader interface has one concrete client; each
  data-source `Read` has one namespace operation and one state-set path.
- [x] Confirm from LSP references that the four namespace operations are used
  only by their owning Terraform data source and do not leak a generic client
  across service boundaries.

## 2026-08-06 diagnostic-boundary remediation

- [x] Obtain an independent contract review of provider configuration, the
  four reader paths, query identity, null mapping, and error propagation.
- [x] Reproduce the review finding in the function graph: syntactically
  invalid configured OData values currently reach `odata.Build` inside the
  reader, and raw Framework mapping diagnostics currently reach `Read`.
- [x] Record the corrected ownership and call graph in the M2 contract and
  track implementation as `ntnx-m2.10` while keeping the IAM evidence task
  blocked.
- [x] Obtain independent ARC approval of the correction before modifying
  production code.
- [x] Implement one shared redacted OData input validator that returns only a
  neutral option enum and preserves `ErrInvalidListQuery`; map the enum and
  static known-and-valid detail at the service boundary, and retain namespace
  defense-in-depth.
- [x] Collapse every state-mapping diagnostic set to the owning data source's
  single static read diagnostic and prevent `State.Set`.
- [x] Re-run LSP diagnostics, references, and call hierarchy plus the complete
  lightweight Podman gate; add and run no product tests.
- [x] Obtain independent spec-compliance and code-quality approvals before
  closing `ntnx-m2.10`.

## Implementation constraints

- Work only in `sprint/m2-readonly` and its dedicated worktree.
- Use Podman through `./dev` for Go, Terraform, Task, Beads, Python CLI,
  generation, packaging, and evidence commands.
- Keep exactly one critical-path Beads task in progress.
- Keep wire operations in namespace packages and Terraform behavior in service
  packages.
- Before each operation, retain its MCP exact-path result and operation-ID
  indexing status beside the locked Developer Portal binding; stop on a
  missing or conflicting MCP path.
- Keep the approved function call graph, DTO ownership, null mapping, and
  single-diagnostic error path synchronized with implementation.
- Do not add a Nutanix SDK, OpenAPI generator, generic REST framework, or
  service locator.
- Do not contact PE or PC during implementation.
- Do not add or run tests until the main M2 product corpus is complete.

## Completion evidence

M2 implementation tasks use `gofmt`, `go vet`, the pinned linter and
vulnerability scanner, documentation checks, and `go build` through the
lightweight container gate. These checks prove that the product corpus builds
and meets static policy; they are not presented as product behavioral proof.

The unpublished working set and its atomic GitHub delivery boundary are
projected in `docs/standards/m2-delivery-manifest.md`; Beads task
`ntnx-m2.11` remains canonical for that reconciliation.
