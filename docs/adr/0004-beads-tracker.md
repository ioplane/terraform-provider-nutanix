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

Anchor Beads to `BEADS_DIR=/workspace/.beads` in the development toolbox. The
explicit anchor is required because linked worktrees expose their Git common
directory at `/git-common`; Beads 1.1.2 otherwise treats that non-`.git`
basename as a fallback location and can place the canonical database outside
the mounted worktree. Worktree-local embedded Dolt state keeps each isolated
Compose/worktree environment aligned with its checkout. Ignore database
subtrees beneath `.beads/`, while tracking `.beads/config.yaml` and
`.beads/issues.jsonl`. Do not automatically discover, migrate, or delete a
database from a previous fallback location.

Fail closed until `.beads/config.yaml` is a regular file in the current
worktree. The launcher must not invoke Beads discovery, resolve credentials, or
enter the Podman connector for a rejected command. Permit only a command whose
first Beads argument is exactly `init`; this exception is what creates the
anchored database. Options placed before `init` do not bypass the guard.

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
