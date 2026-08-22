## Purpose

Defines the Terraform resources that bind products to entitlements and to packages, modelling
RevenueCat's attach and detach action endpoints as declarative relationships that can be created,
updated and destroyed independently of the objects they connect.

## ADDED Requirements

### Requirement: Entitlement product attachment binds products to an entitlement
The `revenuecat_entitlement_product_attachment` resource SHALL manage the set of products attached
to one entitlement. It SHALL require `project_id`, `entitlement_id` and a non-empty set of
`product_ids`. Creating it SHALL attach exactly the configured products. Changing `project_id` or
`entitlement_id` SHALL force replacement.

#### Scenario: Products attached on create
- **WHEN** the resource is created with two product IDs
- **THEN** the provider calls the entitlement's attach action with exactly those two product IDs

#### Scenario: Empty product set rejected
- **WHEN** `product_ids` is configured as an empty set
- **THEN** validation fails during planning without any API call

### Requirement: Attachment updates send only the difference
When the configured product set changes, the resource SHALL attach only products that are newly
configured and detach only products that were removed, leaving unchanged products untouched.

#### Scenario: Product added and removed together
- **WHEN** state holds products A and B, and the configuration changes to B and C
- **THEN** the provider attaches only C and detaches only A
- **AND** the provider makes no attach or detach call naming B

#### Scenario: No effective change
- **WHEN** the configured set is reordered but contains the same products as state
- **THEN** the provider makes no attach or detach call

### Requirement: Attachment reflects the remote relationship on read
On refresh, the resource SHALL read the products currently attached to the entitlement and record
that set in state, so products attached or detached outside Terraform appear as drift.

#### Scenario: Product detached outside Terraform
- **WHEN** a product recorded in state is no longer attached remotely
- **THEN** refresh records the reduced set and the next plan proposes re-attaching it

#### Scenario: Entitlement deleted outside Terraform
- **WHEN** refresh finds the entitlement itself no longer exists
- **THEN** the resource is removed from state without an error diagnostic

### Requirement: Attachment destroy detaches only managed products
Destroying the resource SHALL detach the products recorded in its state from the entitlement and
SHALL NOT delete the products or the entitlement themselves.

#### Scenario: Attachment destroyed
- **WHEN** the resource is destroyed
- **THEN** the provider calls the detach action with the product IDs in state
- **AND** the provider issues no delete call for any product or for the entitlement

### Requirement: Package product attachment binds products to a package
The `revenuecat_package_product_attachment` resource SHALL manage the products attached to one
package. It SHALL require `project_id` and `package_id`, and a non-empty set of `product` blocks
each carrying a `product_id` and an optional `eligibility_criteria`. It SHALL follow the same
differential update, drift-reflecting read, and detach-on-destroy behavior as the entitlement
attachment. Changing `project_id` or `package_id` SHALL force replacement.

#### Scenario: Products attached to a package on create
- **WHEN** the resource is created with one product entry carrying an eligibility criteria
- **THEN** the provider calls the package's attach action with that product ID and criteria

#### Scenario: Eligibility criteria changed
- **WHEN** only the `eligibility_criteria` of an already-attached product changes
- **THEN** the provider re-attaches that product with the new criteria

#### Scenario: Package attachment destroyed
- **WHEN** the resource is destroyed
- **THEN** the provider calls the package detach action with the product IDs in state
- **AND** issues no delete call for any product or for the package

### Requirement: Only one attachment resource may manage a given relationship
The documentation for both attachment resources SHALL state that managing the same entitlement or
package from more than one attachment resource produces conflicting plans, so each relationship
must be owned by exactly one resource.

#### Scenario: Documented ownership constraint
- **WHEN** a practitioner reads the documentation for either attachment resource
- **THEN** it warns that the target entitlement or package must be managed by only one such resource
