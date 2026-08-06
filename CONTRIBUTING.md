# Contributing

## Engineering contract

Contributions must preserve the public boundaries defined by the
[provider contract](docs/contract.md), [architecture](docs/architecture.md), and
[engineering standards](docs/standards/go-1.26.md).

| Rule | Requirement |
| --- | --- |
| Implementation | Hand-written Go; no Nutanix SDK or generated runtime client |
| API authority | Locked Nutanix Developer Portal artifacts with exact operation evidence |
| Runtime | Terraform Plugin Framework and Protocol 6 |
| Development | Podman through `./dev` |
| Secrets | No credentials, endpoints, Terraform state, or secret-bearing fixtures |
| Tests | Product tests only, after the owning product corpus is structurally complete |

## Pull-request workflow

1. Open or select a GitHub issue with explicit scope and acceptance criteria.
2. Create a focused branch and keep unrelated changes separate.
3. Update public documentation when a contract, workflow, or compatibility surface changes.
4. Run the complete containerized gate.
5. Open a pull request with risk, compatibility, and verification evidence.
6. Resolve required reviews, conversations, and the `Foundation` status check before merge.

```bash
./dev up
./dev task all
```

> [!IMPORTANT]
> Live Nutanix mutation and live acceptance require an isolated non-production target,
> explicit authorization, bounded cleanup, and evidence that cleanup completed.

## Commit convention

Use [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/).

| Prefix | Release effect |
| --- | --- |
| `feat:` | Minor release before `v1.0.0` |
| `fix:` or `perf:` | Patch release |
| `feat!:` or `BREAKING CHANGE:` | Minor before `v1.0.0`; major from `v1.0.0` |
| `docs:`, `build:`, `ci:`, `chore:`, `refactor:`, `test:` | No release by default |

Commit subjects must be imperative, concise, and scoped to one change.

## Documentation

- Use GitHub-flavored Markdown, descriptive headings, tables for exact mappings, and Mermaid for
  non-trivial flows.
- Keep documents factual and present-tense; omit sprint narratives, private repository references,
  review transcripts, and tool-specific planning artifacts.
- Keep relative links valid and run the documentation gate through `./dev task docs:check`.
- Do not edit generated files under `docs/data-sources/` or `docs/index.md` directly.

## Release boundary

Release Please owns the release pull request, `CHANGELOG.md`, SemVer tag, and GitHub Release.
GoReleaser builds archives, checksums, and SBOMs inside Podman after the release PR merges.
Manual release tags and manual changelog version sections are not accepted.

See the [release process](docs/release-process.md).
