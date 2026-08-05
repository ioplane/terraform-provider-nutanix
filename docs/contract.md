# Terraform provider contract

## Provider identity

| Property | Contract |
| --- | --- |
| Terraform Registry address | `ioplane/nutanix` |
| Provider type | `nutanix` |
| Provider implementation | Terraform Plugin Framework |
| Provider protocol | Terraform Plugin Protocol 6 |

The protocol version defines compatibility between Terraform CLI and the
provider binary. It does not version Nutanix APIs or public Terraform types.

## Public compatibility

The following are compatibility contracts once a Terraform type is released:

- public resource and data-source names;
- schema names, nesting, types, optionality, computed behavior, and
  sensitivity;
- values persisted in state and their meaning;
- mapping between Terraform state and Nutanix remote identity;
- import grammar and import result;
- state-upgrade paths and preserved state semantics.

Terraform types use `nutanix_<domain>_<noun>` in lower snake case. A public
name does not include an API version merely because its current Nutanix
transport does. A version suffix is allowed only when an existing compatibility
contract requires it.

The Nutanix external identifier (ext-ID), represented by `ext_id` in v4 APIs,
is the canonical remote identity when the selected API exposes it. Every type
contract defines how that identity maps to Terraform state and import syntax.
It must not silently substitute a display name or another mutable field.

Each future type contract defines attribute-level null and unknown behavior,
sensitivity, secret handling, drift behavior, replacement boundaries, and
state persistence. This repository-wide contract supplies no implicit defaults
for those choices.

## M1 provider configuration

M1 introduces the provider configuration contract below. Every attribute is
optional in the Terraform schema so the equivalent environment variable can
supply it. Configuration resolution happens during `Configure`; a missing or
ambiguous effective value is an attribute-scoped error.

| Attribute | Terraform type | Sensitive | Environment variable | Effective default | Contract |
| --- | --- | --- | --- | --- | --- |
| `endpoint` | string | no | `NUTANIX_ENDPOINT` | none | HTTPS origin for one PE, PC, or supported external plane; required after resolution |
| `username` | string | no | `NUTANIX_USERNAME` | none | Basic-auth username; must be paired with `password` |
| `password` | string | yes | `NUTANIX_PASSWORD` | none | Basic-auth password; must be paired with `username` |
| `api_key` | string | yes | `NUTANIX_API_KEY` | none | Value sent only as `X-ntnx-api-key` |
| `insecure` | bool | no | `NUTANIX_INSECURE` | `false` | Disables certificate verification only when explicitly true |
| `ca_certificate` | string | yes | `NUTANIX_CA_CERTIFICATE` | system roots | PEM CA bundle appended to the system trust pool |
| `request_timeout_seconds` | integer | no | `NUTANIX_REQUEST_TIMEOUT_SECONDS` | `60` | Per-attempt HTTP deadline in the inclusive range 1 through 600 |

Explicit Terraform values take precedence over the matching environment
variable one attribute at a time. Null means “use the environment or default.”
Unknown provider values are rejected because credentials, TLS behavior, and
the target origin must be fixed before a client can be constructed. Explicit
empty strings are invalid; secret values are never trimmed, echoed, or added
to diagnostics.

Exactly one complete authentication mode is required after resolution:

- Basic authentication uses both `username` and `password`;
- API-key authentication uses only `api_key`;
- partial Basic credentials, both complete modes, or no complete mode are
  configuration errors.

Basic usernames may not contain `:`. API keys contain 1 through 4096 visible
ASCII bytes (`0x21` through `0x7e`); control characters, whitespace, non-ASCII
bytes, and invalid HTTP header values are rejected without reproducing the
value in a diagnostic.

`endpoint` must use `https`, contain a host, and contain no user information,
query, fragment, or non-root path. The port is part of the origin, so PE/PC
port 9440 and external-plane port 443 require no separate provider attribute.
Provider aliases represent multiple endpoints; one configured provider does
not fan out across unrelated control planes.

`insecure = true` and `ca_certificate` conflict. TLS verification is enabled
by default, TLS 1.2 is the minimum, redirects are not followed, and the
standard `HTTPS_PROXY` and `NO_PROXY` environment variables are honored.
There is no API-version, retry-count, poll-interval, task-timeout, proxy, or
telemetry attribute in M1. Namespace versions are internal artifact-backed
choices; resource and action timeouts belong to their own future contracts.

