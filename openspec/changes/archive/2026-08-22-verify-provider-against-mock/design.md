## Context

See proposal.md ("Why") for motivation and the two capability specs for the behavior contract.

The relevant discovery is that `terraform-plugin-testing` drives a real `terraform` binary through
the **reattach** mechanism (`TF_REATTACH_PROVIDERS`) rather than by installing a provider from the
Registry. That was verified in this environment, where `registry.terraform.io` is unreachable: a
real Terraform run against an in-process provider succeeds anyway. So the expensive-sounding part
of this change — running actual Terraform — turns out to need only a binary on disk, no network and
no published provider.

What Terraform adds over the existing unit tests is its own consistency contract. Terraform, not
the test author, checks that every attribute in the plan matches what apply returned, that a
refresh does not silently change state, and that imported state matches applied state. Those are
exactly the provider bugs a client-level unit test cannot see.

## Goals / Non-Goals

**Goals:**
- Catch the Terraform-contract failures — inconsistent result after apply, non-empty plan after
  apply, import mismatch — automatically, offline, on every commit.
- Make the mock good enough that passing against it is meaningful, and honest enough that it cannot
  paper over a provider mistake.
- Keep `go test ./...` runnable with no Terraform binary and no Docker.

**Non-Goals:**
- Verifying the provider against the real RevenueCat service. The mock encodes the same assumed
  contract as the client, so agreement between them proves self-consistency, not correctness. This
  limit is stated rather than glossed over.
- Reimplementing RevenueCat's business rules (pricing, store validation, customer state). The mock
  covers catalog structure only.

## Decisions

### One mock implementation, two entry points

The mock is an `http.Handler` in `internal/mockrevenuecat`, wrapped by `cmd/mock-revenuecat` for
standalone use. Go tests mount it with `httptest.NewServer`; Docker Compose runs the binary. A
second implementation for containers would drift from the one the tests use, and the drift would be
invisible until it mattered.

### The mock is stateful, not a canned-response stub

Returning fixed payloads would let the provider pass while sending nonsense: a stub cannot notice
that Create and Read disagree, or that a delete never happened. A real store makes the mock able to
contradict the provider — which is the only way these tests can fail for a good reason. It also
means the assertions are about observable behavior ("read it back and it is gone") rather than
about request shapes the unit tests already cover.

### The mock rejects unauthenticated requests and mismatched parents

Both are deliberate tripwires. Without the `401`, a provider that dropped its `Authorization`
header would sail through every end-to-end test. Without parent scoping, a provider that addressed
an entitlement through the wrong project would too. A fake that accepts anything verifies nothing.

### Fault injection is opt-in and counted

Rate-limit injection is expressed as "fail the next N requests with 429", not as a probability.
A deterministic fault keeps the retry test from being flaky, and flakiness in a verification suite
is worse than not having the test: it trains people to re-run rather than to look.

### Terraform-driven tests reuse `TF_ACC`, and skip loudly

They run under `TF_ACC=1` like any acceptance test, but point `base_url` at the mock instead of
requiring an API key, so they need no credentials. When `TF_ACC` is set but no binary is found, they
skip with a message naming what to install — a silent skip in a suite whose whole purpose is
verification would be self-defeating.

`terraform-plugin-testing` locates the binary from `TF_ACC_TERRAFORM_PATH`, and the tests probe for
it up front to produce the actionable message rather than a failure from inside a harness.

### Compose runs the real example, not a bespoke configuration

The Compose stack applies `examples/complete` unmodified apart from an endpoint override. Testing a
configuration written for the test would verify the test; testing the shipped example verifies the
documentation, which is what actually goes stale.

### Two CI jobs, not one

Unit tests stay a separate fast job. The Terraform job installs a binary and runs the end-to-end
suite. Keeping them apart means a failure names itself — "the provider is inconsistent under
Terraform" reads differently from "a unit test broke" — and the fast job still gates quickly.

## Risks / Trade-offs

- **The mock encodes the same assumptions as the client, so both can be wrong together** → This is
  the central limitation and is documented in the README rather than left implicit: these tests
  prove self-consistency, not fidelity to RevenueCat. Mitigated only in that a wrong assumption is
  now wrong in one place and correcting it breaks the tests loudly.
- **The mock drifts from the real API as RevenueCat changes** → It lives beside the client it
  mirrors, and the same correction touches both. Nothing prevents drift; the tests simply keep
  claiming self-consistency, which is all they ever claimed.
- **Docker Compose cannot be exercised in this environment** (a Docker daemon is not available here,
  only the CLI) → The Compose path is authored and wired into CI, where it does run, but it is
  reported as unverified locally rather than as working. The Go end-to-end tests, which run here,
  cover the same provider behavior; Compose adds container-level packaging confidence.
- **A Terraform binary is an extra dependency for contributors** → Everything degrades to skips,
  so the default workflow is unchanged for anyone who does not want it.

## Open Questions

None. The mechanism is verified to work in this environment, and the remaining choices are
mechanical.
