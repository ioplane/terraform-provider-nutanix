# Nutanix provider handoff review

## Reviewed snapshot

The review uses the GitHub API to pin the unlisted
[handoff Gist](https://gist.github.com/dantte-lp/6650873ed8cee5fb0b199b7fc3d0a1e9)
instead of following mutable local notes. The initial handoff file remains
byte-identical, while the current revision adds two planning inputs:

- Gist ID: `6650873ed8cee5fb0b199b7fc3d0a1e9`;
- current revision: `9034b9fe51973a440428d00a328217d6f0ab3a85`;
- `tf-provider-handoff.md` SHA-256:
  `89493d37c483bfb1e6462e96a53cc8270ff3efd19f8372e5b27cfa799615a2ec`;
- `coverage-vs-official.md` SHA-256:
  `d55adfbc63f281a09aa93f02fb43782a958eb045538ef4d3b34d50669effb138`;
- `live-vs-static-diff.md` SHA-256:
  `43d4d2360bfb95cc1e03d20d09cf189110aad998347e4c525bcd2819590596c8`;
- GitHub metadata: created and last updated on 2026-08-05 UTC;
  `public=false`.

The handoff is secondary evidence. It contains useful shipped-product route
inventory, but it also contains architectural recommendations, live-system
details, and claims that require independent verification. No endpoint,
credential-store path, account name, or other environment identifier from the
handoff is copied into this repository.

## Reconciliation matrix

| Handoff topic | Independent evidence | Provider decision |
| --- | --- | --- |
| Extend the official provider rather than build a new provider | GitHub API shows that `nutanix/terraform-provider-nutanix` still uses the legacy `github.com/terraform-providers/terraform-provider-nutanix` module identity, `go 1.25.3`, and Terraform Plugin SDK v2. Its `v2.4.2` tag points to `8eed0bdf`, while current `master` points to later commit `5d035f75`; the handoff combines those two states and gives the latter an incorrect August date. | Rejected. This repository remains an independent greenfield Plugin Framework provider using Protocol 6. Upstream is compatibility evidence only. |
| Nutanix Go SDK is the golden implementation path | The upstream provider does use generated Nutanix clients. GitHub API also shows current `licensing-go-client` and `volumes-go-client` modules, contrary to the handoff's claims that Licensing has no SDK and Volumes has no separate SDK. | Rejected as an implementation boundary. SDK source and examples may expose discrepancies, but no Nutanix SDK becomes a runtime dependency or generation input. |
| Extracted OpenAPI artifacts are tracked and ready for implementation | At the initial review they were untracked in a checkout without a remote, so the Gist claim was premature for that snapshot. They are now tracked in `ioplane/nutanix-re` by commit `740198d8cfb3274695a79411d44e2282b4513e15`. The VMM document still has placeholder schemas and duplicate operation IDs; the other structurally valid documents also have placeholder schemas. | Pin the tracked commit and use the corpus only as version-identified route and availability evidence. Do not copy the documents or derive DTOs and Terraform schemas from them. |
| API versions can be taken from shipped PC/AOS clients | Live Developer Portal discovery selects independently versioned GA artifacts. Several selected versions differ from the handoff's shipped-image routes. | Preserve one locked version per namespace and never expose one provider-wide API-version switch. |
| HTTP Basic is supported | The selected OpenAPI documents and MCP material declare Basic authentication; official v4 guidance also declares `X-ntnx-api-key`. | Accepted and already implemented as two mutually exclusive modes. |
| Add bearer/OIDC, service-token, and certificate-header modes | Exact MCP searches did not corroborate the claimed v4 OIDC token path or the proposed service headers. Current official v4 guidance found through MCP names Basic and API-key authentication. | Not added to the public contract. Each future mode needs authoritative operation and security-scheme evidence, threat review, and a separate ARC decision. |
| Implement Licensing first as a custom v4.0 client | Developer Portal discovery and MCP select Licensing v4.3, and MCP exposes 17 endpoints, 19 operations, Basic/API-key security, and an official Go sample. The GitHub SDK repository has `licensing-go-client/v4.3.2`. | Licensing is a valid M9 product candidate, but not a v4.0 exception and not an SDK-driven shortcut. Its Terraform surface requires its own hand-written contract and MCP checks. |
| Adopt VMM v4.3 now | The shipped AOS 7.6 client adds 13 method/path pairs, but Developer Portal and MCP still select VMM v4.2 and the extracted v4.3 schemas are incomplete. | Keep current M2 image reads on v4.2. Beads `ntnx-m4.1` owns the blocked v4.3 qualification. |

## Coverage and live-probe reconciliation

The current Gist correctly identifies 18 GA v4 namespaces, but its definition
of a "true gap" means absent from the upstream provider and the RE inventory.
That is not the provider's source gate. Live Developer Portal discovery has a
locked artifact for every GA namespace and one additional Storage preview;
an absent SDK or decompiled client is therefore not an API-contract gap.

| Gist claim | Independent result | Planning decision |
| --- | --- | --- |
| Files, Monitoring, Ops Management, and Multi Domain are true gaps | Portal locks Files v4.0, Monitoring v4.2, Ops Management v4.0, and Multi Domain v4.3. MCP currently has the exact Monitoring document only; representative Files, Ops Management, and Multi Domain exact paths are absent. | Add separate M9 namespace tasks. Monitoring may proceed to operation selection; the other three begin with MCP ingestion. |
| Licensing v4.0 should be the first unique feature | Portal and MCP select Licensing v4.3. The shipped v4.0 delta contains retired routes and placeholder schemas. | Keep `ntnx-m9.1` on v4.3; do not implement from the RE stub. |
| AIOps alpha v2.a1 is ready for a custom client | Portal selects GA v4.0, while MCP currently contains only `api-swagger-aiops-v4.2.b1-all`. | Add a version-reconciliation task; adopt neither the old alpha nor the newer beta by inference. |
| All preview LCM routes failed on the probed PC | This agrees with the independent namespace comparison: preview `lcm` v4.0.a2 is not official `lifecycle` v4.2. | Preserve the split in `ntnx-m9.3`; do not normalize or merge the namespaces. |
| Live IAM and Prism GET probes justify their full extracted surfaces | Read-only probe results establish deployment availability only. They do not prove write semantics, schemas, portability, or current versions, and the IAM exact-operation MCP gate is still absent. | Keep IAM blocked; keep Prism on the Portal v4.3 contract. Never promote all RE operations from a GET probe. |
| Bearer, service-token, and certificate modes should be exposed | The claims remain decompiled or inferred and have no selected Portal security scheme plus exact MCP binding. | Keep the public provider contract at Basic and API key. |
| NKP, NDB, and Move should follow the v4 corpus | MCP returns product guides but no exact API reference for these external planes. Their endpoints, authentication, lifecycle, and task models differ from PC v4. | Decompose them under M10 only after an external-plane contract and authoritative artifact ingestion. |

The live-probe file includes environment addresses, credential-store paths, and
operational access details. None are copied here. Its safe GET observations are
secondary compatibility evidence and cannot waive the repository's no-live-
mutation or exact-MCP rules.

## Version conflicts

| Namespace | Handoff route family | 2026-08-06 Developer Portal selection | Result |
| --- | --- | --- | --- |
| `iam` | `v4.0` | `v4.0` GA | Version agrees; exact role and operation paths remain absent from MCP. |
| `licensing` | `v4.0` | `v4.3` GA | Use the locked v4.3 contract for future design. |
| `lifecycle` | `v4.0` | `v4.2` GA | Use the locked v4.2 contract. |
| `prism` | `v4.0.b1` | `v4.3` GA | Do not regress the current category and task clients. |
| `vmm` | `v4.3` | `v4.2` GA | Keep v4.2 until the separate v4.3 evidence gate passes. |
| `aiops` | `v2.a1` | `v4.0` GA | Treat the alpha route inventory as historical shipped evidence. |

## Effect on the active implementation

No product code changes follow directly from this handoff. The four registered
M2 data sources already use the Portal-locked and exact-MCP-corroborated paths.
The two IAM data sources remain blocked because the MCP corpus still does not
index `/iam/v4.0/authz/roles` or `/iam/v4.0/authz/operations`; secondary
shipped-client parity cannot waive that gate.

The handoff does strengthen three planning decisions:

1. retain namespace-local version constants and operation policies;
2. qualify VMM v4.3 independently rather than performing a global upgrade;
3. give Licensing v4.3 an explicit M9 contract task without moving product
   tests or live-system calls ahead of implementation.

The separately pinned [Nutanix API knowledge-base review](nutanix-api-evidence.md)
reaches the same implementation boundary while adding external-plane discovery
candidates. Its generated prefix and coverage claims do not override this
matrix.

## Evidence links

- [Reviewed Gist](https://gist.github.com/dantte-lp/6650873ed8cee5fb0b199b7fc3d0a1e9)
- [Official provider repository](https://github.com/nutanix/terraform-provider-nutanix)
- [Official Go client repository](https://github.com/nutanix/ntnx-api-golang-clients)
- [Nutanix reverse-engineering evidence](nutanix-re-evidence.md)
- [Nutanix API knowledge-base evidence](nutanix-api-evidence.md)
- [Nutanix artifact standard](nutanix-artifacts.md)
