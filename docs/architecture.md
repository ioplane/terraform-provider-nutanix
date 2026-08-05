# Provider architecture

This project is a greenfield, one-binary Terraform provider implemented as a
modular monolith. It does not fork or reuse implementation code from the
official Nutanix provider.

## Package layout

The production and test package layout is:

```text
cmd/terraform-provider-nutanix
internal/provider
internal/service/<domain>/<terraform-type>
internal/nutanix/<api-namespace>
internal/transport
internal/auth
internal/task
internal/capability
internal/testserver
```

Allowed dependencies are explicit:

| Package | Allowed imports and constraints |
| --- | --- |
| `cmd/terraform-provider-nutanix` | Imports `provider` only. |
| `provider` | May import `service`, `auth`, `transport`, `task`, `capability`, Terraform Plugin Framework, and Terraform Plugin Log. |
| `service` | May import `nutanix/<namespace>`, `task`, `capability`, and Terraform Plugin Framework; never imports `auth` or `transport`. |
| `nutanix/<namespace>` | May import `transport`, `task`, and `capability` as needed; never imports `auth` or Terraform Plugin Framework. |
| `task` | Depends only on the Go standard library and project-neutral helpers; never imports Terraform Plugin Framework. |
| `capability` | Depends only on the Go standard library and project-neutral helpers; never imports a namespace, transport, or Terraform Plugin Framework. |
| `transport` | May import `auth` and Terraform Plugin Log; never imports Terraform Plugin Framework. |
| `auth` | Depends only on the Go standard library and project-neutral helpers; never imports Terraform Plugin Framework. |
| `testserver` | May import production packages only for tests; no production package may import `testserver`. |

No reverse dependency or import cycle is allowed. `provider` assembles the
Terraform provider; `service` owns Terraform schemas, state models, and
lifecycle behavior; `nutanix/<namespace>` owns namespace-specific DTOs and
operations; and `transport`, `auth`, `task`, and `capability` remain focused
support packages.

Catch-all Go packages named `api`, `core`, `common`, `types`, or `util` are
forbidden. Shared behavior belongs in a package with one concrete
responsibility.

## Implementation boundary

Transport code, Nutanix DTOs, Terraform schemas, and Terraform state models
are hand-written. Official OpenAPI and related Nutanix artifacts are locked
design evidence and test inputs, not code-generation inputs. Nutanix SDKs are
not runtime dependencies, and neither SDKs nor OpenAPI generate implementation
code or public Terraform schemas.

M0 will establish an empty provider served through Terraform Plugin Protocol
6. It will register no resource, data source, action, function, or ephemeral
resource. Product behavior begins only in later phases after its public
contract is approved.

## M1 kernel composition

M1 preserves manual composition. `provider.Configure` resolves the Terraform
configuration and environment, constructs immutable authentication and TLS
values, then constructs one shared transport client and one lazy capability
registry. Configuration constructs clients but performs no live network call.
The configured provider data passed to future services contains interfaces at
their point of consumption, not a service locator or global singleton.

Each service package defines the smallest provider-data port it consumes.
`provider` implements that port with already constructed namespace adapters,
task waiters, and capability accessors. A service never receives the raw
transport client and cannot construct an operation policy or bypass a
namespace adapter.

```text
Terraform provider configuration
          |
          v
internal/provider  ---- Framework diagnostics and tflog context
          |
          +---- internal/auth       Basic or X-ntnx-api-key
          +---- internal/transport  HTTPS, errors, retry, pagination, ETag
          +---- internal/task       context-bound Prism task waiter
          +---- internal/capability lazy read-only probes and cache
                         |
                         v
                internal/nutanix/<namespace>
```

The kernel uses five explicit boundaries:

1. `provider` owns the public provider schema, null/unknown/environment
   resolution, Framework diagnostics, and configured service data.
2. `auth` owns credential application. It has no logging, environment access,
   URL construction, or Terraform dependency.