Provider configuration is not Terraform resource state. M1 persists no
endpoint, credential, CA material, retry token, ETag, task reference, or
capability result in state. It defines no remote identity, import grammar, or
state-upgrade path.

## M1 kernel behavior

The hand-written kernel obeys these public-behavior constraints:

- request paths are relative `/api/...` templates resolved against the
  configured origin; structured parameters are escaped as single path
  segments, and an operation cannot redirect or escape to another authority;
- Basic and API-key credentials are applied immediately before a request and
  never enter URLs, DTOs, logs, returned errors, or Terraform diagnostics;
- requests identify the provider and Terraform versions in a bounded,
  sanitized `User-Agent`; user configuration cannot override it;
- each operation selects an explicit retry class. HTTP method alone never
  makes a request retryable;
- read, list, and task-poll operations may retry eligible transport failures
  and HTTP 408, 429, 502, 503, and 504 responses;
- a mutation may retry only when its locked operation contract declares
  `NTNX-Request-Id` idempotence. One RFC-compatible UUID is generated for the
  logical operation and reused for every attempt;
- retry honors both forms of `Retry-After`, caller cancellation, the operation
  deadline, a four-attempt ceiling, a 30-second per-delay ceiling, a 60-second
  cumulative delay budget, and bounded exponential full jitter;
- HTTP 401, 403, 404, 409, 412, and 428 are never retried automatically. ETag
  conflicts return to the owning service for an explicit re-read decision;
- response and error bodies are bounded, closed on every path, decoded only by
  the owning namespace, and excluded from logs and default diagnostics;
- default success and non-success ceilings are 16 MiB and 1 MiB respectively;
  an operation-specific contract may select another reviewed ceiling;
- a response body is closed before any provider-managed retry delay begins;
- namespace adapters cannot set authentication, user-agent, framing, host, or
  request-ID headers owned by the kernel;
- pagination begins at page zero, uses an operation-specific limit, accepts
  optional `totalAvailableResults`, and falls back to short-page then empty-page
  termination. It never follows an arbitrary pagination URL and has explicit
  page and item safety ceilings;
- task polling is synchronous and context-bound. `QUEUED`, `RUNNING`,
  `CANCELING`, and `SUSPENDED` continue; `SUCCEEDED` succeeds; `FAILED` and
  `CANCELED` return typed task errors; unknown or redacted states fail closed;
- ETags are transport concurrency tokens, not implicit public state. A future
  resource contract must explicitly decide whether private or public state is
  needed;
- capability checks use documented, read-only operation probes and a
  per-provider cache. Authentication, authorization, throttling, and server
  failures are not misreported as unsupported capabilities.

Logs use Terraform `tflog` context and allowlisted structured fields only:
operation name, HTTP method, path template, attempt, status, duration, and
request correlation identifier. Raw URLs, queries, headers, bodies,
credentials, object names, and remote identifiers are excluded.

## Design gate

Before implementation begins, every Terraform type requires an approved ARC
contract. For resources and data sources, that contract covers:

1. public name and complete schema;
2. state model and lifecycle semantics;
3. remote identity and `ext_id` mapping;
4. import grammar and state-upgrade obligations;
5. positive, negative, drift, and compatibility tests.

The contract names the exact locked Nutanix namespace, version, operations,
and schemas used as evidence. Implementation remains hand-written and follows
the dependency boundaries in [the provider architecture](architecture.md).

## M0 boundary

M0 registers zero resources, data sources, actions, functions, and ephemeral
resources. It proves only the empty provider foundation and protocol 6 delivery
controls. M1 replaces the empty provider schema with the configuration contract
above but still registers zero product resources, data sources, actions,
functions, and ephemeral resources.

Legacy state inventory, state migration, and full downstream compatibility are
M7 gates, not M0 completion claims.

## References

- [Approved foundation design](superpowers/specs/2026-08-04-foundation-design.md)
- [Approved foundation implementation plan](superpowers/plans/2026-08-04-foundation.md)
- [M1 kernel design](superpowers/specs/2026-08-05-m1-kernel-design.md)
- [Go dependency policy](standards/dependencies.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform plugin protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
