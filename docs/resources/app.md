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

### Optional

- `play_service_account_credentials_json_wo` (String, Write-only, Sensitive) Contents of the Google
  Cloud service account key file RevenueCat uses to verify Play purchases server-side. Terraform
  sends it but never writes it to state or to a plan file. Requires Terraform 1.11 or later, and
  must be set together with `play_service_account_credentials_json_wo_version`.
- `play_service_account_credentials_json_wo_version` (String) Version marker for the write-only
  credential. Changing it is what tells the provider to send the credential again; bump it whenever
  the key is rotated.

### Read-Only

- `id` (String) RevenueCat identifier of the app.
- `created_at` (Number) Creation time of the app, in milliseconds since the Unix epoch.
- `play_service_account_credentials_configured` (Boolean) Whether RevenueCat holds Play service
  account credentials for this app.

## Play service account credentials

RevenueCat cannot verify a Play purchase without a service account credential. An app missing one
still applies cleanly, still shows products, and still reaches the Play checkout — but no purchase
is ever acknowledged, and Google auto-refunds every unacknowledged purchase after its
acknowledgement window. `play_service_account_credentials_configured` is the only signal that
distinguishes the two states, so it is worth asserting on.

The credential is a long-lived private key, so the attribute is write-only: Terraform never
persists it. To keep it out of every artifact, source it from an `ephemeral` resource rather than a
data source — a `data` block would store the value in state, defeating the point.

```terraform
ephemeral "azurerm_key_vault_secret" "play_service_account" {
  name         = "play-service-account-json"
  key_vault_id = data.azurerm_key_vault.main.id
}

resource "revenuecat_app" "android" {
  project_id   = data.revenuecat_project.main.id
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"

  play_service_account_credentials_json_wo         = ephemeral.azurerm_key_vault_secret.play_service_account.value
  play_service_account_credentials_json_wo_version = "1"
}
```

Two things behave in ways worth knowing:

- **Rotation needs the version bumped.** Terraform cannot detect a change to a value it does not
  store, so replacing the key alone produces no plan and the new key never reaches RevenueCat.
  Change `play_service_account_credentials_json_wo_version` in the same commit as the key.
- **Setting the credential without a version is rejected.** That pairing would silently accept
  rotations that never happen, so the provider refuses it rather than appearing to work.

Google's propagation for a newly granted service account can take up to 36 hours. Until it
completes, RevenueCat reports the credential as configured while verification still fails — so a
successful apply is not by itself proof that purchases are being verified.

## Import

Apps are imported with a `<project_id>:<app_id>` identifier:

```shell
terraform import revenuecat_app.android proj1abc:app1xyz
```
