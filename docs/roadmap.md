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
| IAM | Provisional roles and operations; directories, users, groups, policies, and user keys planned | Roles and operations implemented; MCP verification pending |
| Objects compatibility | Object Store lifecycle compatible with public API constraints | Planned |
| Compatibility surface | Downstream resource, data-source, import, and state compatibility | Planned |
| Segmented Objects | Draft, precheck, and deployment actions | Planned |
| Product expansion | All locked GA v4 namespaces | Planned |
| External planes | Foundation, NDB, Self-Service, NC2, NKP, NDK, NAI, Move, Beam, and Flow Security Central | Planned |

## Phase gates

| Gate | Required evidence |
| --- | --- |
| API | Selected public artifact, exact operation, path, version, schema, and error contract; exact MCP corroboration is required before promotion |
| Terraform | Schema, state, identity, import, lifecycle, null, unknown, and sensitivity contract |
| Architecture | Package ownership and complete function interaction path |
| Implementation | Hand-written code passes the complete Podman static and build gate |
| Product verification | Product-scoped tests and authorized acceptance where required |
| Release | Release PR, protected checks, SemVer tag, archives, checksums, and SBOMs |
