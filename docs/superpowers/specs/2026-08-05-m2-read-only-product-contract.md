# M2 Read-only Product Contract

**Status:** Approved by independent ARC review

**Date:** 2026-08-05

**Beads task:** `ntnx-m2.1`

## Outcome

M2 defines the first six hand-written Nutanix product data sources. Its current
implementation increment registers the four operations corroborated by
`nutanix-mcp`, providing the cluster, category, image, and subnet read surface
while the two IAM types remain fail-closed behind their missing exact-path
evidence.

The main product corpus is implemented before its product tests. M2 adds no
test for repository automation, containers, Beads, documentation, CI wiring,
or policy scripts. It performs no live mutation.

## Evidence

Repository-locked Developer Portal OpenAPI artifacts are the normative wire
contract. The official Terraform provider at commit
`8eed0bdf7e74acc6e86199e5b3a353e00cf07101` and the current downstream
repositories are compatibility evidence only. Nutanix MCP documentation is
an operation-by-operation corroboration gate, not a substitute for OpenAPI.

| Terraform data source | Namespace | Locked API | Operation and wire path | OpenAPI SHA-256 |
| --- | --- | --- | --- | --- |
| `nutanix_clusters_v2` | `clustermgmt` | v4.2 GA | `listClusters`, `GET /api/clustermgmt/v4.2/config/clusters` | `ec8ec2382665a1e5432dcea189b3cae074460ef10ce68463621fc9c6e3d9f167` |
| `nutanix_categories_v2` | `prism` | v4.3 GA | `listCategories`, `GET /api/prism/v4.3/config/categories` | `efb04f7aff22e65b3abaf099d3a4cbd113b27d18375f504ea384ed432bb13976` |
| `nutanix_images_v2` | `vmm` | v4.2 GA | `listImages`, `GET /api/vmm/v4.2/content/images` | `e5e6ff8a14a009f7a0a50a6cad0b73011cd0b32cdf40e8b8f7f3e9a30124966a` |
| `nutanix_subnet_v2` | `networking` | v4.3 GA | `getSubnetById`, `GET /api/networking/v4.3/config/subnets/{extId}` | `d80246a462ecade8d39314eab87df38ded35ede57ec898182a15f3eb077878b2` |
| `nutanix_roles_v2` | `iam` | v4.0 GA | `listRoles`, `GET /api/iam/v4.0/authz/roles` | `596c9c11d6db7f3005616fad3f32f1fc292178e23012db9509f0fdcc5cf6d6b3` |
| `nutanix_operations_v2` | `iam` | v4.0 GA | `listOperations`, `GET /api/iam/v4.0/authz/operations` | `596c9c11d6db7f3005616fad3f32f1fc292178e23012db9509f0fdcc5cf6d6b3` |

The 2026-08-05 MCP pass queried each operation with both its exact operation ID
and versioned path. The MCP Swagger documents are condensed endpoint catalogs:
their exact paths are indexed, but the four matched documents do not index
these operation IDs. The locked OpenAPI rows above supply the normative
operation-ID-to-path binding.

| Operation | `nutanix-mcp` document and exact-path result | Operation ID in MCP | Gate result |
| --- | --- | --- | --- |
| `listClusters` | `api-swagger-clustermgmt-v4.2-all` contains `/clustermgmt/v4.2/config/clusters` | not indexed | path corroborated |
| `listCategories` | `api-swagger-prism-v4.3-all` contains `/prism/v4.3/config/categories` | not indexed | path corroborated |
| `listImages` | `api-swagger-vmm-v4.2-all` contains `/vmm/v4.2/content/images` | not indexed | path corroborated |
| `getSubnetById` | `api-swagger-networking-v4.3-all` contains `/networking/v4.3/config/subnets/{extId}` | not indexed | path corroborated |
| `listRoles` | no IAM v4 API document or `/iam/v4.0/authz/roles` path; the API category contains 12 documents and no IAM artifact | not indexed | blocked |
| `listOperations` | no IAM v4 API document or `/iam/v4.0/authz/operations` path; the API category contains 12 documents and no IAM artifact | not indexed | blocked |

