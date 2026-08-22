---
page_title: "revenuecat_package Data Source - revenuecat"
subcategory: ""
description: |-
  Looks up a package in a RevenueCat offering.
---

# revenuecat_package (Data Source)

Looks up a package in a RevenueCat offering by identifier or by lookup key. Exactly one of `id` or
`lookup_key` must be set.

## Example Usage

```terraform
data "revenuecat_package" "monthly" {
  project_id  = data.revenuecat_project.main.id
  offering_id = data.revenuecat_offering.default.id
  lookup_key  = "$rc_monthly"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the package belongs to.
- `offering_id` (String) Identifier of the offering the package belongs to.

### Optional

- `id` (String) Identifier of the package. Conflicts with `lookup_key`.
- `lookup_key` (String) Lookup key of the package. Conflicts with `id`.

### Read-Only

- `display_name` (String) Human-readable name of the package.
- `position` (Number) Position of the package within its offering.
- `created_at` (Number) Creation time of the package, in milliseconds since the Unix epoch.
