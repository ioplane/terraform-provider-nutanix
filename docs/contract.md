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
| `nutanix_roles_v2` | `listRoles` | `GET /api/iam/v4.0/authz/roles` |
| `nutanix_operations_v2` | `listOperations` | `GET /api/iam/v4.0/authz/operations` |
| `nutanix_licenses_v2` | `listLicenses` | `GET /api/licensing/v4.3/config/licenses` |
| `nutanix_license_keys_v2` | `listLicenseKeys` | `GET /api/licensing/v4.3/config/license-keys` |
| `nutanix_license_features_v2` | `listFeatures` | `GET /api/licensing/v4.3/config/features` |

## Managed resources

| Terraform resource | Operations | Locked wire paths | State identity |
| --- | --- | --- | --- |
| `nutanix_category` (provisional) | `createCategory`, `getCategoryById`, `updateCategoryById`, `deleteCategoryById` | `POST /api/prism/v4.3/config/categories`; `GET`, `PUT`, `DELETE /api/prism/v4.3/config/categories/{extId}` | Prism `extId` UUID |
| `nutanix_subnet` (provisional) | `createSubnet`, `getSubnetById`, `updateSubnetById`, `deleteSubnetById` | `POST`, `GET`, `PUT`, `DELETE /api/networking/v4.3/config/subnets[/{extId}]` | Networking `extId` UUID |
| `nutanix_storage_container` (provisional) | `createStorageContainer`, `getStorageContainerById`, `updateStorageContainerById`, `deleteStorageContainerById` | `POST`, `GET`, `PUT`, `DELETE /api/clustermgmt/v4.2/config/storage-containers[/{extId}]` | Cluster Management `extId` UUID |
| `nutanix_image_placement_policy` (provisional) | `createPlacementPolicy`, `getPlacementPolicyById`, `updatePlacementPolicyById`, `deletePlacementPolicyById` | `POST`, `GET`, `PUT`, `DELETE /api/vmm/v4.2/images/config/placement-policies[/{extId}]` | VMM `extId` UUID |

List state IDs are lowercase SHA-256 values over the Terraform type and normalized caller-only
query identity. Namespace-added projections and server defaults do not alter identity. The subnet
reader requires a canonical UUID `ext_id`, preserves valid caller spelling in state, and rejects a
response with a semantically different UUID.

Missing collections map to typed Terraform null lists; explicit empty JSON arrays remain empty
lists. Required remote identity fields are validated before state is written. Generated public
schemas are available under [`docs/data-sources/`](data-sources/).

The Licensing data source exposes the applied-license inventory from Licensing v4.3. Its
`expand` query is optional and is passed through only for reviewed OData relationships such as
`consumptionDetails`; expanded cluster consumption is nullable when the relationship is not
requested. The first slice is read-only and does not expose license-key mutation or assignment
actions.

The license-key data source exposes the read-only `listLicenseKeys` inventory. Its selected state
fields are the Portal `licensing.v4.3.config.LicenseKey` base projection; `assignmentDetails` and
`associationDetails` are nullable unless requested through `expand`. Both data sources use the
caller-only normalized OData query identity for deterministic state IDs, and preserve explicit
empty collections separately from absent or null collections.

The license-feature data source exposes the read-only `listFeatures` inventory. Its selected state
fields are the Portal `licensing.v4.3.config.Feature` projection. The API union value is stored as
an exact string and interpreted with `value_type`, because the Terraform Plugin Framework does not
support dynamic types inside list nested attributes. The caller-only query identity includes
`$page`, `$limit`, `$filter`, `$orderby`, and `$select`.

### Licensing v4.3 operation qualification

The direct Portal v4.3 artifact observed on 2026-08-18 contains 17 paths and 19 operations. Nutanix
MCP document `api-swagger-licensing-v4.3-all` independently reports version 4.3.1, 17 endpoints,
and 19 operations. The matrix below qualifies every operation against the current provider scope;
`rejected` means rejected from this read-only provider slice, not that the vendor operation is invalid.

| Operation | Portal v4.3 path | Decision | Evidence-bound reason |
| --- | --- | --- | --- |
| `getEula` | `GET /licensing/v4.3/agreements/eula` | Deferred | Read-only, but the EULA response has no reviewed Terraform identity/state contract |
| `addUser` | `POST /licensing/v4.3/agreements/eula/$actions/add-user` | Rejected | Internal EULA user mutation; outside the current provider scope |
| `listLicenseKeys` | `GET /licensing/v4.3/config/license-keys` | Implemented, demoted | Source implementation exists, but the selected v4.4 manifest does not authorize promotion of the v4.3 wire contract |
| `addLicenseKey` | `POST /licensing/v4.3/config/license-keys` | Rejected | Mutation requires a separate idempotence, dry-run, secret handling, and product gate |
| `getLicenseKeyById` | `GET /licensing/v4.3/config/license-keys/{extId}` | Deferred | Read-only candidate; import, identity, and product acceptance are not approved |
| `deleteLicenseKeyById` | `DELETE /licensing/v4.3/config/license-keys/{extId}` | Rejected | Destructive mutation; no reviewed rollback and product gate |
| `assignLicenseKeys` | `POST /licensing/v4.3/config/$actions/assign-license-keys` | Rejected | Cluster assignment mutation; task/idempotence and product gate are absent |
| `associateLicenseKeys` | `POST /licensing/v4.3/config/license-keys/{extId}/$actions/associate-license-keys` | Rejected | Association mutation; lifecycle and rollback contract are absent |
| `reclaimLicenseKey` | `POST /licensing/v4.3/config/license-keys/{extId}/$actions/reclaim` | Rejected | Reclaim mutation; quantity, task, and rollback contract are absent |
| `listReclaimLicenseTokens` | `GET /licensing/v4.3/config/reclaim-license-tokens` | Deferred | Read-only candidate; token sensitivity and state projection require review |
| `listFeatures` | `GET /licensing/v4.3/config/features` | Implemented, demoted | Source implementation exists, but v4.3 `valueType` is not present in the selected v4.4 schema |
| `listLicenses` | `GET /licensing/v4.3/config/licenses` | Implemented, demoted | Source implementation exists, but the selected v4.4 manifest does not authorize promotion of the v4.3 wire contract |
| `listSettings` | `GET /licensing/v4.3/config/settings` | Deferred | Read-only candidate; setting sensitivity and stable state projection require review |
| `listViolations` | `GET /licensing/v4.3/config/violations` | Deferred | Read-only candidate; nested violation semantics and product acceptance require review |
| `listAllowances` | `GET /licensing/v4.3/config/allowances` | Deferred | Read-only candidate; nested allowance limits and state shape are not approved |
| `listEntitlements` | `GET /licensing/v4.3/config/entitlements` | Deferred | Read-only candidate; cluster identity and nested entitlement state require review |
| `listCompliances` | `GET /licensing/v4.3/config/compliances` | Deferred | Read-only candidate; service compliance state and product acceptance require review |
| `listRecommendations` | `GET /licensing/v4.3/config/recommendations` | Deferred | Read-only candidate; recommendation freshness and state semantics require review |
| `syncLicenseState` | `POST /licensing/v4.3/config/$actions/sync-license-state` | Rejected | State-sync mutation; task, idempotence, and live product gate are absent |

