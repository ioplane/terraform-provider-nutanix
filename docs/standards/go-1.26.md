# Go 1.26 engineering standard

## Toolchain contract

- `go.mod` declares `go 1.26.0`.
- The development container supplies exactly Go 1.26.5.
- The same container supplies exactly `gopls` 0.23.0. Its module declares Go
  1.26.0, so it runs on the pinned Go 1.26.5 toolchain without toolchain
  switching.
- `GOTOOLCHAIN=local` prevents automatic toolchain downloads or switches.
- Normal builds and releases use `CGO_ENABLED=0`.
- Race tests run in a separate container target with cgo and the container C
  compiler enabled.
- Experimental `GOEXPERIMENT` features are outside the supported build.

Go commands run only inside the repository's Podman development environment.
The host Go toolchain is never completion evidence.

The `go` directive is the module's minimum required Go version, including the
minimum language and module semantics. It is not the selected build-patch pin.
The immutable development image and `GOTOOLCHAIN=local` select Go 1.26.5 and
reject an implicit download or switch. Do not rewrite `go 1.26.0` to mirror an
installed patch without intentionally raising the minimum version accepted by
the module.

## 2026-08-05 release audit

Go 1.26.5, released 2026-07-07, is the current stable release. The repository,
development container, `go version -m` build evidence, and `gopls` all use the
current 1.26 line; `gopls` 0.23.0 is also current. Every direct dependency is
at its newest published version and declares a Go baseline no newer than
1.25.8.

The Go 1.26.1 through 1.26.5 patch line contains security fixes affecting
packages and tools relevant to a network provider, including `crypto/tls`,
`crypto/x509`, `html/template`, `net`, `net/http`, `net/url`, `os`,
`archive/tar`, the compiler, and the `go` command. Building with 1.26.0 merely
because it is the module directive would omit those fixes. The exact 1.26.5
toolchain pin is therefore a security requirement, not a formatting choice.

Go 1.26 also changes `go mod init` to choose an older default module baseline.
Repository bootstrap must continue to pass the intended version explicitly.
`gopls` and `go vet` now share analyzer implementations, so both remain useful
gates but their duplicate findings should not be counted as independent
evidence. `go fix` gained broader modernization behavior and remains a
reviewed, explicit operation rather than an automatic CI mutation.

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

## Required Go gates

During main product-corpus implementation, the lightweight containerized gate
includes formatting, `go vet ./...`, repository linting, `govulncheck ./...`,
and `go build ./...`. It does not add or run tests.

After the corresponding product corpus is complete, its separate product-test
gate may include only approved Nutanix product behavior:

1. Terraform schema and lifecycle behavior;
2. exact API mapping and state behavior;
3. race checks where concurrent product code exists;
4. bounded fuzz smoke checks for product parsers where approved;
5. authorized product acceptance.

Existing non-product tests remain frozen and outside the default delivery
gate. Repository automation, containers, Beads, documentation, roadmap, CI
wiring, and policy scripts do not gain tests.

Go 1.26 `go fix` is an explicit, reviewed modernization operation. It is not
an automatic mutating CI step, and its diff must pass the full gate.

## References

- [Go 1.26 release notes](https://go.dev/doc/go1.26)
- [Go release history source](https://github.com/golang/website/blob/master/internal/history/release.go)
- [Go 1.26.4 to 1.26.5 changes](https://github.com/golang/go/compare/go1.26.4...go1.26.5)
- [Go toolchain selection](https://go.dev/doc/toolchain)
- [Effective Go](https://go.dev/doc/effective_go)
- [Organizing a Go module](https://go.dev/doc/modules/layout)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [Go security best practices](https://go.dev/doc/security/best-practices)
- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
- [Go dependency policy](dependencies.md)
