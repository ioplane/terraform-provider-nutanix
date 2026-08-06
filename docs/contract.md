# Terraform provider contract

## Identity and compatibility

| Property | Contract |
| --- | --- |
| Terraform Registry address | `ioplane/nutanix` |
| Provider type | `nutanix` |
| Implementation | Terraform Plugin Framework |
| Protocol | Terraform Plugin Protocol 6 |

Released compatibility includes public type names, schema shape, null and unknown behavior,
sensitivity, persisted state meaning, remote identity, import grammar, and state upgrades.

Public Terraform names use `nutanix_<domain>_<noun>` in lower snake case. API versions enter public
names only when compatibility with an established name requires the suffix. Nutanix `ext_id` is the
canonical remote identity when the selected API exposes it; a display name never silently replaces
remote identity.

## Provider configuration

Every attribute is optional in the Terraform schema so the matching environment variable can
supply it. Explicit Terraform values take precedence. Unknown values and explicit empty strings are
invalid during provider configuration.

| Attribute | Type | Sensitive | Environment variable | Default |
| --- | --- | --- | --- | --- |
| `endpoint` | string | No | `NUTANIX_ENDPOINT` | Required |
| `username` | string | No | `NUTANIX_USERNAME` | None |
| `password` | string | Yes | `NUTANIX_PASSWORD` | None |
| `api_key` | string | Yes | `NUTANIX_API_KEY` | None |
| `insecure` | bool | No | `NUTANIX_INSECURE` | `false` |
| `ca_certificate` | string | Yes | `NUTANIX_CA_CERTIFICATE` | System roots |
| `request_timeout_seconds` | integer | No | `NUTANIX_REQUEST_TIMEOUT_SECONDS` | `60` |

Exactly one authentication mode is required:

- Basic authentication uses both `username` and `password`;
- API-key authentication uses only `api_key` and the `X-ntnx-api-key` header.

Partial Basic credentials, both complete modes, or no complete mode are configuration errors. Basic
usernames cannot contain `:`. API keys contain 1 through 4096 visible ASCII bytes and are never
included in diagnostics.

The endpoint must be an HTTPS origin with a host and no user information, query, fragment, or
non-root path. The port is part of the origin. `insecure = true` conflicts with `ca_certificate`.
TLS verification is enabled by default, TLS 1.2 is the minimum, redirects are rejected, and standard
`HTTPS_PROXY` and `NO_PROXY` variables are honored.

Provider configuration is not Terraform state. Endpoint, credentials, CA material, retry tokens,
ETags, task references, and capability results are not persisted.

## Kernel behavior

| Concern | Contract |
| --- | --- |
| Request paths | Relative `/api/...` templates resolved against one configured origin |
| User agent | Bounded provider and Terraform versions; not user-configurable |
| Retryable reads | Transport failures and HTTP `408`, `429`, `502`, `503`, `504` |
| Non-retryable statuses | HTTP `401`, `403`, `404`, `409`, `412`, `428` |
| Attempts | Maximum four attempts with caller cancellation and deadline enforcement |
| Delay | `Retry-After` support, 30-second per-delay and 60-second cumulative ceilings |
| Body limits | 16 MiB success and 1 MiB failure defaults; operation-specific override requires review |
| Pagination | Zero-based with explicit page and item ceilings |
| Task polling | Synchronous and context-bound with fail-closed unknown states |
| Capabilities | Documented read-only probes; authentication and server failures are not unsupported results |

Mutations retry only when their locked operation contract declares `NTNX-Request-Id` idempotence.
One RFC-compatible UUID is reused for every eligible attempt. Once a mutation returns a task
reference, mutation retry ends and task polling begins.

Logs use Terraform `tflog` context and allowlist operation name, method, path template, attempt,
status, duration, and request correlation ID. Raw URLs, queries, headers, bodies, credentials,
object names, and remote identifiers are excluded.

## Read-only product surface

| Terraform data source | Operation | Locked wire path |
| --- | --- | --- |
| `nutanix_clusters_v2` | `listClusters` | `GET /api/clustermgmt/v4.2/config/clusters` |
| `nutanix_categories_v2` | `listCategories` | `GET /api/prism/v4.3/config/categories` |
| `nutanix_images_v2` | `listImages` | `GET /api/vmm/v4.2/content/images` |
| `nutanix_subnet_v2` | `getSubnetById` | `GET /api/networking/v4.3/config/subnets/{extId}` |

List state IDs are lowercase SHA-256 values over the Terraform type and normalized caller-only
query identity. Namespace-added projections and server defaults do not alter identity. The subnet
reader requires a canonical UUID `ext_id`, preserves valid caller spelling in state, and rejects a
response with a semantically different UUID.

Missing collections map to typed Terraform null lists; explicit empty JSON arrays remain empty
lists. Required remote identity fields are validated before state is written. Generated public
schemas are available under [`docs/data-sources/`](data-sources/).

Resources, actions, functions, and ephemeral resources are not registered. IAM role and operation
data sources are not registered until their exact namespace, version, operation ID, and path satisfy
the API evidence gate.

## API evidence gate

Every Terraform type must define:

1. public name and complete schema;
2. state, lifecycle, null, unknown, and sensitivity semantics;
3. remote identity and `ext_id` mapping;
4. import and state-upgrade behavior where applicable;
5. exact locked namespace, version, operation IDs, paths, and response schemas;
6. the complete function path from Framework request to `State.Set`;
7. product verification required after the main product implementation is complete.

The selected Developer Portal artifact is the wire authority. Exact-path API corroboration must
agree with it. Missing or conflicting evidence blocks implementation. Extracted or live evidence
may identify a documentation gap but does not silently replace the locked public contract.

## References

- [Provider architecture](architecture.md)
- [Nutanix artifact standard](standards/nutanix-artifacts.md)
- [Verification standard](standards/testing.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform Plugin Protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
