# M1 Kernel Design

**Status:** Approved by independent ARC review

**Date:** 2026-08-05

**Beads task:** `ntnx-m1.1`

## Outcome

M1 replaces the empty provider configuration with a secure, environment-aware
configuration contract and implements the reusable kernel needed by every
Nutanix adapter: authentication, TLS, bounded HTTP execution, typed errors,
operation-specific retries, pagination, ETags, task polling, capability
checks, redacted diagnostics, and structured Terraform logs.

M1 still registers no product resource, data source, action, function, or
ephemeral resource. It makes no resource-state, import, or downstream
compatibility claim.

## Inherited decisions

- one Protocol 6 Terraform Plugin Framework binary;
- modular monolith with explicit inward dependency direction;
- hand-written transport, DTOs, schemas, state models, and lifecycle;
- no Nutanix SDK runtime and no OpenAPI code generation;
- locked Developer Portal artifacts are primary API evidence;
- all development and evidence run through `./dev` inside Podman;
- Beads is canonical delivery state and TDD is mandatory.

## Official evidence

The selected Prism artifact is namespace `prism`, version `v4.3`, GA. The
locked OpenAPI body is 825239 bytes with SHA-256
`efb04f7aff22e65b3abaf099d3a4cbd113b27d18375f504ea384ed432bb13976`.

| Concern | Locked Prism v4.3 evidence |
| --- | --- |
| Origins and auth | server `https://{host}:{port}/api`; Basic auth; API-key header `X-ntnx-api-key` |
| Idempotence | `NTNX-Request-Id`, format `uuid`, defined as a safe network-error retry token |
| Async operations | HTTP 202 with `TaskReference` and `Location` |
| Task states | `getTaskById`; OpenAPI path `/prism/v4.3/config/tasks/{extId}` joined to server base `/api`; response `Task` and `TaskStatus` |
| Pagination | zero-based `$page`, bounded `$limit`, OData query fields, totals, and links |
| Concurrency | required `If-Match`, HTTP 428 when absent, and HTTP 412 when stale |
| Throttling | operation-specific `x-rate-limit` entries |
| Errors | English error reference plus `ErrorResponse`, `AppMessage`, and validation shapes |

OpenAPI examples and SDK samples are comparison evidence only. Their generic
retry settings do not override the operation policy defined here.

## Provider configuration

