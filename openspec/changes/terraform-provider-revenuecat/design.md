## Context

Greenfield repository — no existing Go code, no provider scaffolding. See proposal.md ("Why") for
motivation and the five capability specs under `specs/` for the behavior contract.

Two constraints shape the approach:

1. **The API contract cannot be verified in this environment.** Network egress blocks
   `revenuecat.com` and `api.revenuecat.com`, so endpoint paths and payload field names are encoded
   from documented knowledge of RevenueCat REST API v2 rather than read off the live reference.
   Any of them could be wrong in detail.
2. **Terraform is not installed here.** Acceptance tests that drive a real `terraform` binary
   cannot run, so correctness has to be demonstrated some other way.

Together these mean the design's first job is to make a wrong endpoint or field name cheap to fix
and impossible to spread.

## Goals / Non-Goals

**Goals:**
- Confine every fact about RevenueCat's wire format to one package, so a correction is a one-file
  change rather than an audit of every resource.
- Make resource behavior testable without Terraform and without network access.
- Give each resource one obvious shape, so adding the sixth resource is mechanical rather than
  another design exercise.

**Non-Goals:**
- Registry publication, signing keys, or release automation beyond a CI build.
- Runtime/customer-facing endpoints (customers, subscriptions, purchases, invoices).
- Generating the client from an OpenAPI document — the document is not reachable from here, and a
  hand-written client of this size stays readable.

## Decisions

### Plugin Framework over SDKv2

Use `terraform-plugin-framework`. It is HashiCorp's supported path for new providers, and its
explicit null/unknown handling matters here: several RevenueCat fields are genuinely optional, and
SDKv2's zero-value conflation would make "display name cleared" indistinguishable from "display
name unset". Alternative considered: SDKv2, rejected as legacy for new work.

### A hand-written typed client behind a narrow interface

`internal/revenuecat` owns the base URL, auth header, JSON encoding, pagination, retry and error
typing. Resources never construct URLs or read raw JSON; they call methods that take and return Go
structs. This is what makes constraint (1) survivable: if `lookup_key` turns out to be spelled
differently, one struct tag changes.

A single unexported `do()` method carries request execution, so auth, retry, error decoding and
logging exist once rather than per-endpoint.

### Errors carry status, not prose

`APIError` holds the HTTP status plus the API's own code and message, and `IsNotFound(err)` reports
404 without string matching. Resources need this constantly — every Read must distinguish "gone,
drop from state" from "broken, raise a diagnostic" — and string matching on error text is exactly
the kind of coupling that breaks when a message is reworded. Alternative considered: sentinel
errors per status, rejected because callers only care about 404 and "everything else".

### Retry policy lives in the client, not in resources

Retry `429`, `5xx` and transport errors with exponential backoff, honoring `Retry-After`; never
retry other `4xx`. Backoff waits select on the caller's context so a canceled apply returns
promptly instead of sleeping out the schedule. Putting this in resources would mean six copies and
six chances to get cancellation wrong.

### A shared resource skeleton

Each resource is one file with the same five sections (schema, create, read, update, delete) plus
import, and each uses a plain model struct with `tfsdk` tags mapped by the framework rather than
hand-rolled attribute reads. Required-replace behavior is declared with
`RequiresReplace()` plan modifiers on the identity attributes named in the specs, so the "which
changes replace" decision is visible in the schema instead of buried in Update.

### Attachments as separate resources, computing a diff

Attach/detach are action endpoints, not sub-resources with their own IDs, so they cannot be plain
CRUD resources. Modelling them as separate resources that own a *set* keeps them composable: a
practitioner can attach products to an entitlement they did not create.

Update computes `toAttach = config - state` and `toDetach = state - config` rather than
detaching everything and re-attaching. Beyond being fewer calls, a full detach/re-attach would
briefly revoke entitlement access for live customers — an outage caused by a no-op-shaped plan.
That differential behavior is specified, not incidental, which is why it has its own scenarios.

The synthetic resource ID is the parent ID (`<project_id>:<entitlement_id>`), since the
relationship has no identity of its own.

Alternative considered: folding `product_ids` into the entitlement and package resources. Rejected
because it forces one resource to own both the object and its relationships, which breaks the
common case of attaching to an entitlement managed elsewhere.

### Testing against an in-process fake

Constraint (2) rules out real acceptance tests, so the substitute has to be more than schema
assertions. `internal/revenuecat` is tested against `httptest` servers that assert on method, path,
headers and body and return recorded-shape responses — this is what pins pagination, retry,
`Retry-After`, cancellation and error decoding.

Resource logic that has real branching — the attachment set diff, import ID parsing, model
conversion — is written as pure functions over plain values and tested directly, so it is covered
without a Terraform binary. Acceptance tests are still written, gated behind `TF_ACC`, so they run
unchanged wherever Terraform and a real API key are available.

This is deliberately a compromise: unit tests prove the provider is internally consistent, not that
it matches the live API. Whoever first runs it against a real key is doing the verification these
tests cannot.

## Risks / Trade-offs

- **Encoded endpoint paths or field names are wrong** → Every wire-format fact is confined to
  `internal/revenuecat`, and `base_url` is overridable, so a fix is one package and a rerun of the
  fake-server tests. README and provider docs state plainly that the contract is unverified against
  the live API.
- **Unit tests pass while the real API rejects the requests** → Called out explicitly rather than
  papered over: acceptance tests exist and are gated behind `TF_ACC`, and the README documents
  running them as the verification step this environment could not perform.
- **The API may not support updating some fields the specs treat as updatable** (product identity
  is already assumed immutable) → Identity attributes carry `RequiresReplace`; if a field turns out
  to be immutable in practice, moving it to `RequiresReplace` is a one-line schema change.
- **Attachment resources can conflict if two of them target the same entitlement** → Cannot be
  enforced by the provider, since Terraform gives a resource no view of its siblings. Documented as
  a constraint in both resources' docs, per the spec requirement.
- **Metadata typed as `map[string]string`** → If RevenueCat accepts arbitrary JSON values there,
  richer metadata is not expressible. Chosen anyway because Terraform maps are homogeneous and a
  `jsonencode`-style escape hatch is worse to use than to add later if needed.

## Migration Plan

Not applicable — new repository, nothing deployed, no existing state to migrate. The first release
is `v0.1.0` and the provider is explicitly pre-1.0: resource schemas may change while the encoded
API contract is being corrected against reality.

## Open Questions

- Does RevenueCat v2 expose an update endpoint for products? The design assumes not and marks the
  identity attributes `RequiresReplace`, which is safe either way — adding in-place update later is
  backward compatible, so this can be answered against a live key without reworking anything.
- Exact enum membership for app `type` (which store identifiers are accepted). The validator holds
  the documented set; adding a missing member is a one-line change and does not affect any other
  decision here.
