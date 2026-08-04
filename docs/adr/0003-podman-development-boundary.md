# ADR 0003: Podman Development Boundary

**Status:** Accepted

**Date:** 2026-08-04

## Context

Build and validation evidence must be reproducible across workstations and CI.
Host Go, Terraform, Task, Beads, Python, and quality-tool versions would make
results depend on local machine state.

## Decision

Limit the host control plane to Git, GitHub CLI, Podman, and the pinned `uv`
bootstrap environment. Run every development gate through `./dev` in the
pinned Podman toolbox, including all build, test, lint, generation, security,
Terraform, Task, Beads, Python, and packaging work. The toolbox supplies the
pinned Go 1.26 toolchain.

Use `podman-compose` for container lifecycle and `podman-py` for readiness,
inspection, and command execution. Do not mount the Podman socket inside the
development container.

## Consequences

Local and CI evidence uses the same isolated toolchain, and the toolbox cannot
control unrelated host containers through a mounted socket. Contributors pay
an initial bootstrap cost, require a working Podman environment, and incur
container startup and execution overhead.

## References

- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Go 1.26 engineering standard](../standards/go-1.26.md)
- [Testing standard](../standards/testing.md)
- [Podman documentation](https://docs.podman.io/en/latest/)
- [podman-py](https://github.com/containers/podman-py)
- [Compose Specification](https://compose-spec.io/)
