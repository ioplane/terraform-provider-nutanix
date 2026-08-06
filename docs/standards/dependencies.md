# Go dependency policy

## Selection rule

Prefer the Go 1.26 standard library and add a module only when it provides a
well-bounded capability that would be riskier to maintain locally. Runtime
dependencies must not take ownership of Nutanix request semantics, Terraform
state semantics, diagnostics, or public schemas.

Every direct module is pinned to an exact version. A version update requires a
fresh check of the Go module delivery channel, upstream maintenance state,
release notes across the complete upgrade range, and the full containerized
gate. Pre-v1 modules are isolated behind internal package boundaries.

## Approved M1 modules

| Module | M1 version | Scope |
| --- | --- | --- |
| `github.com/hashicorp/terraform-plugin-framework` | `v1.19.0` | Terraform provider, schema, diagnostics, and Protocol 6 integration |
| `github.com/hashicorp/terraform-plugin-go` | `v0.31.0` | Narrow `tfprotov6` protocol tests; not business logic |
| `github.com/hashicorp/terraform-plugin-testing` | `v1.16.0` | Test-only Terraform lifecycle and Protocol 6 acceptance |
| `github.com/hashicorp/terraform-plugin-framework-validators` | `v0.19.0` | Framework-native provider and attribute validation |
| `github.com/hashicorp/terraform-plugin-log` | `v0.11.0` | Structured `tflog` integration with Terraform context |
| `github.com/google/uuid` | `v1.6.0` | RFC-compatible `NTNX-Request-Id` generation |
| `github.com/google/go-cmp` | `v0.7.0` | Test-only semantic comparisons |

The 2026-08-05 dependency audit upgraded `terraform-plugin-log` from `v0.10.0`
to the current `v0.11.0`. It is a direct runtime dependency of
`internal/transport`, its release raises the module baseline to Go 1.25, and it
remains below the provider's Go 1.26 baseline. The audit found no newer release
for any direct module listed above.

The standard library owns HTTP, TLS, JSON, URLs, contexts, deadlines, errors,
and deterministic test servers. Use `net/http`, `crypto/tls`, `crypto/x509`,
`encoding/json`, `net/url`, `context`, `errors`, `io`, `time`,
`net/http/httptest`, and Go 1.26 `testing/synctest` before considering an
external package.

## Capability-triggered modules

These modules are approved candidates, not M1 dependencies. Add one only in
the task that first proves the named public or runtime need.

| Module | Current version | Add only for |
| --- | --- | --- |
| `github.com/hashicorp/terraform-plugin-framework-timeouts` | `v0.7.0` | User-configurable resource or action timeouts |
| `github.com/hashicorp/terraform-plugin-framework-nettypes` | `v0.3.0` | Terraform IP, CIDR, and MAC semantic types |
| `github.com/hashicorp/terraform-plugin-framework-timetypes` | `v0.5.0` | Public RFC3339 or duration values |
| `golang.org/x/sync` | `v0.22.0` | Proven bounded parallelism or single-flight need |
| `golang.org/x/time` | `v0.15.0` | Proven client-side rate-limiter requirement |
| `github.com/getkin/kin-openapi` | `v0.146.0` | Build- or test-only checks of locked OpenAPI operations |
| `go.opentelemetry.io/otel` | `v1.45.0` | Approved telemetry lifecycle and privacy contract |
| `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp` | `v0.70.0` | Approved HTTP span instrumentation |

Opaque JSON types are exceptional. Do not add Framework JSON types to expose a
raw Nutanix payload in place of a reviewed Terraform schema.

## Rejected M1 abstractions

- Do not use a Nutanix SDK, OpenAPI generator, Terraform SDKv2 runtime, or
  Terraform Plugin Mux.
- Do not use Resty `v2.17.2` or pre-release Resty `v3.0.0-rc.3`.
- Do not use `go-retryablehttp v0.7.8`, `backoff/v7 v7.0.0`,
  `retry-go/v5 v5.0.0`, or `failsafe-go v0.9.6` for shared kernel retries.
  Retry is selected by a reviewed Nutanix operation policy, not merely by an
  HTTP method or status.
- Do not use `go-playground/validator/v10 v10.30.3`; Terraform null, unknown,
  path diagnostics, and cross-attribute rules belong to Framework validators
  and hand-written domain validation.
- Do not add a second provider logging stack such as `log/slog`, Zap, or
  Zerolog. Runtime logs use `tflog` and Terraform context.
- Do not add Cobra, Viper, Fx, Wire, a web framework, or Testcontainers. The
  provider has one manually assembled binary; HTTP tests use local in-process
  servers.

## References

- [Go 1.26 engineering standard](go-1.26.md)
- [Testing standard](testing.md)
- [Provider architecture](../architecture.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Nutanix artifact standard](nutanix-artifacts.md)
