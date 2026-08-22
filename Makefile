BINARY  := terraform-provider-revenuecat
VERSION ?= dev

.PHONY: default build install test testacc testacc-mock mock compose-e2e fmt fmtcheck vet lint tidy clean

default: build

## build: compile the provider binary
build:
	go build -ldflags "-X main.version=$(VERSION)" -o $(BINARY) .

## install: build and place the binary on GOPATH/bin
install:
	go install -ldflags "-X main.version=$(VERSION)" .

## test: run unit tests (no network, no Terraform binary required)
test:
	go test ./... -timeout 120s

## testacc-mock: run the Terraform-driven end-to-end tests against the in-process
## mock API. Needs a terraform binary on PATH but no credentials and no network.
## This is the verification most worth running before opening a pull request.
testacc-mock:
	TF_ACC=1 go test ./internal/provider/ -run TestE2E -v -timeout 30m

## testacc: run every acceptance test, including those against a real RevenueCat
## project. Those skip unless REVENUECAT_API_KEY and REVENUECAT_PROJECT_ID are
## set; when set, they create and destroy real catalog objects — never point
## them at a production project.
testacc:
	TF_ACC=1 go test ./... -v -timeout 30m

## mock: run the mock RevenueCat API server locally on :8080
mock:
	go run ./cmd/mock-revenuecat -addr :8080 -api-key sk-mock -seed-project-id proj_mock

## compose-e2e: apply the complete example against the mock in containers
compose-e2e:
	docker compose up --build --abort-on-container-exit --exit-code-from e2e
	docker compose down -v

## fmt: format Go sources
fmt:
	gofmt -w .

## fmtcheck: fail if any Go source is unformatted
fmtcheck:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "These files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

## vet: run go vet
vet:
	go vet ./...

## lint: formatting and vet together
lint: fmtcheck vet

## tidy: prune and verify module requirements
tidy:
	go mod tidy

## clean: remove build output
clean:
	rm -f $(BINARY)
