# catalog-resources Specification

## Purpose
Defines the managed Terraform resources for the RevenueCat project catalog — apps, products,
entitlements, offerings and packages — including their create/read/update/delete lifecycle, which
attribute changes force replacement, and how existing objects are imported into state.

## Requirements

### Requirement: Resources reconcile drift and disappearance
Every managed resource SHALL refresh its state from the API on read. When the API reports that the
underlying object no longer exists, the resource SHALL remove itself from state so Terraform plans
its recreation, rather than returning an error.

#### Scenario: Remote object deleted outside Terraform
- **WHEN** a resource is refreshed and the API returns not-found for its ID
- **THEN** the resource is removed from state and no error diagnostic is returned

#### Scenario: Remote attribute changed outside Terraform
- **WHEN** a resource is refreshed and the API returns a display name different from state
- **THEN** the refreshed value is written to state so the next plan shows the drift

#### Scenario: Delete is idempotent
- **WHEN** a resource is destroyed and the API returns not-found for the delete call
- **THEN** the destroy succeeds without an error diagnostic

### Requirement: App resource manages a store app
The `revenuecat_app` resource SHALL manage an app within a project. It SHALL require `project_id`,
`name`, `type` and `package_name`, where `type` is one of the store types this provider implements
(`play_store` only today — the API nests type-specific configuration under a key named after the
type, and every other store type RevenueCat itself supports needs its own such config object added
before this resource can create it). It SHALL expose the computed attributes `id` and `created_at`.
Changing `project_id`, `type` or `package_name` SHALL force replacement; changing `name` SHALL
update in place.

#### Scenario: App created
- **WHEN** an app is created with a project ID, name, type and package name
- **THEN** the provider issues a create call to the project's apps endpoint, with the package name
  nested under the `play_store` key the API requires alongside `type`
- **AND** state records the returned app ID and creation timestamp

#### Scenario: App renamed
- **WHEN** only `name` changes
- **THEN** the provider issues an update call and does not replace the app

#### Scenario: App type changed
- **WHEN** `type` changes
- **THEN** the plan replaces the app rather than updating it

#### Scenario: Invalid app type rejected at plan time
- **WHEN** `type` is set to a value outside the supported store types
- **THEN** validation fails during planning without any API call

### Requirement: Product resource manages a store product
The `revenuecat_product` resource SHALL manage a product within a project. It SHALL require
`project_id`, `app_id`, `store_identifier` and `type`, where `type` is `subscription` or
`one_time`. It SHALL expose computed `id` and `created_at`, and SHALL accept an optional
`display_name`. Because the API does not support changing a product's identity, changes to
`project_id`, `app_id`, `store_identifier` or `type` SHALL force replacement.

#### Scenario: Product created
- **WHEN** a product is created with the required attributes
- **THEN** the provider issues a create call to the project's products endpoint and records the ID

#### Scenario: Store identifier changed
- **WHEN** `store_identifier` changes
- **THEN** the plan replaces the product

#### Scenario: Invalid product type rejected at plan time
- **WHEN** `type` is set to a value other than `subscription` or `one_time`
- **THEN** validation fails during planning without any API call

### Requirement: Entitlement resource manages an entitlement
The `revenuecat_entitlement` resource SHALL manage an entitlement within a project. It SHALL
require `project_id`, `lookup_key` and `display_name` (the API requires the latter on create,
unlike most other resources' `display_name`), and expose computed `id` and `created_at`. Changing
`project_id` or `lookup_key` SHALL force replacement — the API has no update endpoint field for
`lookup_key`, so it is part of the entitlement's identity, not an in-place-updatable attribute;
changing `display_name` SHALL update in place.

#### Scenario: Entitlement created
- **WHEN** an entitlement is created with a lookup key and display name
- **THEN** the provider issues a create call and records the returned entitlement ID

#### Scenario: Lookup key changed
- **WHEN** `lookup_key` changes
- **THEN** the plan replaces the entitlement rather than attempting an update the API does not support

### Requirement: Offering resource manages an offering
The `revenuecat_offering` resource SHALL manage an offering within a project. It SHALL require
`project_id` and `lookup_key`, accept an optional `display_name`, an optional boolean `is_current`,
and an optional string-to-string `metadata` map, and expose computed `id` and `created_at`.
Changing `project_id` SHALL force replacement; other attribute changes SHALL update in place.

#### Scenario: Offering created as current
- **WHEN** an offering is created with `is_current` set to true
- **THEN** the provider creates the offering, then issues a follow-up update call setting the
  flag — the API rejects `is_current` on the create request itself — and state reflects it

#### Scenario: Offering metadata updated
- **WHEN** `metadata` changes
- **THEN** the provider issues an update call carrying the new metadata

### Requirement: Package resource manages a package within an offering
The `revenuecat_package` resource SHALL manage a package belonging to an offering. It SHALL require
`project_id`, `offering_id`, `lookup_key`, `display_name` and `position` — the API requires
`display_name` and `position` on every update even though `position` is optional on create, so
both are required here to avoid a package that can be created but never renamed. Changing
`project_id`, `offering_id` or `lookup_key` SHALL force replacement — the API has no update
endpoint field for `lookup_key`; changing `display_name` or `position` SHALL update in place.

#### Scenario: Package created under an offering
- **WHEN** a package is created with a project ID, offering ID, lookup key, display name and position
- **THEN** the provider issues a create call to that offering's packages endpoint

#### Scenario: Package moved to a different offering
- **WHEN** `offering_id` changes
- **THEN** the plan replaces the package

#### Scenario: Lookup key changed
- **WHEN** `lookup_key` changes
- **THEN** the plan replaces the package rather than attempting an update the API does not support

### Requirement: Resources support import
Every managed resource SHALL support `terraform import` using a colon-delimited identifier that
carries every value needed to address the object: `<project_id>:<id>` for apps, products,
entitlements and offerings, and `<project_id>:<offering_id>:<package_id>` for packages. A malformed
import identifier SHALL produce an error diagnostic that states the expected format.

#### Scenario: Entitlement imported
- **WHEN** a practitioner imports `proj1:entl123` into a `revenuecat_entitlement` resource
- **THEN** state is populated from the API with project ID `proj1` and entitlement ID `entl123`

#### Scenario: Package imported
- **WHEN** a practitioner imports `proj1:ofrng123:pkg456` into a `revenuecat_package` resource
- **THEN** state is populated with all three identifiers

#### Scenario: Malformed import ID
- **WHEN** a practitioner imports an identifier with the wrong number of segments
- **THEN** an error diagnostic states the expected format
