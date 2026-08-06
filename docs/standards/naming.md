# Naming standard

## Go names

Use canonical initialism case in exported and unexported identifiers.

| Initialism | Use | Avoid |
| --- | --- | --- |
| API | `APIClient`, `apiVersion` | `ApiClient`, `api_version` |
| HTTP | `HTTPClient`, `httpRequest` | `HttpClient`, `HTTP_request` |
| ID | `clusterID`, `ID()` | `clusterId`, `Id()` |
| JSON | `JSONBody`, `jsonDecoder` | `JsonBody`, `json_decoder` |
| TLS | `TLSConfig`, `tlsClient` | `TlsConfig`, `tls_client` |
| URL | `baseURL`, `URL()` | `baseUrl`, `Url()` |
| UUID | `clusterUUID`, `uuidText` | `clusterUuid`, `UUID_text` |
| ETag | `ETag`, `ifMatchETag` | `Etag`, `ETAG` |

- Receiver names are consistent one- or two-letter abbreviations such as `c` for `Client`.
- Exported declarations have complete doc comments beginning with the declared name.
- Source filenames use descriptive lower snake case.
- Product tests use `_test.go`; authorized live acceptance uses `_acc_test.go`.
- Sentinel errors use `ErrName` only when callers require stable `errors.Is` identity.
- Structured error types use `NameError`.

## Terraform names

Public types use `nutanix_<domain>_<noun>` in lower snake case. API versions enter a public name only
when an existing compatibility contract requires the versioned name. Provider environment variables
use the `NUTANIX_` prefix and upper snake case.

API namespace and version names remain internal to artifact, transport, and compatibility code.

## References

- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Provider contract](../contract.md)