The later `nutanix-re` AOS 7.6 extraction contains `list_images` at
`/api/vmm/v4.3/content/images` and 13 VMM operations not present in the locked
v4.2 artifact. It is secondary shipped-client evidence with placeholder
schemas and duplicate operation IDs. Live Developer Portal discovery and MCP
still select VMM v4.2, so this M2 contract does not silently move to v4.3. The
qualification boundary and artifact hash are recorded in the
[reverse-engineering evidence standard](../../standards/nutanix-re-evidence.md)
and Beads task `ntnx-m4.1`.

The IAM searches did find general v4 IAM release-note and RBAC material, but
neither exact endpoint. That material is insufficient for this gate. The four
path-corroborated operations may proceed after this revision is approved; the
two IAM operations remain in the target contract but cannot be implemented
until the MCP corpus supplies their exact paths or the evidence policy is
explicitly revised.

Every operation expects HTTP 200, uses the kernel `read` retry class, sends no
request ID or conditional header, and accepts at most the 16 MiB kernel
success-body limit. Namespace adapters decode the standard `metadata` and
`data` envelope and reject a missing, malformed, or trailing payload without
copying vendor text into diagnostics.

The locked operation support and permission boundary is:

| Operation | Product versions declared by OpenAPI | Deployment | Permission |
| --- | --- | --- | --- |
| `listClusters` | PC 2024.3; PE 6.7 | `ON_PREM`, `CLOUD` | `View Cluster` |
| `listCategories` | PC 2024.3 | not constrained by the operation artifact | `View Category` |
| `listImages` | PC 2024.3 | `ON_PREM`, `CLOUD` | `View Image` |
| `getSubnetById` | PC 2024.3; PE 7.0 | `ON_PREM`, `CLOUD` | `View Subnet` |
| `listRoles` | PC 2024.3 | `ON_PREM`, `CLOUD` | `View Role` |
| `listOperations` | PC 2024.3 | `ON_PREM`, `CLOUD` | `View Operation` |

The table is exact artifact evidence, not a claim that every later product
version is compatible. M2 read-only acceptance is scoped to an on-premises PC
2024.3 alias, where all six operations share one supported plane. A PE alias
may call only an operation whose PE version is independently established.
Types remain registered because provider configuration does not guess the
product behind an HTTPS origin. An unsupported endpoint returns a structured,
fail-closed operation error; M2 neither falls back to another API nor converts
authentication, authorization, throttling, or server errors into
"unsupported".

## Public query contract

List data sources expose the exact query inputs supported by their operation:

| Attribute | Terraform type | Validation | Wire name |
| --- | --- | --- | --- |
| `page` | signed 64-bit integer | optional, 0 through 2147483647 | `$page` |
| `limit` | signed 64-bit integer | optional, 1 through 100 | `$limit` |
| `filter` | string | optional, non-empty when set | `$filter` |
| `order_by` | string | optional, non-empty when set | `$orderby` |
| `select` | string | optional, non-empty when set | `$select` |

`nutanix_clusters_v2` and `nutanix_categories_v2` additionally expose
`expand` as `$expand`. M2 deliberately does not expose cluster `$apply`:
aggregation changes the response away from the contracted Cluster entity
shape and requires a separate aggregate data-source contract.

`select` is a comma-separated set of simple OpenAPI property names. The
adapter rejects expressions and unions the requested set with the fields
required by the public state, sorts it by byte value, and sends the result.
The caller therefore cannot remove `extId` or another contracted field.
Category `expand` is normalized the same way and always includes
`associations` and `detailedAssociations`; cluster `expand` is passed only
after the same token validation. No adapter invents a page or limit default;
omitted values remain omitted so the server applies its documented page 0 and
limit 50 defaults.

The mandatory JSON projections are exact:

