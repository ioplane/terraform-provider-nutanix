# Product verification standard

## Product-first sequence

1. Define the API, Terraform, state, identity, and lifecycle contract.
2. Implement the complete hand-written product corpus.
3. Use formatting, static analysis, vulnerability scanning, artifact validation, and compilation
   during implementation.
4. Add tests for the completed product behavior.
5. Run authorized acceptance only in the dedicated acceptance phase.

Repository automation, launchers, documentation, workflow wiring, and policy scripts do not receive
new test suites. Product tests cover Terraform schema and lifecycle, API mapping, state, import,
drift, and authorized Nutanix behavior.

## Evidence hierarchy

| Level | Evidence | Purpose |
| --- | --- | --- |
| Static | Formatting, lint, vet, vulnerability scan, API lock, docs and build | Implementation safety |
| Deterministic product | Local HTTP server and product fixtures | Request, response, state, error and redaction behavior |
| Terraform lifecycle | Protocol 6 and Terraform Plugin Testing | Plan, apply, refresh, import and state behavior |
| Live product | Authorized isolated Nutanix target | End-to-end compatibility and cleanup |

Mock call choreography is not acceptance evidence. Deterministic servers and fixtures must represent
documented product behavior and stable error contracts.

## Execution boundary

All Go, Terraform, automation, build, packaging, and product-test commands run inside Podman
through `./dev`. Host toolchains are not completion evidence. The launcher and repository gates
are Go-based.

The default implementation gate is:

```bash
./dev task all
```

It intentionally excludes Go unit tests, fuzzing, race tests, Protocol acceptance, package
acceptance, and live acceptance until their owning product phase requires them.

## Protocol 6 and ABI-sensitive boundary

The provider executable serves Terraform Plugin Protocol 6 through
`providerserver.Serve`. Provider-boundary changes must run the pinned container check:

```bash
./dev task go:test:protocol
```

This check uses Terraform 1.15.8 and exercises the Protocol 6 server, provider schema,
configuration, and sensitive-diagnostic redaction without contacting Nutanix. It verifies the
framework/protocol boundary and build compatibility; it does not prove API payload compatibility,
Terraform lifecycle behavior for every registered object, or live-product acceptance. Run the
focused Go tests, race tests, package gate, and authorized product gates separately when their
owning change requires them.

## Live acceptance

Live tests require all of the following:

- an isolated non-production target;
- explicit authorization for every mutation class;
- collision-free test identity;
- bounded execution and retries;
- deterministic cleanup behavior;
- evidence for the tested outcome and successful cleanup.

A skipped, unavailable, or partially cleaned live gate is not passed evidence.

## Container-backed Go integration tests

Container-bound behavior uses `testcontainers-go` from Go tests with the explicit `ProviderPodman`
provider, one suite-level container, bounded readiness, strict configuration from
`config/testing.yaml`, and cleanup registered through `t.Cleanup`. The repository disables Ryuk for
this single bounded test because the rootful Podman API does not provide Docker's `bridge` network;
the test's explicit cleanup remains mandatory. The explicit gate is:

```bash
./dev task go:test:containers
```

The default provider HTTP tests remain in-process and do not make outbound product calls.

## Service contract coverage

Every registered data source and resource has a package-local contract test. These deterministic
tests verify the Framework metadata and schema shape, exercise the state projection with empty or
partially populated API models, and cover the invalid-identity or incomplete-response boundary
where the service exposes one. The shared `internal/service/testkit` assertions keep metadata and
attribute-mode checks consistent without coupling service packages to one another.

This layer proves provider-side schema and mapping behavior only. It does not replace the local
HTTP fixture tests, Terraform lifecycle tests, product acceptance, or live Nutanix verification.

## CI/CD

GitHub Actions invokes `./dev task all` for pull requests and pushes to the `dev` integration or
`main` release branch. Release artifacts are built inside the same Podman boundary after a Release
Please PR creates a SemVer tag. If GitLab CI is added, jobs invoke the Go automation binary with
explicit inputs, deterministic exit codes, and concise output.

## References

- [Provider contract](../contract.md)
- [Release process](../release-process.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform Plugin Testing](https://developer.hashicorp.com/terraform/plugin/testing)
