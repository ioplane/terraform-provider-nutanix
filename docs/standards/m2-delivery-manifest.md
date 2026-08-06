# M2 Atomic Delivery Manifest

This document is the delivery projection for `ntnx-m2.13`. Beads remains the
canonical task tracker. The snapshot was reconciled on 2026-08-06 from local
Git worktrees and read-only GitHub API responses.

## Branch lineage

| Milestone | Local branch | Delivery anchor | Remote delivery state |
| --- | --- | --- | --- |
| M0 | `sprint/m0-foundation` | `5ffdfd66c7c28fdfa48551839be763332e6130d4` | Draft PR 1 targets `main` at `3fb8b05340cda7c6db92d75e9ad878284475eeb1` |
| M1 | `sprint/m1-kernel` | `9d06e3115108786cb4f56cb0bf86a95eb5b66094` | Draft PR 2 targets `sprint/m0-foundation` at `5ffdfd66c7c28fdfa48551839be763332e6130d4` |
| M2 | `sprint/m2-readonly` | Product commit `2e8b7af4a98028913741516d9342dc6de1c5b54b` | Draft PR 3 targets `sprint/m1-kernel` at `9d06e3115108786cb4f56cb0bf86a95eb5b66094` |

The M0, M1, and M2 worktrees were clean after the canonical checkout
relocation. Git ancestry proves `main -> M0 -> M1 -> M2`. Product commit
`2e8b7af4a98028913741516d9342dc6de1c5b54b` has the exact M1 commit as its
parent and contains the reviewed product manifest below.

## Delivered manifest

The reviewed snapshot contained 52 unique dirty paths: 25 tracked paths and 27
untracked paths. Before commit, the sorted manifest matched the complete Git
status set and the index exactly, with no unstaged path. `git diff --cached
--check` and `./dev task all` both passed on that final index.

The following groups are exhaustive for product commit
`2e8b7af4a98028913741516d9342dc6de1c5b54b`. The receipt-only follow-up is
restricted to group A projections and does not change the product surface.
The pull request remains the atomic delivery boundary.

## A. Governance, contract, and evidence

This group records the approved behavior, provenance, and tracker projection.

- `.beads/issues.jsonl`
- `AGENTS.md`
- `README.md`
- `docs/architecture.md`
- `docs/contract.md`
- `docs/roadmap.md`
- `docs/standards/dependencies.md`
- `docs/standards/foundation-delivery-audit.md`
- `docs/standards/go-1.26.md`
- `docs/standards/nutanix-artifacts.md`
- `docs/standards/nutanix-api-evidence.md`
- `docs/standards/nutanix-handoff-review.md`
- `docs/standards/nutanix-re-evidence.md`
- `docs/standards/testing.md`
- `docs/standards/m2-delivery-manifest.md`
- `docs/superpowers/plans/2026-08-05-m1-kernel.md`
- `docs/superpowers/plans/2026-08-05-m2-read-only-product.md`
- `docs/superpowers/plans/2026-08-06-m9-m10-product-expansion.md`
- `docs/superpowers/specs/2026-08-04-foundation-design.md`
- `docs/superpowers/specs/2026-08-05-m2-read-only-product-contract.md`

## B. Toolchain and build integration

This group pins and validates the environment required by the product code.
It must precede or accompany group C.

- `Taskfile.yml`
- `deployments/compose/compose.dev.yml`
- `deployments/containers/Containerfile.dev`
- `go.mod`
- `go.sum`
- `pyproject.toml`
- `scripts/checks/docs.py`
- `scripts/checks/docs_links.py`
- `scripts/checks/pins.py`
- `scripts/checks/tool_versions.py`
- `scripts/generate/provider_docs.py`

## C. Provider and product implementation

This group is one build unit. In particular, `internal/provider/provider.go`
imports packages that are currently untracked, so it must never be delivered
without every listed implementation path.

- `internal/provider/provider.go`
- `internal/nutanix/apiresponse/envelope.go`
- `internal/nutanix/clustermgmt/client.go`
- `internal/nutanix/clustermgmt/model.go`
- `internal/nutanix/networking/client.go`
- `internal/nutanix/networking/model.go`
- `internal/nutanix/odata/query.go`
- `internal/nutanix/prism/client.go`
- `internal/nutanix/prism/model.go`
- `internal/nutanix/vmm/client.go`
- `internal/nutanix/vmm/model.go`
- `internal/service/category/data_source.go`
- `internal/service/cluster/data_source.go`
- `internal/service/image/data_source.go`
- `internal/service/listquery/query.go`
- `internal/service/queryid/id.go`
- `internal/service/subnet/data_source.go`

## D. Generated Terraform reference

These pages must be regenerated with the group B generator from the exact
group C provider surface and delivered with the same pull request.

- `docs/data-sources/categories_v2.md`
- `docs/data-sources/clusters_v2.md`
- `docs/data-sources/images_v2.md`
- `docs/data-sources/subnet_v2.md`

## Publication receipt

The user explicitly authorized commit, push, GitHub pull request, and an MR
where a real target exists. The reviewed set was committed as
`2e8b7af4a98028913741516d9342dc6de1c5b54b`, pushed to
`origin/sprint/m2-readonly`, and published as [draft PR
3](https://github.com/ioplane/terraform-provider-nutanix/pull/3). Its base is
the exact M1 SHA recorded above. [Foundation job
92489596957](https://github.com/ioplane/terraform-provider-nutanix/actions/runs/31061285455/job/92489596957)
completed successfully for that product commit.

The standalone `ioplane/terraform-provider-nutanix` checkout now owns the
canonical local repository name. The preserved Nutanix checkout is named
`terraform-provider-nutanix-upstream`, including its pre-existing untracked
`.serena/` data. `git worktree repair` restored all M0, M1, and M2
administrative links, and the recreated healthy development container binds
the canonical Git metadata path.

GitLab inventory found no exact project for this provider. No unrelated
project or new GitLab namespace was used to manufacture an MR receipt. No
merge or release publication is authorized by this delivery.

IAM roles and operations remain outside this delivery set until their exact
method/path and operation-ID evidence passes the live `nutanix-mcp` gate.
