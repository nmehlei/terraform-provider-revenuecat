# provider-configuration Specification

## Purpose
Defines how practitioners configure and authenticate the RevenueCat Terraform provider, where
credentials come from, and how misconfiguration is surfaced as actionable diagnostics rather than
runtime failures.

## Requirements

### Requirement: Provider accepts an API key
The provider SHALL accept a RevenueCat API v2 secret key through an `api_key` argument. The
argument SHALL be marked sensitive so its value is redacted from Terraform plan output, state
display, and logs.

#### Scenario: API key supplied in configuration
- **WHEN** a practitioner sets `api_key` in the `provider "revenuecat"` block
- **THEN** the provider uses that value to authenticate every API request
- **AND** the value is not rendered in plan output or log lines

#### Scenario: API key omitted from configuration
- **WHEN** `api_key` is absent from the provider block and the `REVENUECAT_API_KEY` environment
  variable is set
- **THEN** the provider uses the environment variable value

#### Scenario: API key available from neither source
- **WHEN** `api_key` is absent from the provider block and `REVENUECAT_API_KEY` is unset or empty
- **THEN** the provider returns an error diagnostic attributed to the `api_key` attribute
- **AND** the diagnostic names the `REVENUECAT_API_KEY` environment variable as an alternative

#### Scenario: Configuration value takes precedence
- **WHEN** `api_key` is set in configuration and `REVENUECAT_API_KEY` is also set to a different value
- **THEN** the provider uses the value from configuration

### Requirement: Provider supports overriding the API base URL
The provider SHALL accept an optional `base_url` argument that overrides the default endpoint
`https://api.revenuecat.com/v2`, sourced from the `REVENUECAT_BASE_URL` environment variable when
not set in configuration. This enables testing against a fake or proxy.

#### Scenario: Base URL defaulted
- **WHEN** neither `base_url` nor `REVENUECAT_BASE_URL` is set
- **THEN** requests are sent to `https://api.revenuecat.com/v2`

#### Scenario: Base URL overridden
- **WHEN** `base_url` is set to `http://127.0.0.1:8080/v2`
- **THEN** every API request the provider makes targets that host and path prefix

#### Scenario: Base URL is not a valid URL
- **WHEN** `base_url` is set to a string that cannot be parsed as an absolute HTTP or HTTPS URL
- **THEN** the provider returns an error diagnostic attributed to the `base_url` attribute

### Requirement: Provider supports tuning request behavior
The provider SHALL accept optional `max_retries` and `request_timeout_seconds` arguments that
control how transient failures are retried and how long a single request may take. Both SHALL have
documented defaults and SHALL reject negative values.

#### Scenario: Defaults applied
- **WHEN** neither argument is set
- **THEN** the client retries retryable responses up to 3 times and uses a 30 second request timeout

#### Scenario: Negative value rejected
- **WHEN** `max_retries` is set to a negative number
- **THEN** the provider returns an error diagnostic attributed to that attribute

### Requirement: Unknown configuration values defer validation
The provider SHALL NOT report a missing-credential error while a configuration value is still
unknown at plan time, because the value may be supplied by another resource during apply.

#### Scenario: API key is unknown at plan time
- **WHEN** `api_key` is set to a value that is unknown during planning
- **THEN** the provider defers configuration and emits no missing-credential error