| Data source | Fields unioned into `$select` |
| --- | --- |
| `nutanix_clusters_v2` | `extId`, `name`, `categories`, `vmCount`, `inefficientVmCount`, `containerName`, `clusterProfileExtId`, `backupEligibilityScore` |
| `nutanix_categories_v2` | `extId`, `key`, `value`, `type`, `description`, `ownerUuid`, `associations`, `detailedAssociations` |
| `nutanix_images_v2` | `extId`, `name`, `description`, `type`, `checksum`, `sizeBytes`, `categoryExtIds`, `clusterLocationExtIds`, `createTime`, `lastUpdateTime`, `ownerExtId`, `ownerName`, `placementPolicyStatus` |
| `nutanix_roles_v2` | `extId`, `displayName`, `description`, `clientName`, `operations`, `accessibleClients`, `accessibleEntityTypes`, `accessibleClientsCount`, `accessibleEntityTypesCount`, `assignedUsersCount`, `assignedUserGroupsCount`, `createdTime`, `lastUpdatedTime`, `createdBy`, `isSystemDefined` |
| `nutanix_operations_v2` | `extId`, `displayName`, `description`, `entityType`, `operationType`, `clientName`, `relatedOperationList`, `createdTime`, `lastUpdatedTime` |

Each list data source has a computed `id` derived from the operation and the
caller-supplied query. The exact algorithm is lowercase hexadecimal SHA-256
over these bytes:

```text
"ioplane/nutanix/query-id/v1\x00" + terraform_type_name + "\x00" +
url.Values(caller_inputs_with_wire_names).Encode()
```

Only non-null caller inputs are included; integer values use base-10 with no
leading zero, strings retain their exact UTF-8 bytes, and `url.Values.Encode`
sorts and percent-encodes keys and values. Omitted page/limit and explicitly
configured server defaults are intentionally different queries. Internally
added `select` or `expand` fields do not change the identifier, so additive
state projections do not churn it. The 64-character identifier contains no
raw filter, object name, endpoint, or credential. An empty API collection is a
valid empty computed list, not a warning and not a diagnostic error.

## Public result contract

M2 preserves the existing `_v2` public names because current consumers already
depend on them. The suffix is a Terraform compatibility name and does not
select a Nutanix API version.

All data-source attributes are non-sensitive. List query inputs are Optional;
`id` and every result attribute are Computed. The subnet `ext_id` input is
Required. The complete M2 result shapes are below; `int64` means a signed
64-bit Terraform integer and every object member is Computed.

`nutanix_clusters_v2`:

```text
id: string
cluster_entities: list(object({
  ext_id: string, name: string, categories: list(string),
  vm_count: int64, inefficient_vm_count: int64,
  container_name: string, cluster_profile_ext_id: string,
  backup_eligibility_score: int64
}))
```

`nutanix_categories_v2`:

```text
id: string
categories: list(object({
  ext_id: string, key: string, value: string, type: string,
  description: string, owner_uuid: string,
  associations: list(object({
    category_id: string, resource_type: string,
    resource_group: string, count: int64
  })),
  detailed_associations: list(object({
    category_id: string, resource_type: string,
    resource_group: string, resource_id: string
  }))
}))
```

`nutanix_images_v2`:

```text
id: string
images: list(object({
  ext_id: string, name: string, description: string, type: string,
  checksum: list(object({ hex_digest: string })), size_bytes: int64,
  category_ext_ids: list(string), cluster_location_ext_ids: list(string),
  create_time: string, last_update_time: string,
  owner_ext_id: string, owner_name: string,
  placement_policy_status: list(object({
    placement_policy_ext_id: string, compliance_status: string,
    enforcement_mode: string, policy_cluster_ext_ids: list(string),
    enforced_cluster_ext_ids: list(string),
    conflicting_policy_ext_ids: list(string)
  }))
}))
```

`nutanix_subnet_v2`:

