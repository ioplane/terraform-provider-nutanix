# ADR 0002: Hand-Written Nutanix Client

**Status:** Accepted

**Date:** 2026-08-04

## Context

Nutanix API namespaces advance independently, while Terraform schemas, state,
identity, import behavior, and lifecycle semantics form a public compatibility
contract. SDK types or generated clients do not define that Terraform
contract.

## Decision

Hand-write production transport behavior, Nutanix DTOs, Terraform schemas, and
Terraform state models. Use locked official Nutanix Developer Portal artifacts
as design evidence, contract-test input, and request fixtures. Do not add a
Nutanix SDK as a runtime dependency, and do not generate implementation code,
DTOs, schemas, or state models from OpenAPI or an SDK.

## Consequences

The provider controls its public model and can review every compatibility
choice independently of vendor SDK release cadence. The repository carries
more implementation code and must maintain more focused tests, fixtures, and
artifact-drift checks than an SDK-backed or generated client would require.

## References

- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Provider architecture](../architecture.md)
- [Nutanix artifact standard](../standards/nutanix-artifacts.md)
- [Nutanix Developer Portal](https://developers.nutanix.com/)
