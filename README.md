# Terraform Provider for RevenueCat

A Terraform provider that manages a [RevenueCat](https://www.revenuecat.com/) project's catalog —
apps, products, entitlements, offerings and packages — through the RevenueCat REST API v2.

> [!IMPORTANT]
> **The API contract in this provider has not been verified against a live RevenueCat account.**
> Endpoint paths and request and response field names were written from API documentation, not
> exercised against the real service. Every test runs against an in-process fake. Read
> [Status](#status) before pointing this at a production project.

## Why

RevenueCat's catalog is normally configured by hand in the dashboard, which leaves it untracked,
unreviewable and awkward to reproduce across environments. This provider brings that configuration
under the same review and version control as the rest of your infrastructure.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) 1.0 or later, or
  [OpenTofu](https://opentofu.org/)
- [Go](https://go.dev/dl/) 1.25 or later, to build from source
- A RevenueCat **API v2 secret key** (v1 keys will not work)

## Installation

The provider is not published to the Terraform Registry. Build and install it locally:

```shell
git clone https://github.com/nmehlei/terraform-provider-revenuecat.git
cd terraform-provider-revenuecat
make install
```

Then point Terraform at your local build with a `dev_overrides` block in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "nmehlei/revenuecat" = "/path/to/your/gopath/bin"
  }
  direct {}
}
```

With `dev_overrides` in place, skip `terraform init` and run `terraform plan` directly.

## Quick start

```terraform
terraform {
  required_providers {
    revenuecat = {
      source = "nmehlei/revenuecat"
    }
  }
}

provider "revenuecat" {
  # Reads REVENUECAT_API_KEY from the environment.
}

data "revenuecat_project" "main" {
  name = "Acme"
}

resource "revenuecat_app" "ios" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme iOS"
  type       = "app_store"
}

resource "revenuecat_product" "monthly" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.pro.monthly"
  type             = "subscription"
}

resource "revenuecat_entitlement" "pro" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "pro"
  display_name = "Pro"
}

resource "revenuecat_entitlement_product_attachment" "pro" {
  project_id     = data.revenuecat_project.main.id
  entitlement_id = revenuecat_entitlement.pro.id
  product_ids    = [revenuecat_product.monthly.id]
}
```

A full worked configuration lives in [`examples/complete`](examples/complete).

## Configuration

| Argument | Environment variable | Default | Description |
| --- | --- | --- | --- |
| `api_key` | `REVENUECAT_API_KEY` | — | RevenueCat API v2 secret key. Required. |
| `base_url` | `REVENUECAT_BASE_URL` | `https://api.revenuecat.com/v2` | API base URL. |
| `max_retries` | — | `3` | Retries for `429` and `5xx` responses. |
| `request_timeout_seconds` | — | `30` | Timeout for a single request. |

A value set in the provider block takes precedence over the environment variable. The API key is
marked sensitive, so it is redacted from plan output and logs.

## Resources and data sources

**Resources**

| Name | Manages |
| --- | --- |
| `revenuecat_app` | A store app within a project |
| `revenuecat_product` | A store product |
| `revenuecat_entitlement` | An entitlement |
| `revenuecat_offering` | An offering |
| `revenuecat_package` | A package within an offering |
| `revenuecat_entitlement_product_attachment` | Products granting an entitlement |
| `revenuecat_package_product_attachment` | Products presented by a package |

**Data sources**

`revenuecat_project`, `revenuecat_projects`, `revenuecat_app`, `revenuecat_product`,
`revenuecat_entitlement`, `revenuecat_offering`, `revenuecat_package`.

Full reference documentation is under [`docs/`](docs).

Projects themselves are not manageable: RevenueCat creates them through the dashboard, so the
provider only reads them.

### A note on the attachment resources

`revenuecat_entitlement_product_attachment` and `revenuecat_package_product_attachment` each own
the *entire* set of products attached to their target. Pointing two attachment resources at the
same entitlement or package makes them fight over that set and produces a permanent diff. Terraform
gives a resource no view of its siblings, so the provider cannot detect this for you.

When the set changes, the provider attaches only the newly added products and detaches only the
removed ones. This is deliberate: detaching everything and re-attaching would briefly revoke
entitlement access for live customers, which is not an acceptable side effect of a plan that only
adds one product.

## Import

Every managed resource supports import. Identifiers are colon-delimited:

```shell
terraform import revenuecat_entitlement.pro proj1abc:entl1xyz
terraform import revenuecat_package.monthly proj1abc:ofrng1xyz:pkg1def
```

Each resource's docs page lists its exact import format.

## Development

```shell
make build      # compile the provider
make test       # unit tests: no network, no Terraform binary needed
make lint       # gofmt check and go vet
make testacc    # acceptance tests against a real project (see below)
```

### Tests

Unit tests run against `httptest` fakes and never reach the network. They cover the client's
authentication, pagination, retry and error handling; the attachment set-difference logic; import
ID parsing; credential resolution; and every resource's not-found handling.

Acceptance tests drive a real `terraform` binary against a real RevenueCat project. They are gated
behind `TF_ACC` and skipped by default:

```shell
export TF_ACC=1
export REVENUECAT_API_KEY=sk_...
export REVENUECAT_PROJECT_ID=proj...
make testacc
```

They create and destroy real catalog objects. **Point them at a scratch project, never a production
one.**

## Status

This provider is pre-1.0 and its API contract is unverified. It was built in an environment with no
network access to RevenueCat, so the endpoint paths and payload field names come from API
documentation rather than from observing the live service. The unit tests prove the provider is
internally consistent; they cannot prove it matches the real API.

Whoever first runs it against a real API key is performing the verification the test suite could
not. If a request is rejected, the fix is likely a one-line change: every wire-format detail lives
in [`internal/revenuecat`](internal/revenuecat), deliberately isolated so a correction stays local.
Please open an issue with the failing request and response.

Resource schemas may change as the contract is corrected.

## License

MIT — see [LICENSE](LICENSE).
