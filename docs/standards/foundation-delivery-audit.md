# Foundation Delivery Audit

This audit reconciles the original repository objective with the actual local
and GitHub state on 2026-08-06. Beads remains the canonical task tracker; this
document is an evidence projection, not a second task list.

## Requirement matrix

| Requirement | Verdict | Evidence |
| --- | --- | --- |
| Create `ioplane/terraform-provider-nutanix` on GitHub | Proved | The [repository API receipt](#github-api-receipts) binds identity, visibility, licence, merge controls, default branch, and security settings to the repository URL. |
| Use a PM+DEV delivery method | Proved | The [foundation design](../superpowers/specs/2026-08-04-foundation-design.md) defines responsibilities and phase gates; the [repository contract](../../AGENTS.md) enforces one sprint, branch, worktree, pull request, and critical-path Beads task. |
| Track work locally with Beads | Proved | Beads 1.1.2 is pinned in the [toolbox](../../deployments/containers/Containerfile.dev); reconciliation task `ntnx-m2.14` is closed and approval-only publication task `ntnx-m2.13` is the sole current item in the [tracker projection](../../.beads/issues.jsonl); remote `refs/dolt/data` resolves to `4358aa1782679d9891a51fe7ddf7745ad17a7e33`. |
| Reuse useful PowerDNS automation patterns | Proved | The [foundation design lineage](../superpowers/specs/2026-08-04-foundation-design.md#9-automation-borrowed-and-corrected-from-powerdns) pins both source repositories by exact commit and records the Nutanix-specific corrections. |
| Use Terraform Plugin Framework with protocol 6 | Proved | [`go.mod`](../../go.mod) pins current releases [Framework 1.19.0](https://github.com/hashicorp/terraform-plugin-framework/releases/tag/v1.19.0) and [plugin-go 0.31.0](https://github.com/hashicorp/terraform-plugin-go/releases/tag/v0.31.0). The [entrypoint](../../cmd/terraform-provider-nutanix/main.go) passes the registry address, debug mode, and explicit `ProtocolVersion: 6` to `providerserver.Serve`; the [pinned Framework source](https://github.com/hashicorp/terraform-plugin-framework/blob/v1.19.0/providerserver/serve_opts.go) defines protocol 6 as the default and supported selection. |
| Run development through Podman | Proved | The [toolbox](../../deployments/containers/Containerfile.dev) uses digest-pinned `golang:1.26-trixie`; [`pyproject.toml`](../../pyproject.toml) pins podman-py 5.8.0 and podman-compose 1.6.0; the [Compose service](../../deployments/compose/compose.dev.yml) has no Podman socket; and [`Taskfile.yml`](../../Taskfile.yml) enforces the container guard. |
| Verify local and stacked delivery state | Proved | Full `./dev task all` receipts are recorded in closed Beads tasks `ntnx-m2.12` and `ntnx-m2.14`; the latter includes the final Nutanix API reconciliation and independent reviews. The [M0 and M1 check receipts](#github-api-receipts) bind successful `Foundation` jobs to exact sprint-head commits, while the protection receipt binds the required strict check and review controls. |
| Publish the complete provider foundation to `main` | Stack published; review and merge remain | The [stacked PR receipts](#github-api-receipts) show `main` at bootstrap commit `3fb8b05340cda7c6db92d75e9ad878284475eeb1` and M0, M1, and M2 in draft PRs with successful `Foundation` checks. Merge still requires protected review and separate authorization. |
| Publish a release | Deferred by contract | The [foundation design](../superpowers/specs/2026-08-04-foundation-design.md#13-security-boundaries) records packaging shape only; product evidence, package acceptance, and release publication belong to a later approved phase. |

## GitHub API receipts

The following read-only receipts were refreshed with `gh api` on 2026-08-06:

- [`GET /repos/ioplane/terraform-provider-nutanix`](https://api.github.com/repos/ioplane/terraform-provider-nutanix)
  returned public visibility, Apache-2.0, default `main`, squash-only merges,
  automatic branch deletion, secret scanning, and push protection.
- [Draft PR 1](https://github.com/ioplane/terraform-provider-nutanix/pull/1)
  binds `main@3fb8b05340cda7c6db92d75e9ad878284475eeb1` to
  `sprint/m0-foundation@5ffdfd66c7c28fdfa48551839be763332e6130d4`.
- [Draft PR 2](https://github.com/ioplane/terraform-provider-nutanix/pull/2)
  binds that M0 head to
  `sprint/m1-kernel@9d06e3115108786cb4f56cb0bf86a95eb5b66094`.
- [Draft PR 3](https://github.com/ioplane/terraform-provider-nutanix/pull/3)
  binds that M1 head to M2 product commit
  `2e8b7af4a98028913741516d9342dc6de1c5b54b` on
  `sprint/m2-readonly`.
- M0 [Foundation job 92230155795](https://github.com/ioplane/terraform-provider-nutanix/actions/runs/30982623053/job/92230155795)
  completed successfully for `5ffdfd66c7c28fdfa48551839be763332e6130d4`;
  M1 [Foundation job 92385923310](https://github.com/ioplane/terraform-provider-nutanix/actions/runs/31029390351/job/92385923310)
  completed successfully for `9d06e3115108786cb4f56cb0bf86a95eb5b66094`;
  M2 [Foundation job 92489596957](https://github.com/ioplane/terraform-provider-nutanix/actions/runs/31061285455/job/92489596957)
  completed successfully for `2e8b7af4a98028913741516d9342dc6de1c5b54b`.
- [`GET /branches/main/protection`](https://api.github.com/repos/ioplane/terraform-provider-nutanix/branches/main/protection)
  returned strict required check `Foundation`, one required code-owner review,
  stale-review dismissal, administrator enforcement, and required conversation
  resolution.
- `origin/sprint/m2-readonly` and draft PR 3 now provide the remote M2
  delivery receipt recorded in the [M2 delivery
  manifest](m2-delivery-manifest.md).

## Automation provenance

The PowerDNS comparison is reproducible from two immutable GitHub trees:

- [`dantte-lp/powerdns-upstream-patches@051c654277ee4cc4e6102b5a392854b4004ad392`](https://github.com/dantte-lp/powerdns-upstream-patches/tree/051c654277ee4cc4e6102b5a392854b4004ad392)
- [`ioplane/terraform-provider-powerdns@9ef6fb0ba8ed4449c839639d1ba659771a812d8d`](https://github.com/ioplane/terraform-provider-powerdns/tree/9ef6fb0ba8ed4449c839639d1ba659771a812d8d)

The adopted patterns are the in-container Task command index, isolated Compose
projects per worktree, bounded shell-free subprocesses, a single tool-version
registry, immutable OCI and Action references, role-oriented Python CLI
modules, generated-document drift checks, and a reproducible package shape.

The Nutanix implementation intentionally differs where its contract requires a
single `./dev` launcher, a pinned uv environment for podman-py and
podman-compose, Beads dependency state, no recursive cleanup, and product-first
testing. `verify` therefore remains an alias for the lightweight `all` gate;
`package:test` and release publication are outside the current sprint.

The static pin checker proves that references match the repository's immutable
allowlist and syntax. Runtime resolution has separate evidence: `./dev up`
successfully built the pinned base image, and GitHub Actions successfully
resolved its pinned Actions for both published sprint heads.

## Verified execution boundary

The active development container reported healthy and the pinned toolbox
reported Go 1.26.5, gopls 0.23.0, Terraform 1.15.8, Task 3.52.0, Beads 1.1.2,
podman-py 5.8.0, and podman-compose 1.6.0. All completion evidence is produced
inside that toolbox through `./dev`; the host is limited to Git, GitHub, Podman
control-plane, and pinned uv bootstrap operations.

The vendor checkout was preserved under the `terraform-provider-nutanix-upstream`
name, while the standalone ioplane checkout was promoted to the canonical
`terraform-provider-nutanix` name. Git repaired every linked worktree in both
directions, and the healthy replacement development container binds the
canonical common Git directory.

No product test, package test, live PE/PC product API call, or live-system
mutation is authorized by this audit. Read-only Developer Portal artifact and
MCP evidence calls are part of the documented verification gate. The user
authorized and the project completed the M2 commit, push, and draft pull
request. Merge and release publication remain unauthorized. The exact M2
delivery is defined by the [M2 atomic delivery
manifest](m2-delivery-manifest.md).

## MCP evidence boundary

A fresh `nutanix-mcp` read-back found the exact versioned paths for the four
implemented read operations in the `clustermgmt` v4.2, `prism` v4.3, `vmm`
v4.2, and `networking` v4.3 Swagger documents. The MCP service reported
healthy, but its most recent ingest status was `partial`, and condensed search
does not reliably expose every operation identifier. Consequently, MCP path
evidence supplements but never replaces the repository-locked Developer Portal
OpenAPI operation-ID binding required by the product contract.
