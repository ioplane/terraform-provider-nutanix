# Repository Instructions

This file is the authoritative working contract for this repository.

## Before work

- Read the current `README.md` and this file before planning or changing the repository.
- Use Beads as canonical task state. Keep exactly one critical-path task in progress, and attach evidence before closing it.
- Use one sprint, one branch, one worktree, and one pull request. Reference the Beads task in commits and the pull request.

## Product contract

- Obtain an approved design and independent ARC approval before implementing any Terraform type. Approval must cover schema, state, remote identity, import behavior, and tests.
- Use the official, repository-locked `developers.nutanix.com` artifacts as the primary API evidence.
- Hand-write transport code, DTOs, Terraform schemas, state models, and lifecycle logic.
- Do not add a Nutanix SDK as a runtime dependency or use OpenAPI code generation.
- Use test-driven development: red, green, refactor.

## Execution boundary

- Run all Go, Terraform, Task, and Beads work, plus Python tests, linting, generation, and packaging, inside Podman through `./dev`.
- Limit the host to normal Git/GitHub operations, the Podman control plane, and the pinned `uv` bootstrap/control plane.
- Do not use host toolchains as completion evidence.

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