3. `transport` owns one origin-bound `net/http.Client`, TLS, request attempts,
   body limits, typed HTTP errors, retry scheduling, pagination control, ETag
   extraction, and structured attempt logging. It does not decode
   namespace-specific success DTOs.
4. `task` owns a synchronous state machine over a consumer-defined task reader.
   Prism DTO conversion belongs to `nutanix/prism`, so the task package remains
   vendor-shape-neutral and independently testable.
5. `capability` owns concurrency-safe positive and negative probe results. A
   namespace adapter supplies a documented read-only probe; the registry never
   scans undocumented endpoints and never turns 401, 403, 429, or 5xx into an
   unsupported result.

Namespace packages describe every operation with an immutable operation
policy: method, path template, expected response codes, response limit,
request-ID requirement, and retry class. This is the only place where a
mutation becomes idempotently retryable. The transport never infers that from
`POST`, `PUT`, `PATCH`, or `DELETE` alone.

`task` and `capability` define consumer-side interfaces and neutral values.
Namespace adapters may import and implement those interfaces; the neutral
packages never import a namespace. This direction lets `provider` compose the
adapters without an import cycle.

Retries are attempts within one logical operation. A stable
`NTNX-Request-Id` spans all mutation attempts, while each attempt has its own
duration and status event. Once a mutation returns a task reference, the
mutation is complete from the transport perspective; task polling begins and
the original mutation is not replayed.

Pagination is callback-based rather than collect-all. A namespace adapter
fetches and decodes one page; the shared walker advances zero-based pages,
checks optional totals, detects short and empty pages, and enforces explicit
page and item ceilings. It constructs the next query locally instead of
following vendor-provided absolute links.

ETags remain outside public Terraform state by default. Read operations expose
the response ETag to their service. Update operations explicitly send
`If-Match`; 412 and 428 remain typed results for service-level drift or
concurrency handling. A future Terraform type ARC may approve private state or
another persistence strategy, but M1 does not decide that on its behalf.

## M1 dependency boundary

The kernel is standard-library-first. Generic HTTP clients, retry frameworks,
dependency-injection frameworks, a Nutanix SDK, and generated clients are not
used. Approved versions and capability-triggered candidates are recorded in
the [Go dependency policy](standards/dependencies.md).

`tflog` is the sole runtime logging path. Transport logs contain only an
allowlist of non-secret attempt metadata and use path templates rather than raw
URLs. OpenTelemetry is intentionally outside M1 until exporter lifecycle,
privacy, metric cardinality, and logical-operation span ownership have their
own approved contract.

## Delivery phases

| Phase | Architectural outcome |
| --- | --- |
| M0 Foundation | Repository controls, reproducible tooling, CI, and an empty protocol 6 provider |
| M1 Kernel | Configuration, authentication, transport, diagnostics, retries, pagination, ETags, task polling, and capabilities |
| M2 Read-only canary | Initial read-only types and an import canary |
| M3 Foundation resources | Categories, projects, subnets, storage containers and policies, and image placement |
| M4 Compute and block storage | Virtual machines, volume groups, and affinity |
| M5 IAM | Directories, users, groups, roles, policies, and user keys |
| M6 Objects compatibility | Official create-only Object Store lifecycle |
| M7 Compatibility release | Current downstream resource and data-source surface with state fixtures |
| M8 Segmented Objects | Separate draft plus precheck and deploy actions |
| M9 Product expansion | Supported stable v4 product namespaces |
| M10 External planes | Foundation, NDB, Self-Service, NC2, NKP, NDK, and NAI adapters |

M0 makes no downstream compatibility claim. The detailed scope and gates for
each phase remain in the approved design and implementation plan.

## References

- [Approved foundation design](superpowers/specs/2026-08-04-foundation-design.md)
- [Approved foundation implementation plan](superpowers/plans/2026-08-04-foundation.md)
- [M1 kernel design](superpowers/specs/2026-08-05-m1-kernel-design.md)
- [M1 kernel contract](contract.md#m1-provider-configuration)
- [Go dependency policy](standards/dependencies.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform plugin protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
