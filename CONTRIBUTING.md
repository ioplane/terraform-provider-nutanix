# Contributing

The M0 foundation and M1 kernel are complete on the current delivery branch;
the M2 read-only product corpus is active. Before contributing, read the authoritative
[repository instructions](AGENTS.md), the
[provider contract](docs/contract.md), the
[M1 kernel design](docs/superpowers/specs/2026-08-05-m1-kernel-design.md), and
the [M1 kernel implementation plan](docs/superpowers/plans/2026-08-05-m1-kernel.md).

## Workflow

1. Start from an issue with defined scope and acceptance criteria, then represent the work and its dependencies in Beads. Beads is the canonical task tracker, and only one critical-path task may be in progress.
2. For a Terraform type, obtain design and independent ARC approval for its schema, state, remote identity, import behavior, and API mapping before implementation begins.
3. Use one sprint branch, one worktree, and one pull request. Keep the change focused on its issue and Beads task.
4. Implement the main product corpus first. Do not add tests for repository or CI tooling. Add Nutanix product tests after the corresponding product implementation exists. Run Go, Terraform, Task, Beads, and Python CLI work in Podman through `./dev`.
5. Run the lightweight implementation gate and record build/static evidence. Product-test evidence becomes required only after the product corpus reaches its planned acceptance phase.
6. Open a pull request that links the issue and Beads task, explains the contract and risks, and includes verification evidence. Resolve review and CI failures before merge.

Use [Conventional Commits](https://www.conventionalcommits.org/) for commit subjects, such as `feat:`, `fix:`, `test:`, `docs:`, `build:`, `ci:`, and `chore:`. Keep commits reviewable and do not mix unrelated changes.

Never commit credentials, endpoints, Terraform state, or secret-bearing fixtures. Live-system mutation requires an explicit acceptance contract and explicit authorization.