```text
id: string
ext_id: string
name: string, description: string, subnet_type: string, network_id: int64
ip_config: list(object({
  ipv4: list(object({
    ip_subnet: list(object({
      ip: list(object({ value: string, prefix_length: int64 })),
      prefix_length: int64
    })),
    default_gateway_ip: list(object({ value: string, prefix_length: int64 })),
    dhcp_server_address: list(object({ value: string, prefix_length: int64 })),
    pool_list: list(object({
      start_ip: list(object({ value: string, prefix_length: int64 })),
      end_ip: list(object({ value: string, prefix_length: int64 }))
    }))
  })),
  ipv6: list(object({
    ip_subnet: list(object({
      ip: list(object({ value: string, prefix_length: int64 })),
      prefix_length: int64
    })),
    default_gateway_ip: list(object({ value: string, prefix_length: int64 })),
    dhcp_server_address: list(object({ value: string, prefix_length: int64 })),
    pool_list: list(object({
      start_ip: list(object({ value: string, prefix_length: int64 })),
      end_ip: list(object({ value: string, prefix_length: int64 }))
    }))
  }))
}))
cluster_reference: string, virtual_switch_reference: string,
vpc_reference: string, is_nat_enabled: bool, is_external: bool,
bridge_name: string, is_advanced_networking: bool
```

This preserves the current downstream path
`ip_config[0].ipv4[0].ip_subnet[0].ip[0].value` and the corresponding prefix
and gateway fields.

`nutanix_roles_v2`:

```text
id: string
roles: list(object({
  ext_id: string, display_name: string, description: string,
  client_name: string, operations: list(string),
  accessible_clients: list(string), accessible_entity_types: list(string),
  accessible_clients_count: int64, accessible_entity_types_count: int64,
  assigned_users_count: int64, assigned_user_groups_count: int64,
  assigned_users_groups_count: int64,
  created_time: string, last_updated_time: string,
  created_by: string, is_system_defined: bool
}))
```

`assigned_users_groups_count` is a computed compatibility alias for the
official provider's misspelling; it always equals the canonical
`assigned_user_groups_count`.

`nutanix_operations_v2`:

```text
id: string
operations: list(object({
  ext_id: string, display_name: string, description: string,
  entity_type: string, operation_type: string, client_name: string,
  related_operation_list: list(string),
  created_time: string, last_updated_time: string
}))
```

`associated_endpoint_list` is not public in M2 because its `endpointUrl` is
authority-bearing state. A future safe projection requires its own contract.

Image source authentication, passwords, certificates, arbitrary links, IAM
endpoint URLs, and other secret-capable or authority-bearing values are
intentionally excluded from state. Public output additions remain additive; a
field is not exposed merely because an SDK model happens to contain it.

At Read time, an unknown input produces an attribute-scoped diagnostic because
the request cannot be constructed. Null optional inputs are omitted. For an
entity field, a missing or JSON-null scalar becomes Terraform null; a missing
or JSON-null nested collection becomes a typed null list; an explicit empty
JSON array becomes an empty list. For a list operation, explicit `data: null`
means an empty result, while a missing `data` member is invalid. Single-subnet
`data: null` is invalid. Every list item must contain a valid `extId`; category
items must also contain `key` and `value`, image items `name` and `type`, and a
subnet response must match the requested identity. Failure rejects the whole
refresh rather than writing partial state.

## State and identity

List data-source state is a point-in-time projection keyed by its deterministic
query ID. Entity identity is always the Nutanix `extId`, exposed as `ext_id`.
The single-subnet data source requires a UUID `ext_id`, preserves the caller's
valid spelling as both `ext_id` and state `id`, and compares the response UUID
semantically rather than by letter case. It rejects a response whose
`data.extId` identifies another subnet.

Data sources have no import grammar, Create, Update, or Delete behavior. The
`nutanix_cluster_v2` import-and-category-management contract is moved to the
foundation-resource phase because it requires a mutation and ETag decision;
it is not hidden inside this read-only milestone.

## Package boundary

```text
internal/provider
  -> internal/service/<domain>/<data-source>
       -> internal/nutanix/<namespace>
            -> internal/transport
```

Namespace packages own hand-written DTOs, exact paths, query encoding,
response validation, and safe errors. Service packages own Terraform schemas,
models, diagnostics, and state mapping. Services receive narrow reader
interfaces through Framework `Configure`; they never receive the raw transport
client. The provider manually constructs and registers the four corroborated
namespace readers now; it may add the two approved IAM readers only after their
MCP gates pass.

### Function interaction contract

