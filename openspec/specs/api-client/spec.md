# api-client Specification

## Purpose
Defines the HTTP contract between the provider and RevenueCat REST API v2 — how requests are
authenticated and shaped, how paginated collections are consumed, how API failures become typed
errors, and when a failed request is retried.

## Requirements

### Requirement: Requests are authenticated with a Bearer token
Every request the client issues SHALL carry an `Authorization: Bearer <api_key>` header, an
`Accept: application/json` header, and a `User-Agent` identifying the provider and its version.
Requests carrying a body SHALL set `Content-Type: application/json`.

#### Scenario: Authenticated request
- **WHEN** the client issues any API request
- **THEN** the request carries `Authorization: Bearer <api_key>`
- **AND** the request carries an `Accept` header of `application/json`
- **AND** the request carries a `User-Agent` header containing `terraform-provider-revenuecat`

#### Scenario: Request with a body
- **WHEN** the client issues a request that sends a JSON body
- **THEN** the request carries `Content-Type: application/json`

### Requirement: Client resolves request paths against the configured base URL
The client SHALL join relative endpoint paths onto the configured base URL without dropping any
path prefix the base URL carries, so a base URL of `http://host/api/v2` produces
`http://host/api/v2/projects/<id>/apps`.

#### Scenario: Base URL with a path prefix
- **WHEN** the base URL is `http://127.0.0.1:9999/v2` and the client fetches the entitlement list
  for project `proj1`
- **THEN** the request URL is `http://127.0.0.1:9999/v2/projects/proj1/entitlements`

### Requirement: Client follows cursor pagination
List endpoints return an object with an `items` array and an optional `next_page` cursor. When
listing, the client SHALL request subsequent pages until no next cursor is returned and SHALL
return the concatenation of all pages.

#### Scenario: Multi-page listing
- **WHEN** a list endpoint returns 2 items and a next-page cursor, and the follow-up request
  returns 1 item and no cursor
- **THEN** the client returns all 3 items in order

#### Scenario: Single-page listing
- **WHEN** a list endpoint returns items and no next-page cursor
- **THEN** the client issues exactly one request and returns those items

#### Scenario: Pagination cannot loop forever
- **WHEN** a list endpoint keeps returning a next-page cursor beyond a bounded page count
- **THEN** the client stops and returns an error rather than paging indefinitely

### Requirement: API errors are translated into typed errors
When the API responds with a non-2xx status, the client SHALL return an error carrying the HTTP
status code, and the API-provided error code and message when the response body contains them.
The error message SHALL include the request method and path to make the failure diagnosable.

#### Scenario: Structured error body
- **WHEN** the API responds `400` with a body containing an error code and message
- **THEN** the returned error exposes status `400`, that code, and that message

#### Scenario: Unstructured error body
- **WHEN** the API responds `500` with a body that is not valid JSON
- **THEN** the returned error still exposes status `500` and includes a truncated excerpt of the body

#### Scenario: Not-found is distinguishable
- **WHEN** the API responds `404`
- **THEN** the returned error is recognizable as a not-found error by callers without string matching

### Requirement: Transient failures are retried with backoff
The client SHALL retry requests that fail with HTTP `429`, `423`, or any `5xx` status, and requests
that fail with a transport error, up to the configured retry limit, waiting with exponential backoff
between attempts. It SHALL NOT retry other `4xx` responses.

#### Scenario: Rate limit is retried
- **WHEN** the API responds `429` once and then `200`
- **THEN** the client returns the successful result without surfacing an error

#### Scenario: Resource-locked is retried
- **WHEN** the API responds `423` (the API's own concurrency control, returned when a package
  mutation races another mutation on a package in the same offering) once and then succeeds
- **THEN** the client returns the successful result without surfacing an error

#### Scenario: Retry-After is honored
- **WHEN** the API responds `429` with a `Retry-After` header specifying a delay
- **THEN** the client waits at least that long before the next attempt

#### Scenario: Client errors are not retried
- **WHEN** the API responds `422`
- **THEN** the client makes exactly one attempt and returns the error

#### Scenario: Retries are exhausted
- **WHEN** the API responds `503` on every attempt
- **THEN** the client returns an error after the configured number of retries and does not hang

#### Scenario: Cancellation is respected
- **WHEN** the caller's context is canceled while a retry backoff is pending
- **THEN** the client stops retrying and returns promptly

### Requirement: Client covers the v2 catalog surface
The client SHALL provide operations for creating, reading, updating, listing and deleting apps,
products, entitlements, offerings and packages within a project; for reading projects; and for
attaching and detaching products to entitlements and to packages.

#### Scenario: Catalog operation coverage
- **WHEN** the provider needs to manage any supported catalog object
- **THEN** a corresponding typed client operation exists that accepts a context and returns a typed
  result or a typed error
