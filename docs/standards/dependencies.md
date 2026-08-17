# Go dependency policy

## Selection rule

Prefer the Go 1.26 standard library. Add a module only when it provides a bounded capability that is
riskier to maintain locally. Runtime dependencies must not own Nutanix request semantics, Terraform
state semantics, diagnostics, or public schemas.

Every direct module uses an exact version. An upgrade requires upstream release-note review,
maintenance and provenance checks, `go mod tidy`, `govulncheck`, and the complete Podman gate.
Pre-v1 modules remain behind internal package boundaries.

## Version review status

The 2026-08-17 review keeps the current versions below until a dedicated upgrade task supplies
release-note, compatibility, hash, and complete-gate evidence. An available release is not an
applied release.

| Component | Current repository baseline | Review result | Follow-up |
| --- | --- | --- | --- |
| Go toolbox | Go 1.26.5, `golang:1.26-trixie` pinned by digest | Go 1.26.6 is current and required because the current `govulncheck` run reports four reachable standard-library advisories | `ntnx-d76.2` (P0) |
| Terraform Plugin Framework | `v1.19.0` | Direct module channel reports no newer version; Protocol 6 ABI tests remain required | Keep baseline |
| Terraform Plugin Go/testing/validators/log | `v0.31.0` / `v1.16.0` / `v0.19.0` / `v0.11.0` | Direct module channels report no newer versions | Keep baseline |
| Testcontainers Go | `v0.44.0` | Podman-only runtime passes with the host socket, explicit `ProviderPodman`, strict YAML, and `t.Cleanup` | Verified; `ntnx-d76.3` closed |
| Beads | `v1.1.2` | `v1.2.2` is a recovery release with local database schema caveats | `ntnx-d76.4` (P2) |
| Syft | `v1.50.0` | `v1.51.0` contains cataloger fixes and two vulnerability remediations | `ntnx-d76.4` (P2) |
| Transitive Go modules | Exact versions in `go.mod`/`go.sum` | Multiple updates are available; no mass upgrade is justified without reachability and ABI review | `ntnx-d76.1` (P2) |

The direct provider modules, Testcontainers, YAML library, Terraform CLI, Task, lint, release, and
action baselines were checked against their authoritative release channels. Tooling upgrades must
update the pinned image and `tool-assets.lock` together; transitive module updates must not become
a second source of provider behavior or version authority.

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
| `github.com/testcontainers/testcontainers-go` | `v0.44.0` | Container-backed Go integration tests |
| `gopkg.in/yaml.v3` | `v3.0.1` | Strict decoding of test runtime configuration |

The standard library owns HTTP, TLS, JSON, URLs, contexts, deadlines, errors, I/O, and deterministic
test servers.

## Test runtime configuration

Go module versions are authoritative in `go.mod` and `go.sum`; they are not duplicated in runtime
YAML or TOML. Runtime test variables belong in `config/testing.yaml`: image digests, bounded startup
and cleanup timeouts, readiness markers, and container commands. The loader rejects unknown fields
and invalid values. Tool and binary versions remain in `Containerfile`, `tool-assets.lock`, or module
metadata according to their ownership. Adding a second version authority to YAML or TOML is
prohibited.

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
