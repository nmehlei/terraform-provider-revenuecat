# end-to-end-verification Specification

## Purpose
Defines what a Terraform-driven run against the mock API must prove about the provider, so that
the failure modes only visible with a real `terraform` binary in the loop are caught automatically.

## Requirements

### Requirement: Every resource survives a full Terraform lifecycle
For each managed resource, a real `terraform` binary SHALL apply a configuration, refresh it,
update it, import it and destroy it against the mock API, and every step SHALL succeed.

#### Scenario: Apply creates the object
- **WHEN** Terraform applies a configuration declaring the resource
- **THEN** the apply succeeds and the resource's computed attributes are populated in state

#### Scenario: Destroy removes the object
- **WHEN** Terraform destroys the configuration
- **THEN** the destroy succeeds and reading the object from the mock afterwards reports it absent

### Requirement: Applying a configuration twice produces no second change
After a successful apply, re-planning the same configuration SHALL produce an empty plan. This is
the check that catches a Read which normalizes a value differently from Create, which would
otherwise present practitioners with a diff that never converges.

#### Scenario: Plan is empty after apply
- **WHEN** Terraform applies a configuration and then plans it again without changes
- **THEN** the plan reports no changes

#### Scenario: Refresh introduces no drift
- **WHEN** Terraform refreshes state after an apply and plans again
- **THEN** the plan reports no changes

### Requirement: Updates apply in place without replacement
Changing an attribute the specification says is updatable SHALL result in an update rather than a
destroy and recreate, and the new value SHALL be present in state afterwards.

#### Scenario: Display name updated in place
- **WHEN** an entitlement's display name is changed and applied
- **THEN** the apply succeeds, the resource keeps its original identifier, and state carries the new name

#### Scenario: Identity change replaces the resource
- **WHEN** an attribute marked as forcing replacement is changed and applied
- **THEN** the resource is replaced and its identifier changes

### Requirement: Imported state matches applied state
Importing a resource SHALL produce state matching what an apply produced, verified attribute by
attribute. This is the check that catches an import that populates the wrong or too few attributes.

#### Scenario: Import verification
- **WHEN** a resource created by apply is imported using its documented import identifier
- **THEN** the imported state matches the applied state for every attribute

### Requirement: Attachment relationships converge under Terraform
An attachment resource SHALL reach a stable state under Terraform, and changing its product set
SHALL apply cleanly and leave no pending change.

#### Scenario: Attachment set changed
- **WHEN** a product is added to and another removed from an attachment's set, and the change is applied
- **THEN** the apply succeeds and a subsequent plan reports no changes

#### Scenario: Attachment destroy leaves its objects intact
- **WHEN** an attachment is destroyed while the products and its parent remain in the configuration
- **THEN** the destroy succeeds and the products and parent still exist in the mock

### Requirement: The shipped example applies end to end
The repository's complete example SHALL apply against the mock without modification beyond
provider endpoint configuration, and SHALL produce an empty plan afterwards, so the documentation
cannot drift into being unusable.

#### Scenario: Example applies and settles
- **WHEN** the complete example is applied against the mock
- **THEN** the apply succeeds and a second plan reports no changes

#### Scenario: Example destroys cleanly
- **WHEN** the applied example is destroyed
- **THEN** the destroy succeeds with no remaining resources in state

### Requirement: Terraform-driven checks are gated and self-describing
Tests requiring a `terraform` binary SHALL run only when explicitly enabled, and SHALL skip with a
message naming what is missing rather than failing, so the default `go test ./...` remains runnable
with no Terraform installed.

#### Scenario: Terraform absent
- **WHEN** the Terraform-driven tests are enabled but no `terraform` binary can be found
- **THEN** they skip with a message naming the binary and how to supply it

#### Scenario: Not enabled
- **WHEN** the enabling variable is unset
- **THEN** the Terraform-driven tests skip and the remaining unit tests run normally

### Requirement: Verification runs in continuous integration
Continuous integration SHALL run the Terraform-driven verification against the mock on every push
and pull request, and SHALL fail the build when any of it fails.

#### Scenario: CI runs the end-to-end job
- **WHEN** a pull request is opened
- **THEN** continuous integration installs a Terraform binary and runs the Terraform-driven tests
  against the mock
