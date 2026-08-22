BINARY  := terraform-provider-revenuecat
VERSION ?= dev

.PHONY: default build install test testacc fmt fmtcheck vet lint tidy clean

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

## testacc: run acceptance tests against a real RevenueCat project.
## Requires a Terraform binary, REVENUECAT_API_KEY and REVENUECAT_PROJECT_ID.
## These create and destroy real catalog objects — never point them at a
## production project.
testacc:
	TF_ACC=1 go test ./... -v -timeout 30m

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