Every data-source package declares the reader it consumes at the point of use.
List readers accept `context.Context` plus `odata.ListOptions`; `odata.Build`
validates and copies its reference-bearing fields. They return a
namespace-owned DTO slice, a cloned `url.Values` identity projection, and an
error. The identity projection contains only normalized caller inputs; the
effective wire query remains private to the namespace client. The subnet reader
accepts a validated UUID string and returns one namespace-owned `Subnet`. Each
data source declares a one-reader provider-data accessor;
`configuredProviderData` implements those accessors and returns the concrete
namespace client behind the narrow reader interface. A service never receives
`*transport.Client`.

Namespace clients declare an internal executor interface with only:

```text
Execute(context.Context, transport.Request) (transport.Response, error)
```

For a list operation, the namespace client calls `odata.Build`, passes only
`Query.Values()` into `transport.NewRequest`, executes the request, calls
`apiresponse.DecodeList`, and validates every required identity before
returning the DTOs with the copy from `Query.IdentityValues()`. The data source
passes that returned identity projection to `queryid.New`; it never constructs
the query a second time. Server defaults and namespace-added projections
therefore cannot silently change Terraform identity. `getSubnetById` skips
OData and query identity, builds one escaped path parameter, decodes one
entity, and semantically compares its UUID with the requested UUID.

Terraform request models never cross into namespace packages. Raw response
bytes never cross out of a namespace client. Namespace DTOs never cross into
provider composition or transport. State is constructed only after a complete
validated result exists, and `State.Set` is the single write point.

Input/configuration failures become attribute diagnostics before a reader is
called. Transport and decode failures remain safe wrapped errors until the data
source emits one stable operation diagnostic. No layer logs and returns the
same error, and no diagnostic includes raw vendor text, endpoint, query,
credential, object name, or remote identifier.

The concrete consumer and provider-composition signatures are fixed before
implementation:

| Service package | Reader method | `configuredProviderData` accessor |
| --- | --- | --- |
| `internal/service/cluster` | `ListClusters(context.Context, odata.ListOptions) ([]clustermgmt.Cluster, url.Values, error)` | `ClusterReader() cluster.Reader` |
| `internal/service/category` | `ListCategories(context.Context, odata.ListOptions) ([]prism.Category, url.Values, error)` | `CategoryReader() category.Reader` |
| `internal/service/image` | `ListImages(context.Context, odata.ListOptions) ([]vmm.Image, url.Values, error)` | `ImageReader() image.Reader` |
| `internal/service/subnet` | `GetSubnetByID(context.Context, string) (networking.Subnet, error)` | `SubnetReader() subnet.Reader` |
| `internal/service/role` | `ListRoles(context.Context, odata.ListOptions) ([]iam.Role, url.Values, error)` | `RoleReader() role.Reader` |
| `internal/service/operation` | `ListOperations(context.Context, odata.ListOptions) ([]iam.Operation, url.Values, error)` | `OperationReader() operation.Reader` |

Each service package exports only its `Reader` interface and `NewDataSource`
constructor. Its private `providerData` interface contains exactly the accessor
shown above. `(*dataSource).Configure` type-asserts that one interface and
stores that one reader. Provider `composeProviderData` constructs one namespace
client per corroborated namespace over the shared `*transport.Client`, stores
the concrete clients, and exposes them only through these typed accessors. It
makes no network request. IAM clients and accessors are not added while their
MCP gates are blocked.

The function-by-function `Read` traces are:

| Terraform type | Ordered function trace after `request.Config.Get` |
| --- | --- |
| `nutanix_clusters_v2` | `cluster.optionsFromModel` -> `Reader.ListClusters` -> `queryid.New` -> `cluster.stateFromClusters` -> `response.State.Set` |
| `nutanix_categories_v2` | `category.optionsFromModel` -> `Reader.ListCategories` -> `queryid.New` -> `category.stateFromCategories` -> `response.State.Set` |
| `nutanix_images_v2` | `image.optionsFromModel` -> `Reader.ListImages` -> `queryid.New` -> `image.stateFromImages` -> `response.State.Set` |
| `nutanix_subnet_v2` | `subnet.extIDFromModel` -> `Reader.GetSubnetByID` -> `subnet.stateFromSubnet` -> `response.State.Set` |
| `nutanix_roles_v2` | `role.optionsFromModel` -> `Reader.ListRoles` -> `queryid.New` -> `role.stateFromRoles` -> `response.State.Set` |
| `nutanix_operations_v2` | `operation.optionsFromModel` -> `Reader.ListOperations` -> `queryid.New` -> `operation.stateFromOperations` -> `response.State.Set` |

