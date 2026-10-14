---
page_title: "revenuecat_app_public_api_keys Data Source - revenuecat"
subcategory: ""
description: |-
  Reads the public SDK keys RevenueCat generated for an app.
---

# revenuecat_app_public_api_keys (Data Source)

Reads the public SDK keys RevenueCat generated for an app — the `goog_…` key an Android app
configures the RevenueCat SDK with. The keys are public by design (they ship inside the app), so
they are not marked sensitive, and they can flow straight into the configuration that needs them
instead of being copied out of the dashboard.

## Example Usage

```terraform
data "revenuecat_app_public_api_keys" "android" {
  project_id = revenuecat_app.android.project_id
  app_id     = revenuecat_app.android.id
}

output "android_sdk_key" {
  value = data.revenuecat_app_public_api_keys.android.production_key
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the app belongs to.
- `app_id` (String) Identifier of the app.

### Read-Only

- `production_key` (String) The oldest production key, which is what an app configures its SDK
  with. Null when the app has no production key.
- `keys` (List of Object) Every public key of the app, oldest first. Each has `id`, `key`,
  `environment` (`production` or `sandbox`) and `created_at`.
