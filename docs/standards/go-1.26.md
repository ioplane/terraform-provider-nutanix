# Go 1.26 engineering standard

## Toolchain contract

- `go.mod` declares `go 1.26.0`.
- The development container supplies exactly Go 1.26.5.
- `GOTOOLCHAIN=local` prevents automatic toolchain downloads or switches.
- Normal builds and releases use `CGO_ENABLED=0`.
- Race tests run in a separate container target with cgo and the container C
  compiler enabled.
- Experimental `GOEXPERIMENT` features are outside the supported build.

Go commands run only inside the repository's Podman development environment.
The host Go toolchain is never completion evidence.

## Packages and dependencies

Production Go code stays under `cmd` and `internal`; the module promises no
public Go library API. Packages are small, lower-case, single-word units with
one concrete responsibility. The dependency boundaries in the
[provider architecture](../architecture.md) apply to every package.

Use Effective Go as baseline idiom guidance alongside the current Go release,
module-layout, code-review, and security guidance linked below.

Prefer the standard library. Add a dependency only when its benefit justifies
its maintenance and security surface. Declare an interface in the consuming
package at the point of use. Do not create an interface next to an
implementation solely to enable mocking.

The exact M1 module budget, deferred candidates, and rejected abstractions are
defined in the [Go dependency policy](dependencies.md).

## Context, errors, and concurrency

- `context.Context` is the first argument to request-bound work. Propagate it
  through every call and into every HTTP request; never store it in a struct.
- Add concise operation context to errors and preserve the cause with `%w`
  when callers may inspect or unwrap it. Handle an error at one level rather
  than logging and returning the same failure repeatedly.
- Error text starts with a lower-case letter and has no terminal punctuation.
  Do not parse arbitrary error text as control flow.
- Panic is not normal error handling. Use it only for an unrecoverable
  programmer invariant, never for a remote, configuration, or user error.
- Every goroutine has an owner, a bounded lifetime, a cancellation or shutdown
  path, and a rule for collecting its result. Unbounded background work and
  leaked goroutines are forbidden.

## HTTP behavior

HTTP clients set explicit timeouts and preserve caller cancellation. Every
response body is closed on every path, including non-success responses. Limit
and validate bodies before decoding when an endpoint can return unbounded
data.

Logs, diagnostics, and returned errors redact credentials, authorization
headers, sensitive query values, request bodies, response bodies, and provider
configuration. Tests cover cancellation, timeout, body closure, and redaction
for shared transport behavior.

## Required Go gate

The containerized gate includes:

1. formatting;
2. `go vet ./...`;
3. unit tests;
4. race tests in the cgo-enabled race target;
5. repository linting;
6. `govulncheck ./...`;
7. bounded fuzz smoke tests for fuzz targets that exist.

Parsers, pagination, filter construction, state upgrades, and remote error
decoding gain native fuzz targets as they are introduced. CI runs bounded
smoke fuzzing; longer fuzz runs belong in a scheduled gate.

Go 1.26 `go fix` is an explicit, reviewed modernization operation. It is not
an automatic mutating CI step, and its diff must pass the full gate.

## References

- [Go 1.26 release notes](https://go.dev/doc/go1.26)
- [Effective Go](https://go.dev/doc/effective_go)
- [Organizing a Go module](https://go.dev/doc/modules/layout)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Go security best practices](https://go.dev/doc/security/best-practices)
- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Go dependency policy](dependencies.md)
