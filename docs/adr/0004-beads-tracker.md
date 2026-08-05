# ADR 0004: Beads Tracker

**Status:** Accepted

**Date:** 2026-08-04

## Context

The delivery plan needs dependency-aware task state, an atomic ready queue,
and synchronized history across branches and worktrees. A manually maintained
Markdown plan or custom JSON registry cannot provide those guarantees without
reimplementing tracker behavior.

## Decision

Use Beads 1.1.2 with embedded Dolt as the canonical task and dependency
tracker. Synchronize the Dolt database through the Git origin under
`refs/dolt/data` using the Beads Dolt workflow. Treat `.beads/issues.jsonl` and
the generated roadmap as review and human-readable projections only, never as
canonical task state.

Anchor Beads to `BEADS_DIR=/workspace/.beads` in the development toolbox.
Mount the Git common directory at `/git-metadata/.git`: the `.git` basename
makes Beads 1.1.2 resolve any automatic common-directory fallback to the
container-overlay sibling `/git-metadata/.beads`, not a database nested inside
the host bind. Worktree-local embedded Dolt state keeps each isolated
Compose/worktree environment aligned with its checkout. Ignore database
subtrees beneath `.beads/`, while tracking `.beads/config.yaml` and
`.beads/issues.jsonl`. Do not automatically discover, migrate, or delete a
database from a previous fallback location.

After the launcher's mandatory bounded and token-scrubbed Git project
discovery, fail closed until `.beads/` is a real directory and
`.beads/config.yaml` is a regular, non-symlink file in the current worktree.
The launcher must not invoke Beads, resolve credentials, enter the Podman
connector, or call the Podman API for a rejected command. Permit only a command
whose first Beads argument is exactly `init`; this exception is what creates
the anchored database. Options placed before `init` do not bypass the guard.

The required Git bind makes an existing host fallback physically reachable at
`/git-metadata/.git/.beads`; this ADR does not claim filesystem invisibility
from arbitrary toolbox commands. The invariant is narrower and testable: the
pinned Beads resolver and launcher canonical path never select that nested
location, and no automatic tracker operation mutates it. Avoid nested tmpfs or
volume mounts because a runtime may create the child mountpoint in the host
bind while applying mounts.

Remote Dolt synchronization has a narrower Git environment than ordinary
toolbox commands. Invoke only `bd bootstrap`, `bd dolt push`, and `bd dolt
pull` through the direct argument-array prefix `/usr/bin/env -u GIT_COMMON_DIR
-u GIT_DIR -u GIT_WORK_TREE`, while preserving `BEADS_DIR` and the short-lived
`GH_TOKEN` in the per-exec environment. Bootstrap remains behind the same
fail-closed worktree marker guard; only exact first-argument `init` bypasses
it. This lets Beads select `/workspace/.beads` and lets its internal Git
commands rediscover the repository without the explicit worktree variables
that otherwise produce a fatal Git configuration error.

Keep exactly one critical-path task `in_progress`. Close a task only after its
acceptance evidence is attached.

## Consequences

Task readiness, dependencies, claims, and history remain structured and
synchronizable. Contributors and automation must install the pinned tracker,
bootstrap its embedded database, and follow its explicit pull and push
workflow. The extra tooling and synchronization discipline replace simpler but
non-authoritative Markdown status editing.

## References

- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Beads 1.1.2 Dolt synchronization](https://github.com/gastownhall/beads/blob/v1.1.2/docs/DOLT.md)
