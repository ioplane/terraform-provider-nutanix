# Nutanix reverse-engineering evidence

## Scope and provenance

The initial 2026-08-05 audit found the extraction only as untracked material in
a local checkout without a configured remote. That observation remains part
of the evidence timeline, but it is no longer the current provenance state.
Commit `740198d8cfb3274695a79411d44e2282b4513e15` subsequently added 54
extraction files to `ioplane/nutanix-re`. The previously audited checkout was
at `6ada1284fcfb8b002b68cf835c2c25988cf71f40`; a fresh 2026-08-06 read-back is
clean, tracks `origin/master`, and resolves to
`0446ef20344cda3f83068a78342c176c393ff37f`. The AOS 7.6 consolidated artifact
at that current commit still has the exact digest recorded below. Comparisons
bind to the adding commit and verified artifact bytes rather than mutable
worktree state.

The tracked AOS 7.6 extraction records:

- source package: `ntnx_vmm_py_client` `17.6.0-19150-RELEASE`;
- API prefix: `/api/vmm/v4.3`;
- extracted surface: 192 operations across 135 paths;
- consolidated artifact SHA-256:
  `acbd32ea7f4e3ad61c98c6a4443f387bd8f8fa7b04598afd41932d279c460ce0`.

The consolidated document identifies itself as derived from a decompiled
Python client. Its 232 schemas are placeholders rather than extracted model
definitions, and the document fails OpenAPI validation because operation IDs
are duplicated between AHV and ESXi statistics APIs. It is therefore useful
for route discovery and version comparison only.

The PC corpus has the same limitation. Eight of the nine consolidated files
are structurally valid OpenAPI 3.0 documents, but all extracted schemas are
placeholders. Structural validity therefore does not promote them above the
locked Developer Portal contract.

| Shipped corpus | Paths | Operations | Placeholder schemas | SHA-256 |
| --- | ---: | ---: | ---: | --- |
| PC 7.5.1.6 IAM | 48 | 79 | 97 | `7135b0d19a4a17b18a3d8c2c267b4f29acb6440bd5d303471da439d810170946` |
| PC 7.5.1.6 LCM | 37 | 46 | 62 | `3c8660440f466c695a8a46d7977318b245f9c98121aac2ed1a6b7ffb97b86d6f` |
| PC 7.5.1.6 Licensing | 19 | 24 | 30 | `1cdec783cf3b5e9e835347c4719c8d763e1929ff17353f3948955dbfb452f136` |
| PC 7.5.1.6 Lifecycle | 48 | 64 | 83 | `22b31b384c99d81e781c7c9bdc33037b3b775047054ba02b1627dbbf4b97e0c5` |
| PC 7.5.1.6 Prism | 24 | 28 | 37 | `3b22b08ff69b7fb971573f812a2177f947d234db10a051cbbed9778e1eb96c0c` |
| PC 7.6 IAM | 48 | 79 | 97 | `ce1c53435fbb6288bd4089085de2e5ac9a1728d4f2a5f98074f77fb9d6d3e47d` |
| PC 7.6 Licensing | 19 | 24 | 30 | `4c45913189b736ca7321c9e8f64b7b3e0e7e427c3d99450ab69b8c578709b94d` |
| PC 7.6 Prism | 24 | 28 | 37 | `b3feca16f874e1f2cd1e7fe963ebfe44c8969e8d8875742c00d95dbc0efa479f` |
| AOS 7.6 VMM | 135 | 192 | 232 | `acbd32ea7f4e3ad61c98c6a4443f387bd8f8fa7b04598afd41932d279c460ce0` |

IAM, Licensing, and Prism have different source-document hashes between PC
7.5.1.6 and PC 7.6, but their normalized method, path, operation-ID,
parameter-name, and response-code inventories are equal. This is shipped
operation parity, not binary document or response-schema equality.

## IAM PC 7.5.1.6 to PC 7.6 parity

The local PC 7.5.1.6 and PC 7.6 IAM extractions each contain 79 operations.
After removing source-location metadata, their method, path, operation ID,
parameter-name, and response-code tuples are byte-equivalent; the normalized
inventory SHA-256 is
`5e07542a29db83f0c8b90b43eab12e7d04ed33f928fba9056a8379dcb88464b1`.
The consolidated source-document hashes are:

