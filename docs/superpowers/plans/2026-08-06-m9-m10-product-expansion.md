# M9 and M10 Product Expansion Plan

**Goal:** Assign every Portal-locked GA v4 namespace and the first named
external product planes to an evidence-gated delivery phase without importing
the official provider, generated clients, or Nutanix SDKs.

**Beads parents:** `ntnx-m9`, `ntnx-m10`

## Planning boundary

The Developer Portal registry is the authoritative v4 breadth inventory. It
currently selects 18 GA namespaces plus the separate Storage v4.0.a3 preview.
The handoff Gist is secondary shipped and live-availability evidence. The
private `ioplane/nutanix-api` knowledge base at
`d836fc04bd805c2ad7549115898abd1ceb6f40e5` contributes a 31-family and
925-operation discovery index, but its SDK-derived endpoints, generated
versions, auth claims, and coverage conclusions are not contract evidence.
Absence from the official provider, SDK repositories, `nutanix-re`, or that
knowledge base is not a product gap when a locked Portal artifact exists.

Each namespace keeps its own version, transport adapter, hand-written DTOs,
Terraform schema, identity rules, and operation policy. Implementation starts
only after each selected operation is found in `nutanix-mcp` by exact
namespace, version, method, path, and operation ID. Missing or conflicting MCP
evidence creates an ingestion or version-reconciliation task, not an inferred
client. Product tests remain deferred until that product's main corpus exists.

## GA namespace ownership

| Namespace and lock | Delivery owner | Current decision |
| --- | --- | --- |
| `aiops` v4.0 | M9 AIOps | Reconcile Portal GA v4.0 with MCP v4.2.b1 before selecting operations. |
| `clustermgmt` v4.2 | M2 and M3 | Continue the current cluster reader and foundation-resource scope. |
| `datapolicies` v4.2 | M9 Data Policies | Qualify exact operations and design an independent policy lifecycle. |
| `dataprotection` v4.3 | M9 Data Protection | Qualify recovery-point and policy surfaces before Terraform design. |
| `files` v4.0 | M9 Files | Ingest the official artifact into MCP before operation selection. |
| `iam` v4.0 | M2 and M5 | Complete the operation-aware MCP handoff before any additional IAM code. |
| `licensing` v4.3 | M9 Licensing | Keep v4.3; reject the shipped v4.0 stub as a DTO or route source. |
| `lifecycle` v4.2 | M9 Lifecycle | Preserve the boundary from preview `lcm` v4.0.a2 and ingest exact evidence. |
| `microseg` v4.2 | M9 Microsegmentation | Qualify exact operations before defining security-policy state. |
| `monitoring` v4.2 | M9 Monitoring | Exact MCP document exists; proceed with operation and Terraform-surface selection. |
| `multidomain` v4.3 | M9 Multi Domain | Ingest the official artifact into MCP before operation selection. |
| `networking` v4.3 | M2 and M3 | Continue subnet reads and foundation networking resources. |
| `objects` v4.0 | M6 and M8 | Retain the approved compatibility and segmented-Objects phase split. |
| `opsmgmt` v4.0 | M9 Operations Management | Ingest the official artifact into MCP before operation selection. |
| `prism` v4.3 | M2 and M9 Prism | Keep current categories/tasks; qualify management expansion separately. |
| `security` v4.1 | M9 Security | Qualify exact operations and keep it distinct from IAM and Microsegmentation. |
| `vmm` v4.2 | M2 and M4 | Keep v4.2 until the separate v4.3 migration gate passes. |
| `volumes` v4.2 | M4 | Retain the compute and block-storage phase contract. |

Storage v4.0.a3 is not folded into this GA matrix. It requires a separate
preview-adoption decision and cannot be used to expand M4 merely because the
Portal publishes it.

## Gist and MCP reconciliation

- Files, Monitoring, Operations Management, and Multi Domain are not API
  contract gaps: all four have Portal locks. Only Monitoring currently has the
  exact MCP API document, so the other three start with corpus ingestion.
- AIOps is a version conflict, not a newest-version shortcut. Portal GA v4.0,
  MCP v4.2.b1, and the older RE alpha inventory remain separate evidence.
- Failed live preview-LCM probes reinforce the `lcm` versus `lifecycle`
  boundary. They do not select Lifecycle operations or authorize live writes.
- Successful IAM and Prism GET probes prove only availability on one shipped
  deployment. They do not prove write semantics, response schemas, portable
  provider behavior, or a current-version contract.
- No live address, credential-store path, or account identifier from the Gist
  enters the repository or tracker.

## M9 delivery batches

1. Finish the existing Licensing, Prism, and Lifecycle qualification tasks.
2. Qualify Monitoring directly from its locked Portal and exact MCP corpus.
3. Ingest and qualify Files, Operations Management, and Multi Domain.
4. Resolve AIOps version evidence before choosing any operation.
5. Qualify Data Protection, Data Policies, Microsegmentation, and Security as
   distinct product contracts.
6. For each product, approve the Terraform surface and function interaction
   design before implementing the hand-written client and service layer.
7. Add product tests only after that product's main resource and data-source
   corpus is complete.

The ordering is evidence-driven, not a promise to build every namespace in one
release. A namespace may remain blocked without preventing approved earlier
phase work.

## M10 external-plane contract

The external products are not Prism Central v4 namespaces. Current MCP searches
for NKP, NDB, and Move return product or operator guides but no exact
authoritative API reference for their provider-facing operations. The pinned
`nutanix-api` index adds names but not complete provider contracts. M10 therefore
begins with a shared contract inventory, not a generic REST client:

1. identify the authoritative API artifact, product/version matrix, endpoint
   ownership, authentication, TLS, pagination, task, error, and import models;
2. ingest exact operation-level evidence into MCP and record source hashes;
3. approve whether the existing provider block can address the plane or a
   named endpoint configuration is required;
4. implement one plane-specific adapter at a time without SDK or generator
   dependencies;
5. defer its product tests until the selected plane's main corpus exists.

Foundation, Foundation Central, Self-Service, NC2, NDK, NAI, Beam, and Flow
Security Central remain in M10 inventory but need the same authoritative-
artifact contract before implementation. Move is explicit because it has a
distinct migration lifecycle rather than a PC v4 resource model. NKE is
deprecated in the pinned knowledge base and receives a compatibility-boundary
decision only; it is not a new implementation target. NDK remains an existing
roadmap candidate but is absent from that knowledge-base snapshot, so its
identity and source contract require independent qualification.

## Completion evidence

The plan is complete when every GA namespace has one explicit delivery owner,
every new M9 or M10 task names its authoritative artifact and MCP gate, and the
generated roadmap remains a deterministic projection of Beads with exactly one
critical task in progress. This planning work does not clear IAM, mutate PE or
PC, publish the producer, or constitute product-test evidence.
