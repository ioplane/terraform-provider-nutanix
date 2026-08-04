# Terraform provider contract

## Provider identity

| Property | Contract |
| --- | --- |
| Terraform Registry address | `ioplane/nutanix` |
| Provider type | `nutanix` |
| Provider implementation | Terraform Plugin Framework |
| Provider protocol | Terraform Plugin Protocol 6 |

The protocol version defines compatibility between Terraform CLI and the
provider binary. It does not version Nutanix APIs or public Terraform types.

## Public compatibility

The following are compatibility contracts once a Terraform type is released:

- public resource and data-source names;
- schema names, nesting, types, optionality, computed behavior, and
  sensitivity;
- values persisted in state and their meaning;
- mapping between Terraform state and Nutanix remote identity;
- import grammar and import result;
- state-upgrade paths and preserved state semantics.

Terraform types use `nutanix_<domain>_<noun>` in lower snake case. A public
name does not include an API version merely because its current Nutanix
transport does. A version suffix is allowed only when an existing compatibility
contract requires it.

The Nutanix external identifier (ext-ID), represented by `ext_id` in v4 APIs,
is the canonical remote identity when the selected API exposes it. Every type
contract defines how that identity maps to Terraform state and import syntax.
It must not silently substitute a display name or another mutable field.

Each future type contract defines attribute-level null and unknown behavior,
sensitivity, secret handling, drift behavior, replacement boundaries, and
state persistence. This repository-wide contract supplies no implicit defaults
for those choices. In particular, it invents no provider endpoint,
authentication, credential, or TLS attributes.

## Design gate

Before implementation begins, every Terraform type requires an approved ARC
contract. For resources and data sources, that contract covers:

1. public name and complete schema;
2. state model and lifecycle semantics;
3. remote identity and `ext_id` mapping;
4. import grammar and state-upgrade obligations;
5. positive, negative, drift, and compatibility tests.

The contract names the exact locked Nutanix namespace, version, operations,
and schemas used as evidence. Implementation remains hand-written and follows
the dependency boundaries in [the provider architecture](architecture.md).

## M0 boundary

M0 registers zero resources, data sources, actions, functions, and ephemeral
resources. It proves only the empty provider foundation and protocol 6 delivery
controls. Provider configuration and product types are future contracts; this
document does not define them ahead of their design and review.

Legacy state inventory, state migration, and full downstream compatibility are
M7 gates, not M0 completion claims.

## References

- [Approved foundation design](superpowers/specs/2026-08-04-foundation-design.md)
- [Approved foundation implementation plan](superpowers/plans/2026-08-04-foundation.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform plugin protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
