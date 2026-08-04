# Terraform Provider for Nutanix

> **Development status:** the M0 foundation is in progress. No Terraform resources, data sources, or actions are implemented. This repository is not ready for provider use.

This project is a greenfield Terraform provider for the Nutanix Cloud Platform. It is a handwritten modular monolith built with Terraform Plugin Framework and served over Terraform Plugin Protocol 6. It does not use a Nutanix SDK as a runtime dependency or generate implementation code from OpenAPI.

| Property | Value |
| --- | --- |
| Go module | `github.com/ioplane/terraform-provider-nutanix` |
| Terraform Registry address | `ioplane/nutanix` |
| Provider type | `nutanix` |
| Protocol | Terraform Plugin Protocol 6 |

## Foundation workflow

The repository launcher and development container are part of M0 and are not available yet. Once that work lands, the supported host entrypoints will be limited to:

```text
./dev up
./dev task all
./dev task verify
./dev beads ready --json
```

Do not run these commands until M0 provides `./dev`. Build, test, lint, generation, packaging, Terraform, Task, Beads, and required Python quality work will run inside Podman through that launcher.

## Project documents

- [Approved foundation design](docs/superpowers/specs/2026-08-04-foundation-design.md)
- [Approved foundation implementation plan](docs/superpowers/plans/2026-08-04-foundation.md)
- [Provider architecture](docs/architecture.md)
- [Terraform provider contract](docs/contract.md)
- [Go 1.26 engineering standard](docs/standards/go-1.26.md)
- [Naming standard](docs/standards/naming.md)
- [Nutanix artifact standard](docs/standards/nutanix-artifacts.md)
- [Testing standard](docs/standards/testing.md)
- [ADR 0001: Modular Monolith](docs/adr/0001-modular-monolith.md)
- [ADR 0002: Hand-Written Nutanix Client](docs/adr/0002-hand-written-nutanix-client.md)
- [ADR 0003: Podman Development Boundary](docs/adr/0003-podman-development-boundary.md)
- [ADR 0004: Beads Tracker](docs/adr/0004-beads-tracker.md)
- [ADR 0005: Nutanix Artifact Lock](docs/adr/0005-nutanix-artifact-lock.md)
- [Contribution guide](CONTRIBUTING.md)
- [Security policy](SECURITY.md)

## License

Licensed under the [Apache License 2.0](LICENSE).
