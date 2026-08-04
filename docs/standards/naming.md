# Naming standard

## Go names

Use canonical initialism case in exported and unexported identifiers.

| Initialism | Use | Do not use |
| --- | --- | --- |
| API | `APIClient`, `apiVersion` | `ApiClient`, `api_version` |
| HTTP | `HTTPClient`, `httpRequest` | `HttpClient`, `HTTP_request` |
| ID | `clusterID`, `ID()` | `clusterId`, `Id()` |
| JSON | `JSONBody`, `jsonDecoder` | `JsonBody`, `json_decoder` |
| TLS | `TLSConfig`, `tlsClient` | `TlsConfig`, `tls_client` |
| URL | `baseURL`, `URL()` | `baseUrl`, `Url()` |
| UUID | `clusterUUID`, `uuidText` | `clusterUuid`, `UUID_text` |
| ETag | `ETag`, `ifMatchETag` | `Etag`, `ETAG` |

Receiver names are short, consistent one- or two-letter abbreviations for the
receiver type. For example, methods on `Client` use `c`; methods on
`Transport` use `t`. Never use `self`, `this`, or `me`.

Every exported declaration has a complete doc comment that begins with the
declared name. Package names are lower-case, single words with a concrete
purpose.

Go source filenames are descriptive lower snake case. Unit and integration
test files end in `_test.go`; `_acc_test.go` is reserved for tests that access
an authorized live system.

Use `ErrName` for a sentinel error only when callers need stable identity with
`errors.Is`. Use `NameError` for a structured error type whose fields or type
carry meaning. Error strings start lower-case and have no terminal punctuation.

## Terraform names

Public Terraform type names use `nutanix_<domain>_<noun>` in lower snake case.
An API namespace or version does not enter a public name merely because the
current transport uses it. A version suffix is allowed only when an approved
compatibility contract requires that exact existing name.

This rule defines the shape of future names; it does not claim that any
resource, data source, action, function, or ephemeral resource is implemented.
Each public name still requires the ARC gate in the
[Terraform provider contract](../contract.md).

Provider environment variables use the `NUTANIX_` prefix and upper snake case.
Nutanix API namespace and version names remain internal to artifact,
transport, and compatibility code.

## References

- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Organizing a Go module](https://go.dev/doc/modules/layout)
- [Terraform provider contract](../contract.md)
- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)

