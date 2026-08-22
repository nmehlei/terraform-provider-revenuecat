## Why

RevenueCat's in-app subscription catalog — apps, products, entitlements, offerings and packages —
is normally configured by hand in the dashboard, which makes it untracked, unreviewable and hard to
reproduce across projects or environments. RevenueCat's REST API v2 exposes that catalog for
programmatic management, but no Terraform provider exists to drive it, so teams that manage the rest
of their infrastructure as code must click through a UI for their monetization configuration.

## What Changes

- Introduce a new Terraform provider, `revenuecat`, built on `terraform-plugin-framework`, that
  manages the RevenueCat project catalog through the REST API v2.
- Add a typed Go API client (`internal/revenuecat`) covering the v2 catalog endpoints, Bearer
  authentication, cursor pagination, structured error decoding, and retry with backoff on `429`
  and `5xx`.
- Add managed resources: `revenuecat_app`, `revenuecat_product`, `revenuecat_entitlement`,
  `revenuecat_offering`, `revenuecat_package`, plus the two attachment resources
  `revenuecat_entitlement_product_attachment` and `revenuecat_package_product_attachment`
  that model the API's attach/detach actions.
- Add data sources: `revenuecat_project`, `revenuecat_projects`, `revenuecat_app`,
  `revenuecat_product`, `revenuecat_entitlement`, `revenuecat_offering`, `revenuecat_package`.
- Add import support for every managed resource so existing dashboard-created catalogs can be
  adopted into Terraform state.
- Add repository scaffolding: Go module, `Makefile`, GitHub Actions CI, provider documentation
  under `docs/`, runnable examples under `examples/`, and a unit test suite that runs against an
  in-process fake of the RevenueCat API.

Not in scope for this change: customer, subscription, purchase, invoice and refund endpoints
(runtime data, not declarative configuration); the deprecated v1 API; and publishing to the
Terraform Registry.

## Capabilities

### New Capabilities
- `provider-configuration`: how the provider is configured and authenticated, how credentials are
  sourced from configuration or environment, and how configuration errors are reported.
- `api-client`: the HTTP contract with RevenueCat API v2 — request shape, authentication header,
  pagination, error translation, and retry behavior.
- `catalog-resources`: the managed resources for apps, products, entitlements, offerings and
  packages, including their CRUD lifecycle, required-replace behavior, and import.
- `catalog-attachments`: the attachment resources that bind products to entitlements and to
  packages via the API's action endpoints.
- `catalog-data-sources`: the read-only data sources for looking up existing catalog objects.

### Modified Capabilities

None — this is a greenfield repository.

## Impact

- New Go module `github.com/nmehlei/terraform-provider-revenuecat` targeting Go 1.24.
- New dependencies: `terraform-plugin-framework`, `terraform-plugin-go`, `terraform-plugin-log`,
  and `terraform-plugin-testing` (test-only).
- External system: RevenueCat REST API v2. This session's network egress blocks
  `revenuecat.com` and `api.revenuecat.com`, so the endpoint and payload contract is encoded from
  documented knowledge rather than verified live. All of that contract is confined to
  `internal/revenuecat` so a correction touches one package, and every test exercises an
  `httptest` fake — no test reaches the real API.
- No CI secrets are required: acceptance tests are gated behind `TF_ACC` and are skipped by default.
