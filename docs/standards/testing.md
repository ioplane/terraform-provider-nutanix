# Testing standard

## Development cycle

Production behavior follows red, green, refactor:

1. write the smallest test that expresses the required behavior;
2. run it and confirm that it fails for the expected missing behavior, not for
   a syntax, fixture, environment, or setup error;
3. implement the smallest change that makes the test pass;
4. run the relevant test and broader gate;
5. refactor while keeping the tests green.

Prefer tests of real behavior at the narrowest practical boundary. Use fakes,
local test servers, and deterministic fixtures where they preserve the real
protocol. Do not replace behavior with mocks merely to make a test easy to
write, and do not test mock call choreography as a substitute for outcomes.

Configuration and prose are exceptions to test-first sequencing. Their final
normative content, generated output, formatting, and internal links must still
be checked by the repository gate before M0 closes.

## Execution boundary

All tests, Go commands, Terraform protocol checks, linters, security checks,
fuzzing, and packaging run inside Podman through the repository launcher. Host
toolchains are not completion evidence.

## Required gates

The applicable containerized gate includes:

- focused and full unit tests;
- race tests in the cgo-enabled race target;
- `go vet ./...` and repository linting;
- `govulncheck ./...`;
- bounded fuzz smoke tests for available fuzz targets;
- Terraform Plugin Protocol 6 loading and handshake checks;
- deterministic package and consumer-install checks;
- repository content, generated-output, and internal-link checks;
- a clean-clone bootstrap and full-gate proof.

A required gate that does not run is not green. Never convert a failure into a
pass by skipping, weakening, deleting, or silently excluding the required test.
Record the failure and leave status incomplete until the gate genuinely passes.

## Live acceptance boundary

M0 uses no live Prism Element or Prism Central system. Its protocol, package,
transport, and repository checks use local, deterministic fixtures.

Later live acceptance tests require an approved contract that identifies an
isolated non-production target, explicit authorization, collision-free test
identity, bounded execution, and cleanup behavior. Evidence must show both the
tested outcome and successful cleanup. A skipped or unavailable live gate
cannot be reported as passed.

## References

- [Go fuzzing](https://go.dev/doc/security/fuzz/)
- [Go security best practices](https://go.dev/doc/security/best-practices)
- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform plugin protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
- [Approved foundation design](../superpowers/specs/2026-08-04-foundation-design.md)
