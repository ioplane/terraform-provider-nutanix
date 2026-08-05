# Product verification standard

## Development sequence

Implementation is product-first:

1. implement the main hand-written provider corpus;
2. use formatting, static analysis, compilation, and artifact validation during
   implementation;
3. add tests only after the corresponding Nutanix product implementation is
   structurally complete;
4. run product acceptance in the dedicated acceptance phase.

Tests cover Terraform schema and lifecycle, API mapping, state, import, and
authorized Nutanix product behavior. Repository automation, launchers, Beads,
documentation, roadmap generation, CI wiring, and policy scripts do not receive
test suites. Existing non-product tests are frozen, excluded from the default
gate, and removed when their owning tooling is replaced.

Product tests prefer real behavior at the narrowest practical boundary. Local
servers and deterministic product fixtures are acceptable; mock call
choreography is not acceptance evidence.

## Execution boundary

All Go, Terraform, Python CLI, build, static-analysis, packaging, and later
product-test commands run inside Podman through the repository launcher. Host
toolchains are not completion evidence.

## Gates

The default implementation gate is intentionally small: formatting, static
analysis, dependency and artifact validation, provider compilation, and
documentation/tracker consistency. It does not run Python tooling tests, Go
unit tests, fuzzing, race tests, Protocol acceptance, or package acceptance.

The later product-acceptance gate contains only tests tied to implemented
Nutanix product behavior. A required product test that does not run cannot be
reported as passed.

## GitLab CI/CD

GitLab jobs invoke small role-oriented Python CLI modules. Each module has one
responsibility, explicit inputs, deterministic exit codes, and concise output.
CI automation is not a general Python framework and has no separate test suite.

## Live acceptance boundary

Live acceptance tests require an approved contract that identifies an
isolated non-production target, explicit authorization, collision-free test
identity, bounded execution, and cleanup behavior. Evidence must show both the
tested outcome and successful cleanup. A skipped or unavailable live gate
cannot be reported as passed.

## References

- [Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)
- [Terraform plugin protocol](https://developer.hashicorp.com/terraform/plugin/terraform-plugin-protocol)
