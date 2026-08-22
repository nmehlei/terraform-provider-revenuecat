---
page_title: "revenuecat_entitlement Data Source - revenuecat"
subcategory: ""
description: |-
  Looks up an entitlement in a RevenueCat project.
---

# revenuecat_entitlement (Data Source)

Looks up an entitlement in a RevenueCat project by identifier or by lookup key. Exactly one of `id`
or `lookup_key` must be set.

## Example Usage

```terraform
data "revenuecat_entitlement" "pro" {
  project_id = data.revenuecat_project.main.id
  lookup_key = "pro"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the entitlement belongs to.

### Optional

- `id` (String) Identifier of the entitlement. Conflicts with `lookup_key`.
- `lookup_key` (String) Lookup key of the entitlement. Conflicts with `id`.

### Read-Only

- `display_name` (String) Human-readable name of the entitlement.
- `created_at` (Number) Creation time of the entitlement, in milliseconds since the Unix epoch.
