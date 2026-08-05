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
at `/git-metadata/.git`, then set `GIT_WORK_TREE`, `GIT_COMMON_DIR`, and the
safely relative `GIT_DIR` explicitly. Reject a resolved Git directory outside
the common directory. This preserves real Git behavior in both primary and
linked worktrees without mounting a Podman control socket.

Set `BEADS_DIR=/workspace/.beads` in the Compose service so an interactive
shell uses the worktree-local tracker. The launcher also passes that exact
value explicitly to each noninteractive `bd` exec; other per-exec commands do
not receive a launcher-added Beads variable.
The top-level launcher must first discover the project through exactly three
bounded, token-scrubbed Git queries; a pre-discovery Beads guard is not
possible because the worktree root is not yet known. After discovery, require
a real `.beads/` directory and a regular, non-symlink `.beads/config.yaml`, and
reject every other shape before connector entry, credential lookup, Beads
exec, or Podman API use. Only a command whose first Beads argument is exactly
`init` bypasses this guard, and it still receives the explicit worktree-local
`BEADS_DIR`.

The `.git` basename is an intentional resolver boundary. Beads 1.1.2 derives
its automatic common-directory fallback as the sibling
`/git-metadata/.beads`, which remains in the container overlay. A host fallback
inside the required bind is still physically reachable at
`/git-metadata/.git/.beads` by arbitrary toolbox processes, but the pinned
Beads resolver and launcher never select it as canonical state or mutate it
automatically. Do not add a nested mount: container runtimes may create its
host-side mountpoint while applying mounts sequentially.

Remove `GH_TOKEN` and `GITHUB_TOKEN` from ordinary Compose, shell, Task, and
Beads environments and every ordinary host subprocess environment. Redact both
known nonempty token values from captured host-command output. Only remote
`bd bootstrap`, `bd dolt push`, and `bd dolt pull` operations may receive a
token, with nonempty precedence `GH_TOKEN`, `GITHUB_TOKEN`, then a bounded,
scrubbed host `gh auth token` lookup. Inject the token per exec, configure Git
authentication in the container first, and redact it from both output streams.
`bd bootstrap` remains subject to the fail-closed worktree marker guard; the
only bypass remains an exact first-argument `init` command.

For remote `bd bootstrap`, `bd dolt push`, and `bd dolt pull` only, prefix the
direct argument array with `/usr/bin/env -u GIT_COMMON_DIR -u GIT_DIR -u
GIT_WORK_TREE`.
Beads resolves the worktree database from `BEADS_DIR`, while its internal Git
commands rediscover repository metadata without conflicting explicit
worktree/common-dir variables. Keep the remote exec environment exactly
`BEADS_DIR` plus the short-lived `GH_TOKEN`; keep `gh auth setup-git`
token-only. Ordinary Beads, Task, and shell operations are unchanged.

Pinned Beads 1.1.2 serializes `sync.remote` with `strings.Join` and removes the
tracked config's final LF during an executing bootstrap. Before invoking `bd`,
the launcher records through a read-only, non-symlink file descriptor whether
the regular source config ended in LF. Only when that provenance is true and an
execution-mode bootstrap returns zero does it run `python -m
scripts.automation.beads_config` inside the same toolbox with only `BEADS_DIR`.
The normalizer appends a single missing LF through a regular, non-symlink file
descriptor, preserves all other bytes and file mode, and rejects missing,
empty, directory, or symlink targets. Help and version use the ordinary
token-free path; dry-run remains remote-capable but never normalizes. A source
already lacking LF is never normalized. A normalizer failure makes bootstrap
fail. No normalization runs after failed bootstrap or any other Beads command.

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
