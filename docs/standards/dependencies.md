# Go dependency policy

## Selection rule

Prefer the Go 1.26 standard library. Add a module only when it provides a bounded capability that is
riskier to maintain locally. Runtime dependencies must not own Nutanix request semantics, Terraform
state semantics, diagnostics, or public schemas.

Every direct module uses an exact version. An upgrade requires upstream release-note review,
maintenance and provenance checks, `go mod tidy`, `govulncheck`, and the complete Podman gate.
Pre-v1 modules remain behind internal package boundaries.

## Approved modules

| Module | Version | Scope |
| --- | --- | --- |
| `github.com/hashicorp/terraform-plugin-framework` | `v1.19.0` | Provider, schemas, diagnostics, and Protocol 6 integration |
| `github.com/hashicorp/terraform-plugin-go` | `v0.31.0` | Narrow `tfprotov6` protocol verification |
| `github.com/hashicorp/terraform-plugin-testing` | `v1.16.0` | Deferred Terraform lifecycle and acceptance verification |
| `github.com/hashicorp/terraform-plugin-framework-validators` | `v0.19.0` | Framework-native configuration and attribute validation |
| `github.com/hashicorp/terraform-plugin-log` | `v0.11.0` | Structured `tflog` integration |
| `github.com/google/uuid` | `v1.6.0` | RFC-compatible `NTNX-Request-Id` generation |
| `github.com/google/go-cmp` | `v0.7.0` | Deferred semantic comparisons in product tests |

The standard library owns HTTP, TLS, JSON, URLs, contexts, deadlines, errors, I/O, and deterministic
test servers.

## Capability-triggered modules

These modules are candidates, not dependencies. Add one only when the owning product contract proves
the named requirement.

| Module | Version baseline | Capability |
| --- | --- | --- |
| `terraform-plugin-framework-timeouts` | `v0.7.0` | User-configurable resource or action timeouts |
| `terraform-plugin-framework-nettypes` | `v0.3.0` | Terraform IP, CIDR, and MAC semantic types |
| `terraform-plugin-framework-timetypes` | `v0.5.0` | Public RFC 3339 or duration values |
| `golang.org/x/sync` | `v0.22.0` | Proven bounded parallelism or single-flight behavior |
| `golang.org/x/time` | `v0.15.0` | Proven client-side rate limiting |
| `github.com/getkin/kin-openapi` | `v0.146.0` | Build-only validation of locked OpenAPI operations |
| `go.opentelemetry.io/otel` | `v1.45.0` | Approved telemetry lifecycle and privacy contract |

## Rejected abstractions

- Nutanix SDKs, OpenAPI-generated runtime clients, Terraform SDKv2, and Terraform Plugin Mux;
- generic HTTP, retry, dependency-injection, configuration, and validation frameworks;
- a second runtime logging stack beside Terraform `tflog`;
- raw JSON Terraform attributes used instead of reviewed schemas;
- a public Go library API from this provider module.

## References

- [Go engineering standard](go-1.26.md)
- [Provider architecture](../architecture.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
