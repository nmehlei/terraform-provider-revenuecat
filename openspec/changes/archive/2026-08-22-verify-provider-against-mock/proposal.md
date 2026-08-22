## Why

The provider currently ships with an explicit caveat: its encoding of the RevenueCat API v2
contract has never been exercised by a real `terraform` binary, and the unit tests — which call the
client directly against `httptest` fakes — cannot catch the failure modes that only appear when
Terraform itself is in the loop. A resource whose model does not round-trip produces
"Provider produced inconsistent result after apply"; a Read that normalizes a value differently
from Create produces a permanent non-empty plan; an import that populates the wrong attributes
fails `ImportStateVerify`. None of those are visible today.

A real RevenueCat account cannot be used for this, and would be the wrong tool anyway: the checks
above are about the provider's internal consistency under Terraform's contract, not about the
remote service. A stateful fake of the API makes them runnable by anyone, offline, on every commit.

## What Changes

- Add `internal/mockrevenuecat`: a stateful in-memory implementation of the RevenueCat API v2
  catalog endpoints — real create/read/update/delete semantics, generated identifiers and
  timestamps, cursor pagination, attach/detach tracking, and `404` for absent objects. It is an
  `http.Handler`, usable both in-process from Go tests and as a standalone server.
- Add `cmd/mock-revenuecat`: a standalone binary serving that handler, with configurable address,
  seeded project, and fault injection for rate limiting.
- Add end-to-end acceptance tests that run a **real `terraform` binary** against the mock, driving
  the full lifecycle for every resource: apply, verify no drift on refresh, update in place, verify
  replacement on identity change, import and verify, and destroy.
- Add a Docker Compose stack that runs the mock alongside a container which applies the repository's
  own `examples/complete` configuration end to end and asserts a clean second plan.
- Extend CI with jobs that run the Terraform-driven acceptance tests and the Compose stack, so both
  run on every push and pull request.
- Update the README and provider documentation to describe what is now verified and what remains
  unverified, replacing the blanket "unverified" caveat with an accurate one.

Not in scope: verifying the provider against the real RevenueCat service, which still requires a
live API key; and testing customer-facing runtime endpoints, which the provider does not manage.

## Capabilities

### New Capabilities
- `mock-api-server`: the behavior of the stateful RevenueCat API fake — the semantics it
  guarantees, the faults it can inject, and how it is configured and seeded.
- `end-to-end-verification`: what a Terraform-driven run against the mock must prove about the
  provider, and the conditions under which those checks run or are skipped.

### Modified Capabilities

None. This change adds verification machinery; it does not change how the provider behaves.

## Impact

- New packages `internal/mockrevenuecat` and `cmd/mock-revenuecat`; no change to
  `internal/revenuecat` or `internal/provider` production code.
- New test-only dependency on a real `terraform` binary, resolved in CI by `setup-terraform` and
  locally by whatever is on `PATH`. Tests skip with a clear message when it is absent, so
  `go test ./...` stays runnable on a machine without Terraform.
- New `Dockerfile`, `docker-compose.yaml` and an end-to-end script; new CI jobs.
- The provider's honesty caveat becomes narrower and more accurate rather than disappearing: the
  contract is still unverified against the live service, but the provider is now known to be
  self-consistent under Terraform.
