# ADR 0005: Nutanix Artifact Lock

**Status:** Accepted

**Date:** 2026-08-04

## Context

The Nutanix Developer Portal publishes independently versioned API namespaces.
There is no single global API version, and remote artifacts can change outside
the repository. Provider designs need reviewable, reproducible API evidence.

## Decision

Maintain a repository lock with one selected version for every namespace in
the Developer Portal registry. Select the newest GA `v<major>.<minor>` release;
select a preview only when that namespace has no GA release and record its
stability explicitly.

For every available artifact, lock its URL, expected media type, byte size, and
SHA-256 digest. Keep downloaded bodies in the ignored artifact cache and do not
commit vendor bodies before a separate redistribution review. Artifact updates
are explicit, reviewable maintenance changes that verify the live namespace
set and locked metadata. The provider never downloads registry or artifact
content at runtime.

## Consequences

Design and contract tests use deterministic evidence, preview use is visible,
and artifact drift becomes a reviewable failure. Updates require network
verification and cache downloads, and registry changes must be reconciled
before the lock can pass verification.

## References

- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Nutanix artifact standard](../standards/nutanix-artifacts.md)
- [Nutanix namespace registry](https://developers.nutanix.com/api/v1/namespaces/)
