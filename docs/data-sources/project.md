---
page_title: "revenuecat_project Data Source - revenuecat"
subcategory: ""
description: |-
  Looks up a RevenueCat project by identifier or by name.
---

# revenuecat_project (Data Source)

Looks up a RevenueCat project by identifier or by name. Exactly one of `id` or `name` must be set.

Looking up by `name` lists the accessible projects and matches exactly one. A name matching no
project, or more than one, is an error rather than an arbitrary pick.

## Example Usage

```terraform
data "revenuecat_project" "main" {
  name = "Acme"
}
```

## Schema

### Optional

- `id` (String) Identifier of the project. Conflicts with `name`.
- `name` (String) Name of the project. Must match exactly one project. Conflicts with `id`.

### Read-Only

- `created_at` (Number) Creation time of the project, in milliseconds since the Unix epoch.
