## 1. Mock API server

- [x] 1.1 Implement the in-memory store and object model for projects, apps, products, entitlements, offerings and packages with generated ids and timestamps; verify with unit tests for id uniqueness and create-then-read
- [x] 1.2 Implement request routing and the CRUD handlers with parent scoping, returning 404 for absent or wrongly-parented objects; verify with tests for a wrong project id and for listing packages scoped to one offering
- [x] 1.3 Implement bearer authentication returning 401 for a missing or non-bearer credential; verify with tests that state is unchanged after a rejected request
- [x] 1.4 Implement list pagination with a configurable page size and next_page cursors; verify with tests for a single page and for a multi-page walk that repeats no item
- [x] 1.5 Implement attach and detach actions plus attached-product listing, including re-attach updating eligibility criteria; verify with tests for attach, selective detach and criteria update
- [x] 1.6 Implement counted 429 fault injection with Retry-After, off by default; verify with a test that the first request fails and the second succeeds
- [ ] 1.7 Add cmd/mock-revenuecat with address, page size, seeded project and fault flags plus an unauthenticated health endpoint; verify the binary builds and serves a seeded project

## 2. Client conformance against the mock

- [x] 2.1 Add a test that drives the real API client through a full catalog lifecycle against the mock, asserting each object is created, read back, updated, listed and deleted; verify it passes with no network access

## 3. Terraform-driven end-to-end tests

- [x] 3.1 Add the end-to-end harness: locate the terraform binary, skip with an actionable message when absent or when TF_ACC is unset, and start a mock server per test with the provider pointed at it; verify the suite skips cleanly with TF_ACC unset
- [x] 3.2 Add lifecycle tests for entitlement and offering covering apply, empty follow-up plan, in-place update and import verification; verify all steps pass against the mock
- [x] 3.3 Add lifecycle tests for app, product and package including the three-segment package import; verify all steps pass
- [x] 3.4 Add a test asserting an identity-attribute change replaces the resource and changes its id; verify it passes
- [x] 3.5 Add attachment tests covering apply, changing the product set with an empty follow-up plan, and destroying the attachment while its products and parent survive; verify by reading the mock after destroy
- [x] 3.6 Add a test applying the repository's examples/complete configuration against the mock and asserting an empty second plan and a clean destroy; verify it passes

## 4. Docker Compose end-to-end

- [ ] 4.1 Add a Dockerfile building the mock server image; verify the build instructions are consistent with the module layout
- [ ] 4.2 Add docker-compose.yaml wiring the mock and a Terraform runner container with a health-gated dependency; verify the compose file parses
- [ ] 4.3 Add the runner script that applies examples/complete, asserts an empty second plan, then destroys, failing the container on any error; verify the script is syntactically valid shell

## 5. CI and documentation

- [ ] 5.1 Add a CI job installing Terraform and running the end-to-end tests against the mock; verify the workflow parses and the job runs the gated suite
- [ ] 5.2 Add a CI job running the Docker Compose end-to-end stack and failing on a non-zero exit; verify the workflow parses
- [ ] 5.3 Update README and docs/index.md so the caveat states precisely what is verified and what is not; verify a test asserts the corrected wording is present
- [ ] 5.4 Add Makefile targets for the mock server, the end-to-end tests and the Compose stack; verify each target runs

## 6. Verification

- [ ] 6.1 Run gofmt, go vet, the full unit suite and the Terraform-driven suite, and confirm all pass and the module stays tidy