- PC 7.5.1.6:
  `7135b0d19a4a17b18a3d8c2c267b4f29acb6440bd5d303471da439d810170946`;
- PC 7.6:
  `ce1c53435fbb6288bd4089085de2e5ac9a1728d4f2a5f98074f77fb9d6d3e47d`.

Both shipped clients expose:

| Operation | Method and path | Query inputs | Shipped response |
| --- | --- | --- | --- |
| `list_operations` | `GET /api/iam/v4.0/authz/operations` | page, limit, filter, order-by, select | 200 |
| `list_roles` | `GET /api/iam/v4.0/authz/roles` | page, limit, filter, order-by, select | 200 |

The locked Developer Portal document independently binds these paths to
`listOperations` and `listRoles`, adds structured 4XX and 5XX responses,
declares PC 2024.3 for on-premises and cloud deployments, and permits Basic or
`X-ntnx-api-key` authentication. Its SHA-256 remains the authoritative
`596c9c11d6db7f3005616fad3f32f1fc292178e23012db9509f0fdcc5cf6d6b3`.

This establishes shipped-client parity but does not clear the provider gate:
the current MCP corpus does not index either exact IAM path. Beads
`ntnx-m2.9` owns the sanitized MCP evidence handoff; `ntnx-m2.7` remains
blocked until a fresh MCP query returns both paths.

## Product version reconciliation

All comparisons remove only the common `/api` prefix and the namespace-local
version segment. They do not infer DTO or state compatibility.

### Licensing v4.0 to v4.3

The shipped v4.0 client has 24 operations across 19 paths. The current locked
Developer Portal v4.3 artifact has 19 operations across 17 paths. Sixteen
method-and-path pairs are shared and their operation IDs agree after normal
snake-case to camel-case normalization.

The shipped-only v4.0 routes cover portal settings, trials, EULA replacement,
license-state reset, and trial creation or disablement. The official v4.3
contract instead adds license-token reclaim and license-key association or
reclaim operations. These are product-version changes, not missing provider
routes. Future Licensing work must start from v4.3 and must not carry forward
the eight shipped-only v4.0 operations without a new official and exact-MCP
contract.

### Prism v4.0.b1 to v4.3

The shipped beta client has 28 operations across 24 paths; the current Portal
v4.3 artifact has 37 operations across 28 paths. Eleven method-and-path pairs
are shared, 17 are beta-only, and 26 are v4.3-only. The beta-only families
include protect-PC, resource-manager, trust, and category-association routes.
The v4.3-only families include management-domain managers, backup targets,
restore sources and points, registrations, task jobs and affected entities,
and external storage.

Five shared operations were renamed in the official contract:

| Method and resource | Shipped beta operation | Portal v4.3 operation |
| --- | --- | --- |
| DELETE category | `delete_category_by_ext_id` | `deleteCategoryById` |
| GET categories | `get_all_categories` | `listCategories` |
| GET category | `get_category_by_ext_id` | `getCategoryById` |
| GET batches | `get_batches` | `listBatches` |
| PUT category | `update_category` | `updateCategoryById` |

The provider's current categories reader already follows `listCategories` and
the v4.3 path. The beta corpus is migration evidence only.

### Lifecycle v4.2 and LCM v4.0.a2

The shipped Lifecycle client has 64 operations across 48 paths. All 29
method-and-path pairs in the locked Portal Lifecycle v4.2 document occur in
that client and their normalized operation IDs agree. Its remaining 35 routes
cover service-manager, configuration, deploy-artifact, and publish families
that are absent from the official Portal contract and have placeholder
schemas. They remain candidates, not provider operations.

The separate shipped `lcm` namespace uses preview version `v4.0.a2`, with 46
operations across 37 paths. It is not an alias for the official `lifecycle`
v4.2 namespace: even after forcing that namespace substitution, only 17
method-and-path pairs overlap. The LCM corpus contains old/history/repository
and legacy-action routes, while Portal Lifecycle contains selection, summary,
preload, and export routes absent from LCM. The provider must keep these
namespaces separate and must not merge the preview LCM surface into Lifecycle.

