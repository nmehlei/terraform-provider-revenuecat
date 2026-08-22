---
page_title: "revenuecat_offering Data Source - revenuecat"
subcategory: ""
description: |-
  Looks up an offering in a RevenueCat project.
---

# revenuecat_offering (Data Source)

Looks up an offering in a RevenueCat project by identifier or by lookup key. Exactly one of `id` or
`lookup_key` must be set.

## Example Usage

```terraform
data "revenuecat_offering" "default" {
  project_id = data.revenuecat_project.main.id
  lookup_key = "default"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the offering belongs to.

### Optional

- `id` (String) Identifier of the offering. Conflicts with `lookup_key`.
- `lookup_key` (String) Lookup key of the offering. Conflicts with `id`.

### Read-Only

- `display_name` (String) Human-readable name of the offering.
- `is_current` (Boolean) Whether this offering is the project's current offering.
- `metadata` (Map of String) Arbitrary string key-value pairs attached to the offering.
- `created_at` (Number) Creation time of the offering, in milliseconds since the Unix epoch.
