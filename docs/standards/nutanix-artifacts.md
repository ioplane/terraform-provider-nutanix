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
| `clustermgmt` | `v4.2` | GA |
| `datapolicies` | `v4.2` | GA |
| `dataprotection` | `v4.3` | GA |
| `files` | `v4.0` | GA |
| `iam` | `v4.0` | GA |
| `licensing` | `v4.3` | GA |
| `lifecycle` | `v4.2` | GA |
| `microseg` | `v4.2` | GA |
| `monitoring` | `v4.2` | GA |
| `multidomain` | `v4.3` | GA |
| `networking` | `v4.3` | GA |
| `objects` | `v4.0` | GA |
| `opsmgmt` | `v4.0` | GA |
| `prism` | `v4.3` | GA |
| `security` | `v4.1` | GA |
| `storage` | `v4.0.a3` | Preview; no GA published |
| `vmm` | `v4.2` | GA |
| `volumes` | `v4.2` | GA |

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
