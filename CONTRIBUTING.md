# Contributing

The M0 foundation is complete on the current delivery branch and the M1 kernel
is the active phase. Before contributing, read the authoritative
[repository instructions](AGENTS.md), the
[provider contract](docs/contract.md), the
[M1 kernel design](docs/superpowers/specs/2026-08-05-m1-kernel-design.md), and
the [M1 kernel implementation plan](docs/superpowers/plans/2026-08-05-m1-kernel.md).

## Workflow

1. Start from an issue with defined scope and acceptance criteria, then represent the work and its dependencies in Beads. Beads is the canonical task tracker, and only one critical-path task may be in progress.
2. For a Terraform type, obtain design and independent ARC approval for its schema, state, remote identity, import behavior, and test contract before implementation begins.
3. Use one sprint branch, one worktree, and one pull request. Keep the change focused on its issue and Beads task.
4. Work test-first using the red-green-refactor cycle. Run Go, Terraform, Task, Beads, and the required Python quality, generation, and packaging work in Podman through `./dev`.
5. Run the applicable containerized gate and record its evidence. Close Beads work only after its acceptance evidence is attached.
6. Open a pull request that links the issue and Beads task, explains the contract and risks, and includes verification evidence. Resolve review and CI failures before merge.

Use [Conventional Commits](https://www.conventionalcommits.org/) for commit subjects, such as `feat:`, `fix:`, `test:`, `docs:`, `build:`, `ci:`, and `chore:`. Keep commits reviewable and do not mix unrelated changes.

Never commit credentials, endpoints, Terraform state, or secret-bearing fixtures. Live-system mutation requires an explicit acceptance contract and explicit authorization.