Each list namespace method has the same explicit internal trace:
`odata.Build` -> `transport.NewRequest` -> `executor.Execute` ->
`apiresponse.DecodeList` -> operation-specific `validate<Item>` -> return DTOs
and cloned identity values. `networking.Client.GetSubnetByID` uses
`transport.NewRequest` -> `executor.Execute` -> `apiresponse.DecodeEntity` ->
`validateSubnetIdentity`.

`optionsFromModel` or `extIDFromModel` emits an attribute-scoped `Invalid
Nutanix <Type> Query` diagnostic for an unknown or invalid request input and
prevents the reader call. A reader, query-ID, or state-mapping failure emits
one `Unable to Read Nutanix <Type>` diagnostic with static safe detail and
prevents `State.Set`. Only successful complete mapping reaches `State.Set`;
its Framework diagnostics are appended directly and are not duplicated or
logged.

### Diagnostic-boundary correction

The implementation review found that the original shared helper enforced only
known Terraform values before calling a reader. Control-character validation
for `filter` and `order_by`, plus projection-token validation for `select` and
`expand`, occurred later in the namespace client's `odata.Build`. That ordering
turned invalid configuration into a generic remote-read diagnostic and did not
satisfy the contract above.

Validation remains owned by `internal/nutanix/odata`; the service layer must
not duplicate OData syntax. The corrected interaction is:

1. `listquery.Options` converts known Terraform primitives into a fresh
   `odata.ListOptions` value.
2. `odata.ValidateOptions` checks page and limit bounds, UTF-8 and
   control-character constraints, and caller projection tokens. On failure it
   returns a typed, redacted `InvalidOptionError` containing only a neutral
   `QueryOption` enum. The enum describes `page`, `limit`, `filter`, order-by,
   select, or expand without importing Terraform names or packages. Error text
   never contains the rejected value, and `Unwrap` preserves
   `errors.Is(err, odata.ErrInvalidListQuery)`.
3. `listquery.Options` exclusively maps the neutral enum to Terraform attribute
   names, including `QueryOptionOrderBy` to `order_by`. It emits the existing
   product-specific attribute diagnostic with static detail saying that the
   value must be known and valid, then returns before the consumer calls its
   `Reader`.
4. `odata.Build` calls `ValidateOptions` again before applying namespace policy
   and mandatory projections. This defense-in-depth check is still required
   for non-Terraform callers and cannot be replaced by service validation.

State conversion also remains fail-closed. Framework value constructors are
internal mapping machinery, so their diagnostics cannot cross the public
service boundary. If a state-mapping function returns any diagnostic, `Read`
discards that diagnostic set, emits exactly its existing static
`Unable to Read Nutanix <Type>` diagnostic, and returns before `State.Set`.
Only diagnostics returned by the final successful `State.Set` call are
appended directly, as already specified.

## Deferred product-test boundary

After the six-type product corpus is structurally complete, product tests may
cover only:

- exact request paths and OData query encoding;
- bounded response and malformed-envelope behavior;
- Terraform schema and state mapping, including empty lists and null fields;
- stable query identity and subnet identity mismatch;
- authorized read-only Protocol 6 acceptance against PC.

No test is added or run as part of the product implementation tasks. Live
acceptance remains a separate, explicitly authorized phase and performs no
mutation.

## References

- [Locked Nutanix artifact policy](../../standards/nutanix-artifacts.md)
- [Terraform provider contract](../../contract.md)
- [Provider architecture](../../architecture.md)
- [Testing standard](../../standards/testing.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
