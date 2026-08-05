## Summary

Describe the user-visible outcome and the bounded implementation scope.

## Contract

- Beads issue:
- Terraform resources, data sources, or actions affected:
- State, identity, import, and compatibility impact:

## Verification

- [ ] `./dev task verify`
- [ ] Generated docs and artifact locks are current.
- [ ] Acceptance evidence is attached to the Beads issue.

## Safety

- [ ] No credentials, endpoints, Terraform state, or secret-bearing fixtures are committed.
- [ ] Live-system mutation is absent, or its explicit authorization and rollback are documented.
- [ ] The change does not introduce a Nutanix SDK runtime dependency.
