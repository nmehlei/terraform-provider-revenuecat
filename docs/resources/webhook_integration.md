---
page_title: "revenuecat_webhook_integration Resource - revenuecat"
subcategory: ""
description: |-
  Manages a webhook RevenueCat calls with a project's customer events.
---

# revenuecat_webhook_integration (Resource)

Manages a webhook RevenueCat calls with a project's customer events (purchases, renewals,
cancellations, expirations and so on).

## Example Usage

```terraform
ephemeral "azurerm_key_vault_secret" "webhook_auth" {
  name         = "revenuecat-webhook-auth"
  key_vault_id = data.azurerm_key_vault.main.id
}

resource "revenuecat_webhook_integration" "backend" {
  project_id = data.revenuecat_project.main.id
  name       = "Backend"
  url        = "https://api.example.com/billing/revenuecat/webhook"

  authorization_header_wo         = ephemeral.azurerm_key_vault_secret.webhook_auth.value
  authorization_header_wo_version = "1"
}
```

## Schema

### Required

- `project_id` (String) Identifier of the project the webhook belongs to. Changing this forces a new webhook.
- `name` (String) Display name of the webhook.
- `url` (String) HTTPS URL RevenueCat sends events to.

### Optional

- `environment` (String) Only send events from this environment, one of `production`, `sandbox`.
  Unset sends events from both.
- `event_types` (Set of String) Only send these event types (for example `initial_purchase`,
  `renewal`, `expiration`). Unset sends every event type.
- `app_id` (String) Only send events for this app. Unset sends events for every app in the project.
- `authorization_header_wo` (String, Write-only, Sensitive) Value RevenueCat sends in the
  `Authorization` header of every webhook request, so the receiver can reject calls that did not
  come from RevenueCat. Terraform sends it but never writes it to state or to a plan file, and
  RevenueCat never returns it. Requires Terraform 1.11 or later, and must be set together with
  `authorization_header_wo_version`.
- `authorization_header_wo_version` (String) Version marker for the write-only header. Changing it
  is what tells the provider to send the header again; bump it whenever the header is rotated.

### Read-Only

- `id` (String) RevenueCat identifier of the webhook integration.
- `created_at` (Number) Creation time of the webhook, in milliseconds since the Unix epoch.

## The authorization header

The header is a shared secret, so the attribute is write-only. Source it from an `ephemeral`
resource (as in the example) to keep it out of every artifact; a `data` source would store it in
state. Any value works as the input, including `random_password.x.result`, if the secret already
lives in state for another reason.

Because Terraform cannot see a change to a value it does not store, a new header only reaches
RevenueCat when `authorization_header_wo_version` changes. Removing the header while bumping the
version clears it on RevenueCat's side.

## Import

Webhook integrations are imported with a `<project_id>:<webhook_integration_id>` identifier:

```shell
terraform import revenuecat_webhook_integration.backend proj1abc:wh1xyz
```

The header cannot be imported. After an import, set the header and a version to send it again.
