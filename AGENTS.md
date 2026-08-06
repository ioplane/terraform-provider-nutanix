# Repository Instructions

This file is the authoritative working contract for this repository.

## Before work

- Read the current `README.md` and this file before planning or changing the repository.
- Use Beads as canonical task state. Keep exactly one critical-path task in progress, and attach evidence before closing it.
- Use one sprint, one branch, one worktree, and one pull request. Reference the Beads task in commits and the pull request.

## Product contract

- Obtain an approved design and independent ARC approval before implementing any Terraform type. Approval must cover schema, state, remote identity, import behavior, and the deferred product-test boundary. Approval defines later product evidence; it does not move tests ahead of the main product corpus.
- Use the official, repository-locked `developers.nutanix.com` artifacts as the primary API evidence.
- Before implementing each Nutanix operation, query `nutanix-mcp` with its
  exact operation identifier and versioned path. Record the matching MCP
  document or chunk, the exact-path result, and whether the condensed MCP
  artifact indexes the operation identifier. A matching versioned Swagger
  document must contain the exact path, while the locked OpenAPI must bind the
  operation identifier to that path. A missing or conflicting MCP path blocks
  implementation; an unindexed operation identifier is a recorded MCP coverage
  gap and does not replace the locked OpenAPI binding.
- Hand-write transport code, DTOs, Terraform schemas, state models, and lifecycle logic.
- Do not add a Nutanix SDK as a runtime dependency or use OpenAPI code generation.
- Implement the main product corpus before adding product tests.
- Add tests only for Nutanix product behavior: Terraform schema and lifecycle,
  API mapping, state, import, and authorized product acceptance. Do not add
  tests for repository automation, launchers, Beads, documentation, roadmap,
  CI wiring, or policy scripts.
- Existing non-product tests are frozen and excluded from the default delivery
  gate. Do not expand or repeatedly run them while implementing the product.
- Before writing product code, document the package owner, consumer-side
  interfaces, call graph, data ownership, null semantics, and error propagation
  for the affected functions. Reconcile the design with existing code and
  obtain the required ARC approval before implementation continues.

## Execution boundary

- Run all Go, Terraform, Task, and Beads work, plus Python CLI checks, linting,
  generation, and packaging, inside Podman through `./dev`.
- Limit the host to normal Git/GitHub operations, the Podman control plane, and the pinned `uv` bootstrap/control plane.
- Do not use host toolchains as completion evidence.

## CI/CD automation

- GitHub Actions is the current pull-request gate and invokes repository work
  only through `./dev`.
- If GitLab CI/CD wrappers are added, they consist only of small role-oriented
  Python CLI modules.
- Each CLI has one responsibility, explicit arguments and environment inputs,
  deterministic exit codes, and concise machine-readable output.
- Do not build a general Python automation framework or a CI test harness.

### Temporary bootstrap exception

Until Task 2 creates `./dev` and Task 3 initializes Beads, follow the approved
foundation plan using only normal host Git/GitHub operations, Podman
control-plane operations, and pinned `uv` bootstrap operations. Do not run
nonexistent `./dev` or Beads commands. This exception does not permit host Go,
Terraform, Task, tests, linters, generators, packaging, or live-system work.
It expires automatically once both the launcher and tracker exist.

## Safety and evidence

- Do not mutate a live system without an explicit acceptance contract and explicit authorization.
- Do not commit credentials, Nutanix endpoints, Terraform state, environment identifiers, or secret-bearing fixtures.
- Record command or runtime evidence before claiming completion or closing work.
- Keep repository instructions portable: never add host-specific paths or secrets.