The repository manifest currently locks Licensing v4.4, while the implementation above intentionally
targets v4.3. The v4.4 lock is not silently treated as v4.3 evidence: its artifact has 20 paths and
22 operations and changes selected schemas (for example, v4.4 `Feature` has no `valueType`, while
v4.3 does). A separate version-lock reconciliation must complete before changing the provider paths
or state model. Until `ntnx-c57.3` closes, all three implemented v4.3 data sources are demoted
research/provisional surfaces and are not an accepted compatibility contract for downstream users.

The `nutanix-re` v4.0 finding is discrepancy evidence only. Its `creationDate` and `isDeleted`
fields are absent from the v4.3 `LicenseKey` schema, so they remain explicitly excluded from state.
Its v4.0 portal-setting, trial, and reset routes are not present in the v4.3 Portal operation set
and are treated as version drift rather than implementation candidates.

The category resource manages only user-defined categories. `key` is immutable and forces
replacement; `value`, `description`, and `owner_uuid` are mutable through the conditional PUT.
The API requires `If-Match` for updates, so the client reads the current ETag immediately before
the mutation. `type` is computed and associations remain read-only data-source projections in this
slice. Import accepts a Prism `extId` and the first refresh populates the complete state.

The category resource, IAM role data source, and IAM operation data source are provisional surfaces
from locked artifacts and remain outside the accepted compatibility surface until exact Nutanix MCP
corroboration and product verification are complete. Actions, functions, and ephemeral resources
are not registered.

Networking subnet mutations are asynchronous. The hand-written client accepts only the mutable
`SubnetSpec` projection, requires `NTNX-Request-Id` for replayable `POST`, `PUT`, and `DELETE`,
requires `If-Match` for `PUT`, and validates both the `202` response `Location` header
and the `prism.v4.3.config.TaskReference` body before handing the task ID to the shared waiter.
After a successful wait, the shared Prism task projection exposes bounded `entitiesAffected`
references; subnet identity is accepted only from an entity whose relation is
`networking:config:subnet` and whose `extId` is a valid UUID. Missing, malformed, or duplicate
subnet references fail closed. The Terraform resource is provisional until its complete state
mapping is product-tested against an authorized endpoint; no downstream compatibility claim is
made yet.

Cluster Management storage-container mutations are asynchronous. Create requires the target
`cluster_ext_id` as the reviewed `X-Cluster-Id` header, while update requires the current opaque
`ETag`; all mutations require `NTNX-Request-Id` and validate `202`, `Location`, and the task
reference before polling. The resource does not send read-only projections such as capacity
limits, owner, storage-pool, cluster name, or external-storage identity. `is_shared` is modeled
as create-time immutable and `ignore_small_files` is applied only to delete. Task identity is
accepted only from exactly one `entitiesAffected` reference with relation
`clustermgmt:config:storage-containers` and a valid UUID. Product verification and operation-level
MCP corroboration remain deferred; this is a provisional compatibility surface.

VMM image-placement-policy mutations are asynchronous. Create and update require
`NTNX-Request-Id`; update additionally requires the current `ETag`; delete is asynchronous but
has no request-ID requirement in the selected v4.2 OpenAPI contract. The resource sends only the
mutable name, description, placement type, and bounded category filters. Policy identity is
accepted only from exactly one `entitiesAffected` reference with relation
`vmm:images:config:placement-policy` and a valid UUID. Suspend/resume actions are intentionally
not exposed in this CRUD slice. Product verification and operation-level MCP corroboration remain
deferred; this is a provisional compatibility surface.

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
agree with it. Missing or conflicting evidence blocks promotion to the accepted compatibility
surface. A provisional implementation may proceed when the Portal operation contract and immutable
discovery provenance are complete, but it must carry an explicit evidence debt and cannot be released
or used for downstream compatibility claims until the missing corroboration is resolved.

## References

- [Provider architecture](architecture.md)
- [Nutanix artifact standard](standards/nutanix-artifacts.md)
- [Verification standard](standards/testing.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform Plugin Protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
