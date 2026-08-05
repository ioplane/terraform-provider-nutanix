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
inspection, status, and noninteractive command execution. The launcher uses a
direct argument array for every subprocess and applies bounded timeouts.
Interactive attach is the sole exception: after `podman-py` verifies the exact
container is healthy, `./dev shell` replaces the launcher process with
`podman exec --interactive --tty`. This exception is necessary because the
installed `podman-py` exec implementation ignores its `socket` option and does
not expose the required interactive attach stream. A bounded `podman ps` is
allowed only when the status API is unavailable. Do not mount the Podman socket
inside the development container.

Derive the Compose project name from the resolved Git common directory and
worktree root. Mount the worktree at `/workspace` and its Git common directory
at `/git-common`, then set `GIT_WORK_TREE`, `GIT_COMMON_DIR`, and the safely
relative `GIT_DIR` explicitly. Reject a resolved Git directory outside the
common directory. This preserves real Git behavior in both primary and linked
worktrees without mounting a Podman control socket.

Set `BEADS_DIR=/workspace/.beads` in the Compose service so an interactive
shell uses the worktree-local tracker. The launcher also passes that exact
value explicitly to each noninteractive `bd` exec; other per-exec commands do
not receive a launcher-added Beads variable.
Before the worktree contains a regular `.beads/config.yaml`, reject every
Beads command before connector, credential lookup, or exec. Only a command
whose first Beads argument is exactly `init` bypasses this guard, and it still
receives the explicit worktree-local `BEADS_DIR`.

Remove `GH_TOKEN` and `GITHUB_TOKEN` from ordinary Compose, shell, Task, and
Beads environments and every ordinary host subprocess environment. Redact both
known nonempty token values from captured host-command output. Only remote
`bd dolt push` and `bd dolt pull` operations may receive a token, with nonempty
precedence `GH_TOKEN`, `GITHUB_TOKEN`, then a bounded, scrubbed host
`gh auth token` lookup. Inject the token per exec, configure Git authentication
in the container first, and redact it from both output streams.

Wrap every noninteractive container command with pinned GNU `timeout`, using a
direct argument array and no shell. The wall-clock limits are 60 minutes for
Task, 5 minutes for ordinary Beads, 1 minute for remote Git authentication
setup, and 10 minutes for remote Beads. The wrapper sends `TERM` to the command
process group and `KILL` after a 2-second grace period. Podman transport calls
receive the remaining command deadline plus termination and API grace because
the transport timeout alone is not a total command lifetime. Preserve the
wrapped command's exit code and demultiplexed output.

## Consequences

Local and CI evidence uses the same isolated toolchain, and the toolbox cannot
control unrelated host containers through a mounted socket. Contributors pay
an initial bootstrap cost, require a working Podman environment, and incur
container startup and execution overhead. The Git common-directory mount gives
the toolbox repository metadata access, while the short-lived remote-token path
adds explicit credential-handling and redaction logic.

## References

- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Go 1.26 engineering standard](../standards/go-1.26.md)
- [Testing standard](../standards/testing.md)
- [Podman documentation](https://docs.podman.io/en/latest/)
- [podman-py](https://github.com/containers/podman-py)
- [Compose Specification](https://compose-spec.io/)
