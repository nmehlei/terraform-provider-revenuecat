---
page_title: "revenuecat_app Resource - revenuecat"
subcategory: ""
description: |-
  Manages a store app within a RevenueCat project.
---

# revenuecat_app (Resource)

Manages a store app within a RevenueCat project. An app connects a RevenueCat project to one
underlying store, such as the App Store or the Play Store.

The API nests store-specific configuration under a key named after the type — for example
`play_store: { package_name: ... }` — rather than accepting `type` alone. This provider currently
implements that nesting only for `play_store`; the other store types the API itself recognizes
(`app_store`, `mac_app_store`, `amazon`, `stripe`, `rc_billing`, `roku`, `paddle`) each need their
own config object added before this resource can create them.

## Example Usage

```terraform
resource "revenuecat_app" "android" {
  project_id   = data.revenuecat_project.main.id
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the app belongs to. Changing this forces a new app.
- `name` (String) Display name of the app.
- `type` (String) Store the app belongs to. One of `play_store` — RevenueCat's API nests
  type-specific configuration under a key named after the type, and this provider only implements
  that nesting for `play_store` so far. Changing this forces a new app.
- `package_name` (String) Play Store package identifier (e.g. `com.example.app`). Required because
  `type` currently only accepts `play_store`. Changing this forces a new app.

### Read-Only

- `id` (String) RevenueCat identifier of the app.
- `created_at` (Number) Creation time of the app, in milliseconds since the Unix epoch.

## Import

Apps are imported with a `<project_id>:<app_id>` identifier:

```shell
terraform import revenuecat_app.android proj1abc:app1xyz
```
