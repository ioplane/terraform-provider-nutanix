# ADR 0006: M1 Hand-Written Kernel Contract

**Status:** Accepted

**Date:** 2026-08-05

## Context

Nutanix APIs expose independently versioned namespaces and operation-specific
behavior. The locked Prism v4.3 artifact requires `NTNX-Request-Id` on selected
mutations and defines it as an idempotence token, represents long-running work
with task references, exposes zero-based OData pagination, requires
`If-Match` for selected updates, publishes operation-specific rate limits, and
supports both Basic and `X-ntnx-api-key` authentication.

A generic HTTP or retry client cannot decide which Nutanix mutation is safe to
replay, when a task reference ends mutation retries, how an ETag conflict maps
to Terraform drift, or which response fields are safe to log. Those decisions
are provider compatibility behavior.

## Decision

Implement M1 as a standard-library-first hand-written kernel under the package
boundaries in the provider architecture.

- The provider accepts one explicit HTTPS origin, exactly one Basic or API-key
  authentication mode, secure TLS defaults, an optional PEM trust bundle, and
  a bounded per-attempt timeout. Environment values are fallbacks, not hidden
  schema defaults.
- Namespace adapters own operation policies and DTOs. Transport owns bounded
  HTTP execution, typed errors, ETag extraction, explicit retries, and
  callback-based pagination.
- Mutation retries require locked evidence that the operation accepts
  `NTNX-Request-Id`. One RFC-compatible identifier is reused across attempts.
- Task polling, ETag conflict handling, and capability probes are explicit
  state machines with caller-owned contexts and no background goroutines.
- Terraform Plugin Log is the only runtime logging integration. Secrets,
  headers, raw URLs, query values, bodies, names, and remote identifiers are
  excluded.
- M1 adds no product Terraform type and persists no provider or kernel value in
  Terraform state.

Use the exact dependency decisions in the Go dependency policy. Do not add a
Nutanix SDK, generated client, generic HTTP/retry framework, SDKv2 runtime,
Plugin Mux, dependency-injection framework, or second logging stack.

## Rejected alternatives

### Generated Nutanix SDK clients

They cover many endpoints quickly but move request semantics and DTO churn
behind pre-v1 vendor modules, without defining Terraform state or lifecycle.
This conflicts with the accepted hand-written client decision.

### Resty or generic retry libraries

They reduce HTTP boilerplate but own middleware and retry behavior at a layer
that does not know the locked Nutanix operation contract. A method- or
status-based default can replay a mutation unsafely.

### SDKv2 plus Plugin Mux

This is appropriate for gradual migration of an existing provider. The new
provider has no SDKv2 implementation to preserve and would gain a second schema
and lifecycle model without a compatibility benefit.

### OpenTelemetry in M1

Tracing is useful but requires exporter lifecycle, privacy, redaction,
cardinality, and logical-operation ownership decisions. It is deferred until a
separate contract justifies the runtime surface.

## Consequences

The provider gains deterministic, reviewable Nutanix-aware behavior and a small
dependency surface. It must maintain more focused transport, retry,
pagination, task, capability, and redaction tests. Product work cannot obtain a
new retry or state behavior by convenience; it must cite a locked operation and
pass its own ARC gate.

## Approval

An independent ARC review verified the provider schema, secret handling, state
impact, import graph, transport interfaces, path-parameter encoding, error
taxonomy, retry and polling semantics, pagination, capability boundary,
artifact citations, and test contract. Three correction loops closed every
Critical and Important finding, and the final verdict on 2026-08-05 was
`APPROVED`. Command and review evidence is recorded in Beads task
`ntnx-m1.1`.

## References

- [M1 kernel design](../superpowers/specs/2026-08-05-m1-kernel-design.md)
- [Terraform provider contract](../contract.md)
- [Provider architecture](../architecture.md)
- [Go dependency policy](../standards/dependencies.md)
- [Nutanix artifact standard](../standards/nutanix-artifacts.md)
- [Locked Nutanix manifest](../../specs/nutanix/manifest.json)
- [Nutanix Prism v4.3 OpenAPI](https://developers.nutanix.com/api/v1/namespaces/prism/versions/v4.3/yaml)
