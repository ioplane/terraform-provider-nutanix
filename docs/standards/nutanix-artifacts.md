# Nutanix artifact standard

## Authoritative endpoints

The [Nutanix Developer Portal](https://developers.nutanix.com/) is the primary
machine-readable source for Nutanix API contracts. Artifact discovery and
download use these exact GET endpoints:

```text
GET https://developers.nutanix.com/api/v1/namespaces/
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/<version>/yaml
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/<version>/postman-collection
GET https://developers.nutanix.com/api/v1/namespaces/<namespace>/versions/<version>/locale/en_US/error
```

Discovery, update, and verification use GET, not HEAD. Verification checks the
returned media type, content shape, byte count, and SHA-256 digest rather than
inferring availability from headers.

The 2026-08-04 live lock labels all 19 selected OpenAPI documents and all 19
selected Postman collections as `text/plain; charset=utf-8`; all 19 English
error references use `application/json`. The lock pipeline accepts `text/plain`
for OpenAPI YAML and Postman collections only. A Postman body must still parse
as JSON with an `item` array, and the manifest records the observed
`text/plain` media type. Registry, version, and error-reference documents still
require a JSON media type. This explicit compatibility exception does not apply
to arbitrary artifact kinds.

The pipeline rejects redirects before following them when their target leaves
the exact Developer Portal API prefix. Registry-supplied URLs may not contain
userinfo, query strings, fragments, percent-encoded path bytes, or traversal
segments. Reads are bounded and never include a response body in diagnostics.

## Locked namespace set

There is no single global Nutanix API version. M0 locks one selected version
for every namespace in the registry. Selection prefers the newest GA version
matching `v<major>.<minor>`. A preview version is allowed only when the
namespace has no GA version, and the manifest records `preview` explicitly.

The approved initial lock is:

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
| `storage` | `v4.0.a3` | preview; no GA published |
| `vmm` | `v4.2` | GA |
| `volumes` | `v4.2` | GA |

## Manifest and cache

`specs/nutanix/manifest.json` is the repository lock. For each selected
namespace and version it records stability and the published OpenAPI, Postman,
and English error-reference artifact URLs when available. Each locked artifact
records its URL, expected media type, byte count, and SHA-256 digest.
The manifest and each cache body are written through a temporary file in the
destination directory followed by atomic replacement. The manifest has no
wall-clock timestamp, so an unchanged portal produces byte-identical lock
content. A staged-file gate rejects vendor cache paths even if they were added
with Git's force option.

Downloaded bodies live only under the repository-ignored
`.cache/nutanix/artifacts/` tree. The provider never downloads registry or
artifact content at runtime. Vendor bodies are not committed before a separate
redistribution review approves their inclusion.

An update must prove that the locked namespace set equals the live registry
set, that GA-first selection remains correct, and that every locked artifact
matches its recorded metadata and digest. Artifact updates do not change
application code.

## Evidence precedence

Use sources in this order:

1. the selected GA OpenAPI document, or the explicitly selected preview
   OpenAPI document when no GA version exists;
2. the selected version's English error reference;
3. the selected version's Postman collection;
4. official SDK documentation and examples as comparison evidence only;
5. an authorized live PE or PC observation only for an explicitly recorded
   documentation gap.

Record OpenAPI, error-reference, and Postman discrepancies as contract risks;
do not resolve them silently. A live observation documents a gap and does not
rewrite the locked source without review.

Nutanix SDKs are neither runtime dependencies nor code-generation inputs.
OpenAPI does not generate transport code, DTOs, Terraform schemas, state
models, or lifecycle logic. Every product implementation task cites the exact
locked namespace, version, operations, and schemas it uses.

## References

- [Nutanix namespace registry](https://developers.nutanix.com/api/v1/namespaces/)
- [Nutanix Developer Portal](https://developers.nutanix.com/)
- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Provider architecture](../architecture.md)
