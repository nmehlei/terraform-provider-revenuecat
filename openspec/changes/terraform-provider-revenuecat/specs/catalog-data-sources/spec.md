## Purpose

Defines the read-only Terraform data sources that look up existing RevenueCat catalog objects, so
configurations can reference projects and catalog entries they do not manage.

## ADDED Requirements

### Requirement: Project data sources expose accessible projects
The provider SHALL offer a `revenuecat_project` data source that looks up a single project by `id`
or by `name`, and a `revenuecat_projects` data source that lists every project the API key can
access. Both SHALL expose each project's `id`, `name` and `created_at`.

#### Scenario: Project looked up by ID
- **WHEN** `revenuecat_project` is given an `id`
- **THEN** it returns that project's name and creation timestamp

#### Scenario: Project looked up by name
- **WHEN** `revenuecat_project` is given a `name` that matches exactly one project
- **THEN** it returns that project's ID

#### Scenario: Name matches no project
- **WHEN** `revenuecat_project` is given a `name` that matches no project
- **THEN** an error diagnostic reports that no project matched

#### Scenario: Name matches several projects
- **WHEN** `revenuecat_project` is given a `name` that matches more than one project
- **THEN** an error diagnostic reports the ambiguity rather than picking one

#### Scenario: Neither selector supplied
- **WHEN** `revenuecat_project` is given neither `id` nor `name`
- **THEN** validation fails during planning stating that exactly one selector is required

### Requirement: Catalog data sources look up objects by ID or lookup key
The provider SHALL offer `revenuecat_app`, `revenuecat_product`, `revenuecat_entitlement`,
`revenuecat_offering` and `revenuecat_package` data sources. Each SHALL require `project_id` and
SHALL accept either the object's `id` or its natural key — `lookup_key` for entitlements,
offerings and packages, `store_identifier` for products, `name` for apps — resolving the natural
key by listing and matching. Each SHALL expose the same attributes as its corresponding managed
resource.

#### Scenario: Entitlement looked up by lookup key
- **WHEN** `revenuecat_entitlement` is given a project ID and a lookup key that exists
- **THEN** it returns the entitlement's ID, display name and creation timestamp

#### Scenario: Package looked up within an offering
- **WHEN** `revenuecat_package` is given a project ID, an offering ID and a lookup key
- **THEN** it returns the matching package in that offering

#### Scenario: Lookup key matches nothing
- **WHEN** a catalog data source is given a natural key that matches no object
- **THEN** an error diagnostic names the key that was searched for

#### Scenario: Both selectors supplied
- **WHEN** a catalog data source is given both an `id` and a natural key
- **THEN** validation fails during planning stating that the selectors are mutually exclusive

### Requirement: Data sources never mutate remote state
Data sources SHALL only issue read operations against the API.

#### Scenario: Read-only access
- **WHEN** any data source is evaluated
- **THEN** the provider issues only GET requests for that evaluation
