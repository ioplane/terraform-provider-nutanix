# ADR 0001: Modular Monolith

**Status:** Accepted

**Date:** 2026-08-04

## Context

The provider is a greenfield public Terraform provider delivered as one plugin
binary. Product domains need shared transport, authentication, task polling,
and capability behavior while preserving clear ownership of Terraform schemas
and Nutanix API namespaces.

Splitting the implementation into multiple providers or independently deployed
services would add distribution and coordination boundaries without serving
the M0 provider contract.

## Decision

Implement one hand-written provider binary as a modular monolith. Enforce the
package layout, dependency direction, Framework boundary, and focused-package
rules defined in the [provider architecture](../architecture.md). Terraform
behavior belongs in `provider` and `service`; Nutanix API and shared HTTP
behavior stay behind the Framework boundary.

## Consequences

The provider has one Registry identity, one release artifact, and shared
cross-domain behavior. Package boundaries must be enforced in review to avoid
coupling inside the binary. Unlike a multi-provider or microservice design,
components cannot be deployed or versioned independently; that tradeoff is
accepted for a single Terraform plugin.

## References

- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Provider architecture](../architecture.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
