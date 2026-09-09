---
page_title: "revenuecat_product Resource - revenuecat"
subcategory: ""
description: |-
  Manages a store product within a RevenueCat project.
---

# revenuecat_product (Resource)

Manages a store product within a RevenueCat project. A product mirrors an in-app purchase or
subscription that already exists in the underlying store.

~> RevenueCat does not support changing a product's identity, so every attribute other than
`display_name` forces a new product when changed.

## Example Usage

```terraform
resource "revenuecat_product" "monthly_ios" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.pro.monthly"
  type             = "subscription"
  display_name     = "Pro Monthly (iOS)"
}
```

~> **Play Store subscriptions need `<subscription ID>:<base plan ID>`, not a bare ID.** A Play
Billing subscription is one subscription with multiple base plans, and RevenueCat represents each
base plan as its own product. `store_identifier = "com.acme.pro.monthly"` 400s against a Play Store
app with `Play Store subscription product's 'store_identifier' must follow the format
'subscriptionId:basePlanId'.` — this only applies to Play Store apps; App Store and the other types
use a flat identifier as shown above.

```terraform
resource "revenuecat_product" "monthly_android" {
  project_id       = data.revenuecat_project.main.id
  app_id           = revenuecat_app.android.id
  store_identifier = "acme_pro:monthly"
  type             = "subscription"
  display_name     = "Pro Monthly (Android)"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the product belongs to. Changing this forces a new product.
- `app_id` (String) Identifier of the app the product belongs to. Changing this forces a new product.
- `store_identifier` (String) Product identifier in the underlying store, for example the App Store
  product ID. For a Play Store subscription this is not the bare subscription ID — see the warning
  above. Changing this forces a new product.
- `type` (String) Type of the product. One of `subscription`, `one_time`. Changing this forces a new product.

### Optional

- `display_name` (String) Human-readable name of the product, shown in the RevenueCat dashboard.

### Read-Only

- `id` (String) RevenueCat identifier of the product.
- `created_at` (Number) Creation time of the product, in milliseconds since the Unix epoch.

## Import

Products are imported with a `<project_id>:<product_id>` identifier:

```shell
terraform import revenuecat_product.monthly_ios proj1abc:prod1xyz
```
