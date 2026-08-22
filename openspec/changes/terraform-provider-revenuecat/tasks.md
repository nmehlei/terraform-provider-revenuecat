## 1. Repository and module scaffolding

- [x] 1.1 Initialize the Go module `github.com/nmehlei/terraform-provider-revenuecat` for Go 1.24 and add the terraform-plugin-framework dependencies; verify `go mod download` and `go build ./...` succeed
- [x] 1.2 Add `main.go` serving the provider over `providerserver` with a `-debug` flag; verify `go vet ./...` passes and the binary builds
- [ ] 1.3 Add `.gitignore`, `LICENSE` (MPL-2.0), `Makefile` with `build`/`test`/`testacc`/`fmt`/`lint` targets, and a GitHub Actions workflow running build, vet and tests; verify `make build` and `make test` run

## 2. API client (capability: api-client)

- [x] 2.1 Define the client struct, constructor and options (base URL, API key, HTTP client, max retries, timeout, user agent) and a `do()` request path that sets the Bearer, Accept, Content-Type and User-Agent headers; verify with a fake-server test asserting every header
- [x] 2.2 Implement base-URL joining that preserves the base path prefix; verify with a test using a base URL carrying a `/v2` prefix
- [x] 2.3 Implement `APIError` with status, code and message, JSON and non-JSON body decoding, and `IsNotFound`; verify with tests for a structured 400 body, a non-JSON 500 body, and a 404
- [x] 2.4 Implement retry with exponential backoff for 429/5xx/transport errors, `Retry-After` support, no retry on other 4xx, and context-aware backoff waits; verify with tests for retried 429, honored Retry-After, un-retried 422, exhausted 503, and cancellation during backoff
- [x] 2.5 Implement cursor pagination that concatenates pages until no next cursor and errors past a bounded page count; verify with tests for a two-page listing, a single-page listing, and a never-ending cursor
- [x] 2.6 Define the typed models for project, app, product, entitlement, offering and package with JSON tags; verify with a round-trip decode test per model
- [x] 2.7 Implement project read/list and app, product, entitlement, offering and package CRUD operations; verify with fake-server tests asserting the method, path and request body of each operation
- [x] 2.8 Implement entitlement and package attach/detach and attached-product listing operations; verify with fake-server tests asserting the action paths and payload shapes

## 3. Provider configuration (capability: provider-configuration)

- [x] 3.1 Implement the provider type, metadata and schema with `api_key` (sensitive), `base_url`, `max_retries` and `request_timeout_seconds`; verify with a schema validation test
- [x] 3.2 Implement `Configure` resolving config-then-environment for the API key and base URL, deferring on unknown values, validating the base URL and rejecting negative tuning values; verify with tests covering config precedence, environment fallback, missing credential, unknown value, malformed base URL and a negative retry count
- [x] 3.3 Register all resources and data sources on the provider and verify with a test asserting the expected type names are present

## 4. Catalog resources (capability: catalog-resources)

- [x] 4.1 Add shared resource helpers: import-ID parsing for the two- and three-segment forms with a format-stating error, model conversion helpers, and a not-found-to-state-removal helper; verify with unit tests over valid, short, long and empty import IDs
- [x] 4.2 Implement `revenuecat_app` with its schema, enum validation on `type`, `RequiresReplace` on `project_id` and `type`, full CRUD and import; verify with unit tests for the schema, the type validator and import parsing
- [x] 4.3 Implement `revenuecat_product` with `RequiresReplace` on all identity attributes, enum validation on `type`, CRUD and import; verify with unit tests for the schema and validator
- [x] 4.4 Implement `revenuecat_entitlement` with in-place update of `lookup_key` and `display_name`, CRUD and import; verify with unit tests for the schema and import parsing
- [x] 4.5 Implement `revenuecat_offering` including `is_current` and the `metadata` map, CRUD and import; verify with unit tests for the schema and model conversion of metadata
- [x] 4.6 Implement `revenuecat_package` with `RequiresReplace` on `project_id` and `offering_id`, CRUD and three-segment import; verify with unit tests for the schema and import parsing
- [x] 4.7 Ensure every resource's Read removes the resource from state on not-found and every Delete tolerates not-found; verify with a test per resource over a fake server returning 404

## 5. Catalog attachments (capability: catalog-attachments)

- [x] 5.1 Implement the set-difference helper computing attach and detach lists from state and configuration; verify with unit tests for add-only, remove-only, simultaneous add and remove, and reordered-but-equal sets
- [x] 5.2 Implement `revenuecat_entitlement_product_attachment` with non-empty set validation, differential update, drift-reflecting read, detach-on-destroy and replacement on parent change; verify with unit tests for the diff behavior and the empty-set validator
- [x] 5.3 Implement `revenuecat_package_product_attachment` with `product` blocks carrying `eligibility_criteria`, re-attach when criteria change, and the same lifecycle behavior; verify with unit tests for criteria-change detection and the diff behavior

## 6. Catalog data sources (capability: catalog-data-sources)

- [x] 6.1 Implement `revenuecat_project` (lookup by `id` or `name`, exactly-one-selector validation, distinct no-match and ambiguous-match errors) and `revenuecat_projects`; verify with fake-server tests for each of those four outcomes
- [x] 6.2 Implement the `revenuecat_app`, `revenuecat_product`, `revenuecat_entitlement`, `revenuecat_offering` and `revenuecat_package` data sources with ID-or-natural-key selection and mutually-exclusive-selector validation; verify with unit tests for the selector validators and a fake-server test resolving a natural key by listing
- [x] 6.3 Add a shared natural-key resolution helper producing a no-match error naming the searched key; verify with a unit test asserting the error text names the key

## 7. Documentation and examples

- [ ] 7.1 Write `docs/index.md` for the provider plus a page per resource and data source, each with arguments, attributes and an import section; verify every registered type has a corresponding docs page
- [ ] 7.2 Add runnable `examples/` covering provider setup and an end-to-end catalog (app, products, entitlement, offering, packages and both attachments); verify the example files are syntactically well-formed HCL
- [ ] 7.3 Write `README.md` covering installation, provider configuration, a quick-start example, how to run unit and acceptance tests, and an explicit statement that the API contract is encoded from documentation and unverified against the live API; verify the statement is present

## 8. Verification

- [ ] 8.1 Add `TF_ACC`-gated acceptance tests for the entitlement and offering lifecycles that skip cleanly when unset; verify they are skipped by `go test ./...` without `TF_ACC`
- [ ] 8.2 Run `gofmt -l`, `go vet ./...` and `go test ./...` across the module and confirm formatting is clean, vet is silent and all tests pass
