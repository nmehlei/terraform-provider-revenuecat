---
page_title: "RevenueCat Provider"
subcategory: ""
description: |-
  Manage the RevenueCat project catalog with Terraform.
---

# RevenueCat Provider

The RevenueCat provider manages a RevenueCat project's catalog — apps, products, entitlements,
offerings and packages — through the [RevenueCat REST API v2](https://www.revenuecat.com/docs/api-v2).

~> **The API contract in this provider has not been verified against a live RevenueCat account.**
See [Status](#status) below before using it against a production project.

## Example Usage

```terraform
terraform {
  required_providers {
    revenuecat = {
      source = "nmehlei/revenuecat"
    }
  }
}

provider "revenuecat" {
  # Prefer the REVENUECAT_API_KEY environment variable over a literal here.
  api_key = var.revenuecat_api_key
}

data "revenuecat_project" "main" {
  name = "Acme"
}

resource "revenuecat_entitlement" "pro" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "pro"
  display_name = "Pro"
}
```

## Authentication

The provider authenticates with a RevenueCat **API v2 secret key**, which is distinct from a v1
key. Create one in the RevenueCat dashboard under your project's API keys, granting it write
permission for the object types you intend to manage.

Supply it either in the provider block:

```terraform
provider "revenuecat" {
  api_key = var.revenuecat_api_key
}
```

or, preferably, through the environment:

```shell
export REVENUECAT_API_KEY="sk_..."
```

A value in the provider block takes precedence over the environment variable. Because the key is a
credential, avoid committing it: pass it through a variable, a secret store, or the environment.

## Schema

### Optional

- `api_key` (String, Sensitive) A RevenueCat API v2 secret key. May also be set with the
  `REVENUECAT_API_KEY` environment variable.
- `base_url` (String) Base URL of the RevenueCat API. May also be set with the
  `REVENUECAT_BASE_URL` environment variable. Defaults to `https://api.revenuecat.com/v2`.
- `max_retries` (Number) How many times a rate-limited (`429`) or server-error (`5xx`) response is
  retried before failing. Defaults to `3`.
- `request_timeout_seconds` (Number) Timeout in seconds for a single API request. Defaults to `30`.

## Retries and rate limiting

Requests that fail with `429` or a `5xx` status, or with a transport error, are retried with
exponential backoff up to `max_retries`. A `Retry-After` header is honored when present. Other
`4xx` responses are returned immediately, since retrying a rejected request does not help.

## Status

This provider is pre-1.0. Its encoding of the RevenueCat API v2 contract — endpoint paths and
request and response field names — was written from API documentation and has **not** been
exercised against a live RevenueCat account. Resource schemas may change as the contract is
corrected against reality.

Every wire-format detail lives in a single package (`internal/revenuecat`), so a correction is
localized. If you hit a request the API rejects, please open an issue with the failing request and
response.
