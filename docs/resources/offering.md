---
page_title: "revenuecat_offering Resource - revenuecat"
subcategory: ""
description: |-
  Manages an offering within a RevenueCat project.
---

# revenuecat_offering (Resource)

Manages an offering within a RevenueCat project. An offering groups the packages presented to
customers on a paywall.

## Example Usage

```terraform
resource "revenuecat_offering" "default" {
  project_id   = data.revenuecat_project.main.id
  lookup_key   = "default"
  display_name = "Default Offering"
  is_current   = true

  metadata = {
    paywall_variant = "a"
  }
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the offering belongs to. Changing this forces a new offering.
- `lookup_key` (String) Key used to reference the offering from the RevenueCat SDKs, for example `default`.

### Optional

- `display_name` (String) Human-readable name of the offering, shown in the RevenueCat dashboard.
- `is_current` (Boolean) Whether this offering is the project's current offering. Only one offering
  per project can be current; making one current clears the flag on the previous one. Defaults to `false`.
- `metadata` (Map of String) Arbitrary string key-value pairs delivered alongside the offering to the SDKs.

### Read-Only

- `id` (String) RevenueCat identifier of the offering.
- `created_at` (Number) Creation time of the offering, in milliseconds since the Unix epoch.

## Import

Offerings are imported with a `<project_id>:<offering_id>` identifier:

```shell
terraform import revenuecat_offering.default proj1abc:ofrng1xyz
```
