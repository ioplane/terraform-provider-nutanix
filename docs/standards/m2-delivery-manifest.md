# M2 Atomic Delivery Manifest

This document is the delivery projection for `ntnx-m2.13`. Beads remains the
canonical task tracker. The snapshot was reconciled on 2026-08-06 from local
Git worktrees and read-only GitHub API responses.

## Branch lineage

| Milestone | Local branch | HEAD | Remote delivery state |
| --- | --- | --- | --- |
| M0 | `sprint/m0-foundation` | `5ffdfd66c7c28fdfa48551839be763332e6130d4` | Draft PR 1 targets `main` at `3fb8b05340cda7c6db92d75e9ad878284475eeb1` |
| M1 | `sprint/m1-kernel` | `9d06e3115108786cb4f56cb0bf86a95eb5b66094` | Draft PR 2 targets `sprint/m0-foundation` at `5ffdfd66c7c28fdfa48551839be763332e6130d4` |
| M2 | `sprint/m2-readonly` | `9d06e3115108786cb4f56cb0bf86a95eb5b66094` plus the working set below | No GitHub branch or pull request |

The M0 and M1 worktrees are clean. Git ancestry proves `main -> M0 -> M1`.
The M2 branch still points exactly at M1, so every dirty path below belongs to
the unpublished M2 working set; none is a committed M0 or M1 delta.

## Index risk

The snapshot contains 52 unique dirty paths: 25 tracked paths and 27
untracked paths. Only eight paths are currently represented in the index;
all eight also have newer working-tree changes. The index is therefore
not a valid delivery manifest and must not be committed as-is.

The following groups are exhaustive. A future M2 branch publication must
contain all four groups. Groups may be separate reviewed commits in the order
shown, but the pull request is the atomic delivery boundary.

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

## Publication boundary

No staging, commit, push, or pull-request mutation is authorized by this
manifest. Before publication, the complete 52-path set must be reviewed as a
unit, regenerated where applicable, and pass `./dev task all`. Publication
then requires an explicit commit approval, creation of
`origin/sprint/m2-readonly` from the verified local branch, and a draft pull
request targeting `sprint/m1-kernel` at the exact SHA recorded above.

IAM roles and operations remain outside this delivery set until their exact
method/path and operation-ID evidence passes the live `nutanix-mcp` gate.