The complete schema is normative in
[`docs/contract.md`](../../contract.md#m1-provider-configuration). Every field
is optional at schema level for environment fallback. Resolution is:

1. use a known, non-null Terraform value;
2. otherwise use the matching environment variable when present;
3. otherwise use the documented effective default;
4. validate the complete effective configuration with path diagnostics.

Unknown and explicit empty values are errors. Secret strings are neither
trimmed nor reproduced. Environment booleans accept only case-insensitive
`true` or `false`; timeouts are base-10 seconds. Tests that mutate environment
use explicit set/unset restoration and never run in parallel.

The endpoint is an HTTPS origin containing a host and optional port, but no
credentials, query, fragment, or non-root path. The normalized origin has no
trailing slash. Multiple PE, PC, or external-plane targets use provider
aliases, preserving one client and capability cache per origin.

Authentication resolves to exactly one immutable mode:

```text
Basic  = username + password + no API key
APIKey = API key + no username + no password
```

Basic usernames reject `:`. API keys contain 1 through 4096 visible ASCII
bytes and reject every invalid header value without echoing it. TLS uses system
roots, optionally appends the configured PEM bundle, sets TLS 1.2 as the
minimum, and verifies the endpoint host. `insecure = true` conflicts with a CA
bundle. Redirects are errors, and proxy selection uses `ProxyFromEnvironment`.

`Configure` constructs clients but performs no network call, keeping provider
loading and offline schema inspection deterministic.

## Package and interface design

### Provider and composition

`internal/provider` owns Framework schema/model types, environment resolution,
cross-attribute validation, diagnostics, and manual composition. It constructs
immutable auth, transport, task, and capability values and passes configured
data to future services. Configured data contains no raw Terraform values and
has no method that reveals credentials.

Each service defines a narrow provider-data port returning configured
namespace clients, task waiters, or capability accessors. `provider` implements
those ports. Services never receive `transport.Client`, cannot construct a
transport operation, and therefore cannot bypass namespace-owned retry,
request-ID, status, body-limit, or path policies.

`internal/task` and `internal/capability` define neutral consumer-side ports and
standard-library values. Namespace adapters import and implement those ports;
the neutral packages never import a namespace or transport. This is the only
permitted dependency direction.

### Authentication

`internal/auth` provides immutable Basic and API-key authenticators using only
the standard library. The consuming transport declares the minimal unexported
authorization interface. Authentication applies headers to one request and
has no environment, URL, logging, retry, or Terraform behavior.

### Transport

`internal/transport` owns one origin-bound `http.Client`. A request contains:

- logical operation and non-secret path template;
- HTTP method and structured path parameters;
- explicitly encoded query and allowlisted headers;
- optional replayable JSON bytes;
- expected success statuses and response-size ceiling;
- retry class and request-ID requirement from the namespace operation.

Each placeholder occupies one complete segment. Construction applies
`url.PathEscape` to each value and sets validated `URL.Path` and `URL.RawPath`,
so Prism task IDs containing `/`, `+`, `=`, and `:` remain one segment on the
wire. Adapters cannot supply a pre-encoded value, free-form absolute path, or
authority. Tests cover every permitted character, traversal, query, fragment,
and double-encoding attempts.

Adapter headers cannot set `Authorization`, `X-ntnx-api-key`, `User-Agent`,
`Host`, `Connection`, `Content-Length`, `Transfer-Encoding`, or
`NTNX-Request-Id`. Authentication, framing, user agent, and request ID belong
to the kernel.

The client rejects absolute URLs, authority escapes, and redirects. It applies
authentication immediately before each attempt, sets JSON headers, propagates
context, limits and closes every body, and returns status, headers, bounded
body, ETag, and safe correlation metadata. Provider and Terraform versions
form a bounded sanitized `User-Agent` that configuration cannot override.

The default success-body ceiling is 16 MiB and the default non-success ceiling
is 1 MiB. A locked namespace operation may set a lower ceiling or obtain ARC
approval for a higher one; the Prism task reader uses its explicit 4 MiB limit.

Transport is vendor-neutral and never decodes Nutanix success or error DTOs.
`internal/nutanix/<namespace>` decodes a bounded response and may wrap the
transport status with validated stable Nutanix codes. Unknown vendor bodies
remain unreported.

### Error taxonomy

- `ConfigurationError`: configuration or TLS construction failure;
- `TransportError`: network, TLS, cancellation, deadline, redirect, request,
  or bounded-body failure before an accepted API response;
- `HTTPError`: vendor-neutral operation, status, request identifier, and retry
  classification, with the bounded body available only to a namespace decoder;
- `TaskError`: failed, canceled, unsupported, or deadline task state, containing
  only validated stable codes and no arbitrary vendor message;
- `CapabilityError`: documented operation unsupported or support indeterminate.

Errors unwrap stable causes where meaningful. Credentials, raw URLs, queries,
headers, bodies, and arbitrary vendor text never enter `Error()` strings.
Callers use types and fields rather than parsing text.

## Retry

| Class | Automatic retry |
| --- | --- |
| `none` | never |
| `read` | classified transport failures and 408, 429, 502, 503, 504 |
| `idempotent-mutation` | same as read, only with locked request-ID evidence and replayable body |

The ceiling is four total attempts. Backoff starts at 500 ms, doubles to 30 s,
and applies full jitter through an injected deterministic delay source. One
delay cannot exceed 30 seconds, and all provider-managed retry delays for a
logical operation cannot exceed 60 seconds. The per-attempt configured timeout
and attempt ceiling bound network time even when caller context has no
deadline.

A valid future `Retry-After` delta or HTTP date is a server minimum, so the
chosen delay is the greater of it and client backoff. If that delay exceeds the
per-delay ceiling, cumulative budget, or remaining caller deadline, no further
attempt starts and the current error is returned. Malformed, overflowing, or
past values fall back to client backoff. Every response body is bounded and
closed before delay calculation or sleeping.

The classifier is fail closed and unwraps `url.Error` and `net.OpError` without
inspecting text. It retries only:

- `io.EOF` or `io.ErrUnexpectedEOF` before an accepted response;
- `ECONNRESET`, `ECONNREFUSED`, `EPIPE`, or `ETIMEDOUT`;
- a timeout or temporary `net.Error` while caller context remains active;
- a timeout or temporary `net.DNSError`, but never name-not-found.

It first rejects caller cancellation/deadline, malformed requests, every
`crypto/tls` or `crypto/x509` validation/handshake failure, non-temporary DNS,
body failures, and unknown errors. HTTP/2 or future errors without one of the
stable classifications are not retried until separately approved. HTTP 401,
403, 404, 409, 412, and 428 are not retried.

One RFC-compatible UUID is reused across every attempt of one idempotent
mutation. Once an attempt returns an accepted task reference, the mutation is
never replayed; control moves to task polling.

Retry classes count provider-managed calls after `RoundTripper` returns. Go's
standard transport may internally recover a stale reused connection before it
returns; that implementation detail is not reported as another provider
attempt. Tests use an injected scripted `RoundTripper` to prove the provider
never initiates a forbidden replay. Unsafe mutation methods do not receive a
standard idempotency header, and request bodies are replayable only for an
operation approved as `idempotent-mutation`.

## Pagination

The generic page walker accepts context, zero-based start, operation limit,
safety ceilings, a fetch function, and a visitor. Namespace adapters own query
fields and DTO decoding.

When a page has a non-negative total, that page's total is authoritative. After
visiting it, `visited >= total` is terminal. A short or empty page while
`visited < total` is an inconsistency error, preventing silent truncation. The
total may increase or decrease between pages as the collection changes; a
decrease to or below visited is terminal, while a negative total is invalid.

Only when total is absent does short- or empty-page fallback terminate. A full
page without total causes one more request, allowing the following empty page
to end APIs with missing metadata. Integer overflow and page or item ceiling
exhaustion are errors. The walker increments `$page` and never follows an
absolute vendor link.

## ETags

Transport extracts `ETag` without parsing it. A namespace operation sends
`If-Match` only when its locked contract requires it. HTTP 412 and 428 are typed
results and never hidden by retry. M1 stores no ETag in Terraform state; every
future persistence or reconcile strategy requires the resource ARC.

## Tasks

`internal/task` defines a snapshot and reader port. `internal/nutanix/prism`
maps Prism v4.3 DTOs into it. The waiter uses no goroutine: read, evaluate, then
wait with an injected context-aware sleeper.

The only M1 Prism reader policy is locked exactly as follows:

| Field | Contract |
| --- | --- |
| Operation | `getTaskById` |
| Request | GET wire path `/api/prism/v4.3/config/tasks/{extId}` with segment-escaped `extId` |
| Projection | `$select=extId,status,progressPercentage,errorMessages,warnings,completionDetails,lastUpdatedTime` |
| Success | HTTP 200 decoded as `prism.v4.3.config.Task` |
| Body ceiling | 4 MiB |
| Retry | `read`; no `NTNX-Request-Id` |
| Locked support | product PC version `2024.3`; deployments `ON_PREM` and `CLOUD` |

M1 does not expose this reader or its waiter to a PE or arbitrary external
plane. A service can obtain it only after a separately approved read-only
plane/version capability establishes the locked PC 2024.3 support contract.
Until that capability exists, M1 tests the reader against the local
deterministic server but no product operation may consume it. A later locked
artifact may approve another version or plane without changing this existing
policy silently.

| Status | Result |
| --- | --- |
| `QUEUED`, `RUNNING`, `CANCELING`, `SUSPENDED` | continue |
| `SUCCEEDED` | success |
| `FAILED`, `CANCELED` | typed failure |
| `$UNKNOWN`, `$REDACTED`, future value | fail closed |

The default interval is two seconds unless a valid server delay is present.
Caller context owns the total timeout. Arbitrary error/warning text is never
copied to an error, diagnostic, or log. A projection may retain only bounded,
character-validated code, group, severity, and attribute path; default
diagnostics use stable codes and generic recovery guidance.

## Capabilities

`internal/capability` is a concurrency-safe cache around named read-only probe
ports. It does not scan, scrape, or infer support from a hostname.

- documented success caches supported;
- explicit documented unsupported caches unsupported;
- 401/403 remain authentication or authorization failures;
- 408/429/5xx remain indeterminate;
- cancellation and deadline remain caller failures;
- only stable positive and explicit negative results are cached.

Product tasks name the exact probe operation and supported plane before a
capability reaches a Terraform type.

## Diagnostics and logging

Framework diagnostics identify configuration paths without values. Runtime
diagnostics contain safe operation, status class, correlation ID when present,
stable code, and generic recovery guidance. They never dump vendor responses.

Transport uses `tflog` with operation, method, path template, attempt, status,
duration, and request correlation ID only. Raw paths, query strings, object
names, ext-IDs, headers, bodies, and provider configuration are excluded.
OpenTelemetry is deferred.

## Test contract

All commands run through `./dev`.

1. Provider tests cover null, unknown, explicit-over-env, auth modes, partial
   and conflicting auth, username/API-key validation, endpoint validation,
   strict env parsing, PEM errors, insecure conflict, and redaction. Env tests
   restore every value and are not parallel.
2. Auth tests inspect real request headers and prove secret-safe errors.
3. TLS/transport tests use `httptest.Server` for cancellation, limits,
   redirects, closure, auth, ETags, user agent, reserved headers, and envelopes.
4. Retry/task policy tests use Go 1.26 `testing/synctest` without network,
   covering the complete error classifier, deadlines, no-deadline budgets,
   malformed/past/overflowing and both valid `Retry-After` forms, response
   closure between attempts, ceilings, stable IDs, and every task state.
5. Pagination tests cover totals present/absent/changing/negative,
   full/short/empty pages, overflow, cancellation, and ceilings.
6. Fuzz targets cover endpoints, path parameters, `Retry-After`, bounded error
   decoding, and pagination metadata as the parsers arrive.
7. Protocol 6 acceptance uses `ProtoV6ProviderFactories` and deterministic
   local endpoints for schema, sensitive fields, env fallback, and diagnostics.
8. Live PE/PC acceptance remains separate, read-only, explicitly authorized,
   and cannot report an unavailable target as passed.

Fail-safe tests prove: retry `none` never replays; mutations without locked
request-ID evidence never replay; non-replayable bodies fail before retry;
reserved headers and path/authority escapes are rejected; capability
401/403/429/5xx/canceled/deadline/indeterminate results are not cached; and
supported or explicit unsupported results are cached exactly once under
concurrent callers.

Every error, diagnostic, and `tflog` redaction test injects unique canary
secrets into credentials, queries, headers, bodies, object names, IDs, and
vendor messages, then searches all rendered outputs for every canary.

The full `./dev task all` gate remains mandatory after every M1 task.

## Delivery decomposition

Exactly one Beads task is in progress:

1. contract, ADR, plan, and independent ARC approval;
2. provider configuration and Framework diagnostics;
3. auth, TLS, and origin-bound HTTP client;
4. typed responses, errors, redaction, and logging;
5. operation policies, request IDs, retry, and deadline budget;
6. pagination and ETags;
7. Prism task adapter and waiter;
8. capability registry and probes;
9. Protocol 6 acceptance, fuzz smoke, dependency audit, and evidence.

No product Terraform type enters M1.

## References

- [Terraform provider contract](../../contract.md)
- [Provider architecture](../../architecture.md)
- [Go dependency policy](../../standards/dependencies.md)
- [Go 1.26 engineering standard](../../standards/go-1.26.md)
- [Testing standard](../../standards/testing.md)
- [Nutanix artifact standard](../../standards/nutanix-artifacts.md)
- [ADR 0006](../../adr/0006-m1-kernel-contract.md)
- [Locked Nutanix manifest](../../../specs/nutanix/manifest.json)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform Plugin Log](https://developer.hashicorp.com/terraform/plugin/log/writing)
- [Nutanix Prism v4.3 OpenAPI](https://developers.nutanix.com/api/v1/namespaces/prism/versions/v4.3/yaml)
