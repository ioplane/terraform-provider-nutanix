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

All Go, Terraform, Python CLI, build, packaging, and product-test commands run inside Podman through
`./dev`. Host toolchains are not completion evidence.

The default implementation gate is:

```bash
./dev task all
```

It intentionally excludes Python tooling tests, Go unit tests, fuzzing, race tests, Protocol
acceptance, package acceptance, and live acceptance until their owning product phase requires them.

## Live acceptance

Live tests require all of the following:

- an isolated non-production target;
- explicit authorization for every mutation class;
- collision-free test identity;
- bounded execution and retries;
- deterministic cleanup behavior;
- evidence for the tested outcome and successful cleanup.

A skipped, unavailable, or partially cleaned live gate is not passed evidence.

## CI/CD

GitHub Actions invokes `./dev task all` for pull requests and `main`. Release artifacts are built
inside the same Podman boundary after a Release Please PR creates a SemVer tag. If GitLab CI is
added, jobs invoke small role-oriented Python CLI modules with explicit inputs, deterministic exit
codes, and concise output.

## References

- [Provider contract](../contract.md)
- [Release process](../release-process.md)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform Plugin Testing](https://developer.hashicorp.com/terraform/plugin/testing)
