## Summary

Describe the user-visible outcome and bounded implementation scope.

## Contract impact

| Area | Impact |
| --- | --- |
| Terraform types | <!-- Resources, data sources, actions, or none --> |
| State and identity | <!-- State, import, drift, replacement, or none --> |
| Nutanix APIs | <!-- Namespace, version, operation ID, path, or none --> |
| Release | <!-- Conventional Commit release effect --> |

## Verification

- [ ] `./dev task all`
- [ ] Generated provider documentation is current.
- [ ] Locked API artifacts are current and verified.
- [ ] Product-test evidence is attached when the owning product corpus requires it.

## Safety

- [ ] No credentials, endpoints, Terraform state, or secret-bearing fixtures are committed.
- [ ] Live-system mutation is absent, or explicit authorization and cleanup evidence are linked.
- [ ] No Nutanix SDK or generated runtime implementation is introduced.
- [ ] Public documentation contains no internal planning, decision-log, or review artifacts.
