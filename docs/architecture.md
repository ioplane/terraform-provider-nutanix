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
| `provider` | May import `service`, `capability`, and Terraform Plugin Framework. |
| `service` | May import `nutanix/<namespace>`, `task`, `capability`, and Terraform Plugin Framework. |
| `nutanix/<namespace>` | May import `transport` and `auth` as needed; never imports Terraform Plugin Framework. |
| `task` | Imports `transport`; never imports Terraform Plugin Framework. |
| `capability` | May import `nutanix/<namespace>` and `transport`; never imports Terraform Plugin Framework. |
| `transport` | May import `auth`; never imports Terraform Plugin Framework. |
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
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform plugin protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
