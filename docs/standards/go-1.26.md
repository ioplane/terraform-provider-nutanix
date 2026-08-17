# Go 1.26 engineering standard

## Toolchain contract

| Component | Contract |
| --- | --- |
| Module language baseline | `go 1.26.0` |
| Build toolchain | Go 1.26.6 in the current pinned Podman image |
| Language server | `gopls` 0.23.0 in the same image |
| Toolchain selection | `GOTOOLCHAIN=local` |
| Release build | `CGO_ENABLED=0` |
| Experimental features | Not supported |

The module directive defines minimum language and module semantics. The immutable image selects the
build toolchain and is pinned to the Go 1.26.6 security release. Go commands run only through the
repository Podman environment; the host Go installation is not verification evidence.

## Package design

- Production code remains under `cmd/` and `internal/`.
- Packages are lower-case, single-word units with one responsibility.
- Consumer packages declare the smallest interface they need.
- Interfaces are not added next to implementations only to support mocking.
- Standard-library behavior is preferred over generic third-party abstractions.
- Every function makes ownership, mutability, cancellation, and error propagation explicit.

## Context, errors, and concurrency

| Area | Rule |
| --- | --- |
| Context | `context.Context` is the first argument for request-bound work and is never stored in a struct |
| Errors | Add concise operation context and preserve inspectable causes with `%w` |
| Error text | Lower-case initial letter and no terminal punctuation |
| Logging | Handle or return an error; do not log and return the same failure repeatedly |
| Panic | Restricted to unrecoverable programmer invariants |
| Goroutines | Require an owner, bounded lifetime, cancellation path, and collected result |

## HTTP and security

HTTP clients preserve caller cancellation and use explicit timeouts. Every response body is bounded
and closed on every path. Credentials, authorization headers, sensitive query values, bodies, and
provider configuration are excluded from logs, diagnostics, and returned errors.

Run `govulncheck ./...` for reachable Go vulnerabilities. Toolchain and dependency updates require
review of every release note in the upgrade range.

## Required implementation gate

```bash
./dev task all
```

The gate includes Go formatting, `go vet`, `golangci-lint`, `govulncheck`, release configuration
validation, and a provider build. It excludes non-product test suites. Product tests are added only
after the corresponding Nutanix product corpus is structurally complete.

`go fix` is an explicit reviewed modernization operation and never an automatic CI mutation.

## References

- [Go 1.26 release notes](https://go.dev/doc/go1.26)
- [Go toolchain selection](https://go.dev/doc/toolchain)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Go security best practices](https://go.dev/doc/security/best-practices)
- [Dependency policy](dependencies.md)
