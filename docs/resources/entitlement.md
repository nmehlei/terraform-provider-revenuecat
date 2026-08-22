---
page_title: "revenuecat_entitlement Resource - revenuecat"
subcategory: ""
description: |-
  Manages an entitlement within a RevenueCat project.
---

# revenuecat_entitlement (Resource)

Manages an entitlement within a RevenueCat project. An entitlement is the level of access a
customer receives after purchasing an attached product; your app checks entitlements rather than
individual products.

Attach products to an entitlement with
[`revenuecat_entitlement_product_attachment`](entitlement_product_attachment.md).

## Example Usage

```terraform
resource "revenuecat_entitlement" "pro" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "pro"
  display_name = "Pro"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the entitlement belongs to. Changing this forces a new entitlement.
- `lookup_key` (String) Key used to reference the entitlement from the RevenueCat SDKs, for example `pro`.

### Optional

- `display_name` (String) Human-readable name of the entitlement, shown in the RevenueCat dashboard.

### Read-Only

- `id` (String) RevenueCat identifier of the entitlement.
- `created_at` (Number) Creation time of the entitlement, in milliseconds since the Unix epoch.

## Import

Entitlements are imported with a `<project_id>:<entitlement_id>` identifier:

```shell
terraform import revenuecat_entitlement.pro proj1abc:entl1xyz
```
