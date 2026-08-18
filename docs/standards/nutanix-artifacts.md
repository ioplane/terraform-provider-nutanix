# Nutanix API artifact standard

## Public source

The [Nutanix Developer Portal](https://developers.nutanix.com/) is the primary machine-readable API
source. Discovery and verification use the public registry endpoints below.

```http
GET /api/v1/namespaces/
GET /api/v1/namespaces/{namespace}/versions/
GET /api/v1/namespaces/{namespace}/versions/{version}/yaml
GET /api/v1/namespaces/{namespace}/versions/{version}/postman-collection
GET /api/v1/namespaces/{namespace}/versions/{version}/locale/en_US/error
```

Requests use `GET`. The artifact gate validates redirect authority, media type, content shape, byte
count, and SHA-256 digest. URLs with user information, query strings, fragments, encoded path bytes,
or traversal segments are rejected.

## Version selection

Nutanix APIs are versioned per namespace. Selection uses the newest GA version matching
`v<major>.<minor>`. A preview is selected only when the namespace publishes no GA version and the
manifest records that status explicitly.

| Namespace | Version | Stability |
| --- | --- | --- |
| `aiops` | `v4.0` | GA |
| `clustermgmt` | `v4.3` | GA |
| `datapolicies` | `v4.3` | GA |
| `dataprotection` | `v4.4` | GA |
| `files` | `v4.0` | GA |
| `iam` | `v4.0` | GA |
| `licensing` | `v4.4` | GA |
| `lifecycle` | `v4.3` | GA |
| `microseg` | `v4.3` | GA |
| `monitoring` | `v4.3` | GA |
| `multidomain` | `v4.3` | GA |
| `networking` | `v4.4` | GA |
| `objects` | `v4.1` | GA |
| `opsmgmt` | `v4.0` | GA |
| `prism` | `v4.4` | GA |
| `security` | `v4.1` | GA |
| `storage` | `v4.0.a3` | Preview; no GA published |
| `tenancy` | `v4.0.a1` | Preview; no GA published |
| `vmm` | `v4.3` | GA |
| `volumes` | `v4.3` | GA |

## Manifest and cache

[`specs/nutanix/manifest.json`](../../specs/nutanix/manifest.json) is the repository lock. Each
artifact records its URL, expected media type, byte count, and SHA-256 digest. The manifest excludes
wall-clock timestamps so unchanged registry data produces byte-identical output.

Downloaded bodies remain under the ignored `.cache/nutanix/artifacts/` directory. Vendor bodies are
not committed and the provider never downloads registry artifacts at runtime.

```bash
./dev task artifacts:discover
./dev task artifacts:update
./dev task artifacts:verify
```

## Evidence precedence

| Priority | Evidence | Allowed use |
| --- | --- | --- |
| 1 | Selected GA OpenAPI, or selected preview when no GA exists | Wire contract |
| 2 | Selected English error reference | Error contract |
| 3 | Selected Postman collection | Request and example corroboration |
| 4 | Official SDK documentation and examples | Comparison only |
| 5 | Version-identified shipped-product artifacts | Availability and discrepancy evidence only |
| 6 | Authorized live PE or PC observation | Explicit documentation gaps only |

OpenAPI, error-reference, and Postman conflicts are recorded and fail closed. Secondary or live
evidence cannot silently replace the selected Developer Portal contract.

## Secondary discovery index

`ioplane/nutanix-api@d68ea9bd88d5b6a630c4ad04041bed7f97978c62` is the pinned breadth and
discrepancy index. It contributes product identities, candidate operations, version signals, and
provenance pointers. It is not a wire-contract source, and every claim inherits the evidence rank
of the underlying Portal, SDK, provider, shipped-product, or reverse-engineering source.

The 2026-08-06 update confirms the public licensing Go SDK v4.3 and records three pc.7.6 runtime
gaps—Projects 2.0, Security Profiles, and Storage Dashboard—that cannot be promoted from offline
protobuf or binary evidence. It also expands Objects Manager and LCM reverse-engineering notes;
those internal RPC inventories remain product research and do not define REST routes.

The pinned machine index contains 31 API families, 2,516 raw rows, and 2,166 distinct JSON records.
Independent validation found 1,435 non-null path fields and 1,081 null paths. A non-null field does
not establish an operation-exact route: the 372 Prism v2 rows reduce to 71 method and path pairs,
and the 350-row Flow Security Central and NAI inventories are byte-identical. Human summaries in
the same snapshot still report 2,521 total operations, 925 v4 operations, and 1,477 concrete paths;
those values conflict with the machine records and are rejected.

The Licensing v4.3 implementation slice is bound to the official artifact fetched from
`/api/v1/namespaces/licensing/versions/v4.3/yaml` on 2026-08-06. The artifact is OpenAPI 3.0.1,
minimum negotiation `v4.3`, and declares Basic and API-key security. It defines 17 paths and 19
operations; the applied-license inventory is `listLicenses`, `GET
/licensing/v4.3/config/licenses`, success `200`, with a `data` array of
`licensing.v4.3.config.License`. Nutanix MCP document `api-swagger-licensing-v4.3-all` independently
reports version `4.3.1`, 17 endpoints, 19 operations, and the same versioned path. The provider
maps only the stable inventory fields and keeps consumption details behind explicit `expand`.

The same artifact defines `listLicenseKeys`, `GET /licensing/v4.3/config/license-keys`, success
`200`, with a `data` array of `licensing.v4.3.config.LicenseKey`. Its caller query contract is
`$page`, `$limit`, `$filter`, `$orderby`, `$expand`, and `$select`; the reviewed expansion names
are `assignmentDetails` and `associationDetails`. The implementation maps the base key fields,
assignment mappings, and key associations by hand. MCP corroboration covers the Licensing v4.3
document and exact endpoint inventory; product acceptance remains deferred under the repository
verification policy.

The implemented read-only slice is `listFeatures`, `GET /licensing/v4.3/config/features`, success `200`,
with a `data` array of `licensing.v4.3.config.Feature`. Its caller query contract is `$page`,
`$limit`, `$filter`, `$orderby`, and `$select`; the operation permission is `View License Features`.
The provider maps `name`, `valueType`, the boolean-or-integer `value`, `licenseType`,
`licenseCategory`, `licenseSubCategory`, and `scope`. The pinned Portal operation block was
reviewed on 2026-08-10; MCP independently corroborates the same Licensing v4.3 endpoint inventory.

The current manifest selection is now Licensing v4.4, not v4.3. The v4.4 locked artifact is the
wire authority for future v4.4 work, but it is not interchangeable with the v4.3 implementation
contract: v4.4 has 20 paths and 22 operations, and its `Feature` schema omits v4.3 `valueType`.
The provider therefore keeps the three v4.3 data sources explicitly provisional until the lock and
implementation version are reconciled. The direct v4.3 Portal artifact observed on 2026-08-18 has
SHA-256 `d5475e4a2ec572d0f87381229160ed2f663cd4fc86d56c57fd75627d7724d0a5`; the locked v4.4 digest
is `77ec78bd2c4b89e2e0f96f8be475b6983167c814e977b3a364804b5507299c2f`.

Secondary evidence is accepted only when all applicable integrity checks pass:

1. immutable repository and commit provenance;
2. machine-readable row, method, path, operation, and source validation;
3. agreement between generated indexes, namespace metadata, and underlying records;
4. agreement with the selected Developer Portal namespace and version;
5. exact-operation corroboration before implementation.

A missing path, missing schema, conflicting total, ambiguous product version, or unavailable
corroboration channel blocks promotion. A REST path is never inferred from a gRPC service or method
name. Extracted protobuf and binary symbols may establish product availability or RPC topology but
do not define HTTP routing, authentication, payloads, errors, lifecycle, or Terraform state.

Nutanix SDKs are not runtime dependencies or code-generation inputs. OpenAPI does not generate
transport code, DTOs, Terraform schemas, state models, or lifecycle logic.

## Operation gate

Every implemented operation must bind:

1. namespace and selected version;
2. exact operation ID, HTTP method, and versioned path;
3. request, success, error, pagination, task, and ETag semantics;
4. the hand-written namespace DTOs and Terraform state mapping;
5. independent exact-path corroboration where available;
6. an explicit provisional marker when MCP corroboration is unavailable.

An operation absent from the selected public lock remains research input and is not an
implementation contract. Placeholder or incomplete schemas never define public Terraform state.
Source-artifact implementations may be provisional when the selected Portal lock contains the
complete operation and schema; missing MCP corroboration blocks acceptance, release, and downstream
compatibility claims until resolved.

## References

- [Nutanix namespace registry](https://developers.nutanix.com/api/v1/namespaces/)
- [Nutanix Developer Portal](https://developers.nutanix.com/)
- [Provider architecture](../architecture.md)
- [Provider contract](../contract.md)