## MCP corroboration on 2026-08-06

| Query | MCP result | Decision |
| --- | --- | --- |
| `listLicenses GET /licensing/v4.3/config/licenses` | `api-swagger-licensing-v4.3-all`, API Specification chunk `485638` and Endpoints chunk `485637`, exact current path present | Licensing v4.3 is eligible for later contract design. |
| `GET /licensing/v4.0/config/trials` | No exact v4.0 path | Do not revive the shipped-only trial route. |
| `listCategories GET /prism/v4.3/config/categories` | `api-swagger-prism-v4.3-all`, Endpoints chunk `485666`, exact current path present | Retain the current v4.3 reader. |
| `GET /prism/v4.0.b1/protectpc/get-replicas` | No exact beta path | Do not expose protect-PC from RE evidence. |
| `listEntities GET /lifecycle/v4.2/resources/entities` | No exact Lifecycle API result | Lifecycle implementation remains blocked on MCP ingestion. |
| `GET /lifecycle/v4.2/svcmgr/applications` | No exact API result | Shipped-only service-manager routes stay excluded. |
| `GET /lcm/v4.0.a2/resources/bundles` | No exact API result | Preview LCM stays outside the contract. |

MCP search evidence is a gate, not a replacement for the locked Portal file.
Lifecycle's complete shipped overlap strengthens availability evidence but
does not waive the missing exact-MCP binding. The live MCP health query reports
7,343 documents and 489,531 chunks, but its latest recorded ingestion remains
the partial run that ended on 2026-05-01. A healthy query endpoint is not proof
of a complete corpus refresh.

The 2026-08-06 pre-implementation revalidation repeated both exact IAM
queries. `listRoles GET /iam/v4.0/authz/roles` returned KB and unrelated guide
matches, while `listOperations GET /iam/v4.0/authz/operations` returned KB,
release-note, and unrelated command-reference matches. Neither result bound
the operation to an IAM API document, and `list_documents(product="iam")`
returned zero documents. This is explicit negative gate evidence: semantic
similarity to IAM text cannot substitute for the required operation section.

## Authentication RE update

`nutanix-re` commit `6ada1284fcfb8b002b68cf835c2c25988cf71f40`
added decompiled authentication-chain evidence that remains present at the
current verified repository commit. It covers certificate-derived,
Basic, service-token, and upstream bearer handling. It is useful for later
threat modeling and authentication research, but it does not provide a locked
Developer Portal security scheme plus exact MCP operation for a public
provider mode. Internal endpoints, environment identifiers, and speculative
token or key-discovery contracts are intentionally omitted here.

The provider authentication contract therefore remains unchanged: Basic and
API key only. Bearer/OIDC, certificate, and service-token modes each require a
separate authoritative contract and review before implementation.

## Sanitized IAM MCP ingestion handoff

The deterministic ingestion source is the official locked Developer Portal
document, not the local RE extraction:

- source URL:
  `https://developers.nutanix.com/api/v1/namespaces/iam/versions/v4.0/yaml`;
- expected SHA-256:
  `596c9c11d6db7f3005616fad3f32f1fc292178e23012db9509f0fdcc5cf6d6b3`;
- target MCP document ID and slug: `api-swagger-iam-v4.0-all`;
- target title: `Nutanix Identity and Access Management APIs`;
- target category: `api`;
- target namespace and version: `iam`, `v4.0`.

The MCP transformation must retain an API Specification section and a
searchable Endpoints section. It must preserve method, exact versioned path,
operation ID, query parameters, response status families, security schemes,
permissions, deployment list, and supported-product versions. It may produce
an additional condensed index, but that index cannot replace the full locked
binding or silently rename operations.

GitHub API inspection identified the version-controlled corpus producer as
private repository `dantte-lp/nutanix-docs-mcp` at commit
`b24e46b8906c10d9cfaa1a6736a5cf031ea02377`. Its API conversion lives in blob
`6cc9f31f7eee009b7fcb7d712724defb376c18ef` at
`src/nutanix_docs_mcp/ingest.py`. That converter creates a complete endpoint
list but includes only the first 10,000 characters of raw YAML in its API
Specification section. In the 748,189-byte locked IAM document,
`listOperations` starts at byte 242,598 and `listRoles` at byte 256,956. The
current transformation therefore cannot retain either operation ID or its
contract in the raw-YAML section; only the path-only endpoint list survives.
That commit remains the canonical remote baseline and cannot satisfy the IAM
operation gate.

