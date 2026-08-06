# Product roadmap

## Delivery model

Each phase requires an API contract, hand-written implementation, containerized static gate, and
deferred product verification before a stable compatibility claim.

```mermaid
flowchart LR
  F[Foundation] --> K[Provider kernel]
  K --> R[Read-only surface]
  R --> CR[Core resources]
  CR --> CP[Compute and storage]
  CP --> IAM[IAM]
  IAM --> OC[Objects compatibility]
  OC --> CS[Compatibility surface]
  CS --> EX[Product expansion]
  EX --> EP[External planes]
```

## Phases

| Phase | Scope | Status |
| --- | --- | --- |
| Foundation | Reproducible Podman environment, Protocol 6 provider, repository gates | Complete |
| Kernel | Configuration, authentication, transport, retry, pagination, ETags, task and capability services | Complete |
| Read-only surface | Cluster, category, image, and subnet data sources | Implemented; product verification pending |
| Core resources | Categories, projects, subnets, storage containers, policies, and image placement | Planned |
| Compute and storage | Virtual machines, volume groups, affinity, and block storage | Planned |
| IAM | Directories, users, groups, roles, policies, and user keys | Planned |
| Objects compatibility | Object Store lifecycle compatible with public API constraints | Planned |
| Compatibility surface | Downstream resource, data-source, import, and state compatibility | Planned |
| Segmented Objects | Draft, precheck, and deployment actions | Planned |
| Product expansion | All locked GA v4 namespaces | Planned |
| External planes | Foundation, Foundation Central, NDB, Self-Service, NC2, NKP, NDK, NAI, Move, Beam, and Flow Security Central; deprecated NKE compatibility decision only | Planned |

## API research snapshot

The secondary breadth index is pinned to
`ioplane/nutanix-api@d68ea9bd88d5b6a630c4ad04041bed7f97978c62`. It identifies candidates;
the Developer Portal lock and exact-operation corroboration remain the implementation gate.

| Finding | Verified snapshot | Plan decision |
| --- | --- | --- |
| Indexed breadth | 31 API families, 2,516 raw rows, and 2,166 distinct JSON records | Use for coverage and backlog discovery only |
| Non-null path fields | 1,435 records | Treat as syntactic candidates, not operation-exact routes |
| Missing REST paths | 1,081 records | Block operation, payload, state, and lifecycle design |
| Attribution conflicts | Flow Security Central and NAI contain the same 350 records | Re-establish product provenance before qualification |
| Legacy path collapse | 372 Prism v2 rows resolve to 71 method and path pairs | Re-extract before compatibility implementation |
| Portal version agreement | 15 of 18 indexed v4 families | Preserve the repository-selected version per namespace |
| Version conflicts | AIOps, Prism, and VMM | Qualify separately; do not replace a locked version |
| Portal-only namespace | Storage `v4.0.a3` preview | Keep preview status explicit and fail closed |
| Licensing | Index and Portal agree on `v4.3`; 19 candidates | Keep M9 qualification blocked until exact operations pass every gate |
| PC 7.6 extraction | Resource Groups, security, Objects data-plane, alerts, and SaaS signals | Treat binary and protobuf results as discrepancy evidence, not REST contracts |

The source snapshot contains stale human summaries that report 2,521 total operations, 925 v4
operations, and 1,477 concrete paths. The machine records resolve to 2,516, 920, and 1,435 non-null
path fields respectively. Provider planning uses independently computed values and never promotes
a source claim that conflicts with its underlying records.

## Expansion boundaries

| Workstream | Scope boundary |
| --- | --- |
| M3-M5 | Portal-locked core resources, compute, storage, and IAM |
| M6 | Objects v4 control plane; bucket, object, multipart, and federation data-plane contracts remain separate |
| M7 | Existing v2 and v3 state compatibility only; no new legacy-first resource design |
| M8 | Segmented Objects draft, precheck, and deployment actions |
| M9 | Remaining locked v4 namespaces plus separately qualified product-version candidates |
| M10 | Plane-specific adapters inside the monolithic provider; no assumption of Prism v4 compatibility |

NKP remains a Kubernetes-native qualification target with no REST operations in the secondary
index. NDK remains an unresolved product identity and is not inferred to be NKP or NKE. NKE is a
deprecated compatibility decision only.

## Phase gates

| Gate | Required evidence |
| --- | --- |
| API | Selected public artifact, exact operation, path, version, schema, and error contract |
| Terraform | Schema, state, identity, import, lifecycle, null, unknown, and sensitivity contract |
| Architecture | Package ownership and complete function interaction path |
| Implementation | Hand-written code passes the complete Podman static and build gate |
| Product verification | Product-scoped tests and authorized acceptance where required |
| Release | Release PR, protected checks, SemVer tag, archives, checksums, and SBOMs |
