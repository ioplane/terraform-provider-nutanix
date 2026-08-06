# Nutanix API knowledge-base evidence

## Reviewed snapshot

This review binds the private `ioplane/nutanix-api` knowledge base to commit
[`d836fc04bd805c2ad7549115898abd1ceb6f40e5`](https://github.com/ioplane/nutanix-api/tree/d836fc04bd805c2ad7549115898abd1ceb6f40e5).
On 2026-08-06 the clean local `master`, `origin/master`, and GitHub default
branch all resolved to that commit. The evidence timeline after the initial
`19d62225278310119417f1dd7d628e762bc4c251` review is explicit:

- `d423f91e0f10f7365e70727c7f4d2b07d6970f1e` added SDK-derived endpoint
  inventories for the 13 previously empty v4 namespaces;
- `9ee2afe20edf0f133928ca3f53266d3d35803925` added the SDK extractor and CLI;
- the pinned commit added `scripts/build_index.py`, regenerated the index and
  coverage map, and committed the previously pending metadata corrections.

The review followed that repository's read order: `index/all-apis.jsonl`, the
coverage map, v4 authentication, conventions and versioning, and only the
namespace endpoint files relevant to current or planned provider work. No live
PE or PC call was used.

## Authority boundary

`nutanix-api` is a curated discovery index, not an API contract. It helps find
product families, shipped-route candidates, and evidence that needs to be
qualified. It cannot select a provider URL, operation ID, authentication mode,
schema, DTO, or Terraform surface.

| Source | Provider role |
| --- | --- |
| Repository-locked Developer Portal OpenAPI | Binding namespace version, operation ID, parameters, responses, security schemes, and schemas |
| Exact `nutanix-mcp` namespace, version, method, and path result | Mandatory independent operation corroboration |
| Version-identified `nutanix-re` artifact | Shipped availability and route-comparison evidence only |
| `nutanix-api` at the pinned commit | Candidate discovery and product-roadmap breadth only |

When the sources disagree, the provider stays on its locked Portal contract and
opens an ingestion or compatibility task. A knowledge-base claim never clears
an MCP gate.

## Inventory accepted for planning

The index contains 31 rows: 18 v4 GA namespaces, eight external product
families, three SaaS families, and the deprecated Prism v2 and v3 families. It
marks 28 rows GA and three deprecated. This is useful as a breadth cross-check,
not as proof that a provider-facing API is published.

The eight product rows are NDB, NKP, deprecated NKE, NAI, Move, Foundation,
Foundation Central, and Self-Service. The three SaaS rows are NC2, Beam, and
Flow Security Central. M10 retains the non-deprecated rows as qualification
candidates and records NKE only as a deprecated compatibility boundary. NDK
remains an existing provider roadmap candidate but is not corroborated by this
snapshot.

The knowledge base also reinforces two already-approved design choices:

- URL versions are namespace-local and must never be derived from one global
  provider API-version setting;
- external products need plane-specific endpoint, authentication, task, error,
  lifecycle, import, and capability contracts before implementation.

## Version reconciliation

Fourteen of the 18 v4 index prefixes now match the current Portal lock. The
knowledge-base update corrected the Cluster Management and Networking versions
used by M2 and ten other namespace prefixes. Four conflicts remain:

| Namespace | `nutanix-api` prefix | Portal lock | Decision |
| --- | --- | --- | --- |
| `aiops` | v4.2.b1 | v4.0 | Treat the newer SDK beta as a compatibility candidate; do not replace Portal GA by inference. |
| `licensing` | v4.0 | v4.3 | Keep the Portal and MCP v4.3 contract. |
| `prism` | v4.0.b1 | v4.3 | Reject the beta regression; keep the M2 v4.3 contract. |
| `vmm` | v4.3 | v4.2 | Keep M2 on v4.2; qualify v4.3 independently under M4. |

Files, IAM, Objects, Operations Management, Cluster Management, Data Policies,
Data Protection, Lifecycle, Microsegmentation, Monitoring, Multi Domain,
Networking, Security, and Volumes agree at the version-prefix level only. Each
selected operation still needs the exact Portal and MCP binding. The separate
Portal-selected Storage v4.0.a3 preview is absent from the knowledge-base
index.

Fresh exact MCP searches on 2026-08-06 did not promote any of the four
conflicting candidates. They either returned no matching API document or the
current locked version. IAM versions agree, but the exact role and operation
API documents remain absent. The existing fail-closed decisions therefore
remain unchanged.

## Endpoint and authentication reconciliation

All 18 v4 namespaces now have endpoint inventories, totalling 925 candidate
operations. The 387 previously checked-in rows come from shipped Python-client
and RE material; the other 538 are extracted from a local checkout of the
official Go SDK. Their snake-case operation IDs are copied from shipped clients
or derived from Go function names; they are not binding Portal operation IDs.
For example, the beta Prism category candidate is `get_all_categories`, while
the binding v4.3 operation is `listCategories`.

The IAM role and operation paths and the VMM v4.3 image path are useful
discovery candidates. They do not change product code: IAM remains blocked
until MCP indexes its exact API operations, and VMM v4.3 remains a separate
compatibility qualification. The four implemented M2 reads already use the
Portal-locked, exact-MCP-corroborated versions.

The knowledge base's Bearer/OIDC, certificate, service-token, Mercury, and
internal-header descriptions come from reverse engineering and live
observations. They do not provide a selected Portal security scheme plus exact
MCP binding for a public provider mode. The provider therefore retains only
its approved, mutually exclusive Basic and API-key modes.

## Provenance and reproducibility findings

The regenerated v4 index and counts are deterministic from checked-in metadata
and endpoint rows; the builder preserves the external and legacy rows from the
existing index. Source acquisition is not reproducible enough to become
implementation input:

- all 31 index rows and all 925 endpoint rows omit the promised `fetched_at`
  and `fetched_from` fields;
- four source-detail files named by `sources/README.md` are absent;
- `scripts/build_index.py` is committed, but `AGENTS.md` and `README.md` still
  direct consumers to the absent `scripts/build_index.sh`;
- `namespace_writer.py`, named by `scripts/README.md`, is absent;
- the 387 shipped-client rows refer to extraction inputs that are not present
  in the knowledge base, and the corresponding raw client sources are not
  tracked in `nutanix-re`;
- the 538 SDK-derived rows record neither the SDK repository revision nor a
  source file and use generic `auth: any`; their extractor derives operation
  IDs from Go function names rather than retaining a Portal operation binding;
- its bulk refresh filters only Prism legacy rows and would also treat product
  and SaaS rows as v4 namespace directories.

The coverage map is still an audit of the old SDK-based v2.4.2 provider fork.
Its claims that SDK routes are authoritative, SDK coverage eliminates
greenfield work, and an SDK-backed fork addition is straightforward are
incompatible with this provider's hand-written Plugin Framework architecture
and evidence gates. It is not used as a delivery or implementation plan.

## Effect on the provider

No M2 product code changes follow from this review. Accepted information is
limited to roadmap breadth, candidate discovery, and confirmation of
namespace-local versioning. The M9 and M10 plan and Beads graph carry the new
qualification work; implementation still begins with the locked Portal
artifact, exact MCP operation evidence, an approved Terraform contract, and a
reviewed function-interaction design.