An isolated producer worktree on local branch
`feature/iam-operation-sections` now replaces the truncation with parsed,
deterministic API Specification, Endpoints, and per-operation sections. The
renderer preserves method, path, operation ID, tags, descriptions, parameters,
request body, responses, effective security, permissions, deployments, and
supported versions. It rejects malformed paths, duplicate output identities,
unsafe local references, alias cycles, and amplification; external references
remain explicit instead of being fetched. This is a hand-written catalog
transformation, not provider code generation.

The uncommitted producer change passed its format, lint, type, compile, spec,
and code-quality reviews inside the provider's Podman development environment.
No producer tests were added or run. An isolated dry run over the exact locked
IAM artifact produced 31 endpoint sections and 54 operation sections, retained
both `listOperations` and `listRoles` bindings, retained Basic and API-key
security plus permission, deployment, role, and supported-version metadata,
and excluded code samples. The resulting Markdown is 200,876 characters with
SHA-256
`3770d0302313f61a77d7db221fd9ffd1df2188132f6cdbdeb0f8ce02c42fb68d`.
Repeating the render regenerated the same content without leaving temporary
files.

The correction has not been committed or published, and no live MCP catalog
was changed. Consequently the producer qualification is complete only as
local implementation evidence: `ntnx-m2.9` remains in progress and IAM remains
blocked until the reviewed producer revision is published and a completed
isolated catalog ingestion passes the live MCP gates below. Merely adding the
IAM YAML to an ignored local source directory is not reproducible delivery
evidence.

The ingestion payload excludes local paths, live endpoints, credentials,
credential-store references, extracted source locations, and PC-specific
observations. The shipped-client parity hashes above remain comparison
metadata only and are not the MCP source body.

The provider gate passes only when a fresh MCP audit proves all of the
following:

1. `list_documents(category="api")` returns
   `api-swagger-iam-v4.0-all` with the expected namespace and version;
2. an exact search for `listRoles` and `/iam/v4.0/authz/roles` returns that
   document and the exact path;
3. an exact search for `listOperations` and
   `/iam/v4.0/authz/operations` returns that document and the exact path;
4. the full document binds both operation IDs to their respective `GET`
   paths and preserves Basic plus API-key security;
5. the MCP health report records a completed ingestion rather than relying on
   the current partial 2026-05-01 run.

Only after this evidence is recorded in Beads may `ntnx-m2.7` resume. The
provider still uses the repository-locked OpenAPI as wire authority and does
not copy the MCP or RE document into production code.

## VMM v4.2 to v4.3 delta

The shipped AOS 7.6 client contains all 179 method-and-path pairs in the
current Developer Portal VMM v4.2 document plus 13 additions:

- six VM profile operations;
- five guest reboot policy operations;
- transfer files to an AHV guest;
- migrate an image.

The `listImages` route used by `nutanix_images_v2` is present with the same
method and relative path after version normalization. This corroborates that
the operation continues in the shipped v4.3 client, but it does not prove that
the v4.2 and v4.3 response schemas are identical.

## Active-provider decision

The live Developer Portal discovery on 2026-08-06 still selects VMM v4.2 as
the newest GA publication. The current `nutanix-mcp` corpus likewise contains
`api-swagger-vmm-v4.2-all` and returned no v4.3 API document. Therefore:

- `nutanix_images_v2` remains bound to the locked and MCP-corroborated VMM
  v4.2 contract;
- the AOS 7.6 v4.3 artifact is not copied into the provider and is not used as
  a DTO or Terraform-schema source;
- VMM v4.3 becomes an explicit M4 qualification item: ingest authoritative or
  exact shipped evidence into MCP, recover complete schemas, validate the
  document, then perform a reviewed compatibility migration.

This is a fail-closed version decision, not a claim that VMM v4.3 is unusable.
