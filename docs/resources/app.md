---
page_title: "revenuecat_app Resource - revenuecat"
subcategory: ""
description: |-
  Manages a store app within a RevenueCat project.
---

# revenuecat_app (Resource)

Manages a store app within a RevenueCat project. An app connects a RevenueCat project to one
underlying store, such as the App Store or the Play Store.

## Example Usage

```terraform
resource "revenuecat_app" "ios" {
  project_id = data.revenuecat_project.main.id
  name       = "Acme iOS"
  type       = "app_store"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the app belongs to. Changing this forces a new app.
- `name` (String) Display name of the app.
- `type` (String) Store the app belongs to. One of `app_store`, `mac_app_store`, `play_store`,
  `amazon`, `stripe`, `rc_billing`, `roku`, `paddle`. Changing this forces a new app.

### Read-Only

- `id` (String) RevenueCat identifier of the app.
- `created_at` (Number) Creation time of the app, in milliseconds since the Unix epoch.

## Import

Apps are imported with a `<project_id>:<app_id>` identifier:

```shell
terraform import revenuecat_app.ios proj1abc:app1xyz
```
