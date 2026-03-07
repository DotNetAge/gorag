.PHONY: test coverage lint build clean install help

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## test: Run all tests
test:
	go test -v ./...

## test-short: Run tests without integration tests
test-short:
	go test -v -short ./...

## coverage: Run tests with coverage report
coverage:
	go test -cover ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## coverage-summary: Show coverage summary
coverage-summary:
	go test -cover ./...

## lint: Run linter
lint:
	golangci-lint run

## build: Build the CLI binary
build:
	go build -o bin/gorag ./cmd/gorag

## build-all: Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o bin/gorag-linux-amd64 ./cmd/gorag
	GOOS=darwin GOARCH=amd64 go build -o bin/gorag-darwin-amd64 ./cmd/gorag
	GOOS=darwin GOARCH=arm64 go build -o bin/gorag-darwin-arm64 ./cmd/gorag
	GOOS=windows GOARCH=amd64 go build -o bin/gorag-windows-amd64.exe ./cmd/gorag

## install: Install the CLI binary
install:
	go install ./cmd/gorag

## clean: Clean build artifacts
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

## bench: Run benchmarks
bench:
	go test -bench=. -benchmem ./rag/

## integration: Run integration tests (requires Docker)
integration:
	go test -v ./integration_test/...

## fmt: Format code
fmt:
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## mod-tidy: Tidy go modules
mod-tidy:
	go mod tidy

## mod-verify: Verify go modules
mod-verify:
	go mod verify

## deps: Download dependencies
deps:
	go mod download

## all: Run fmt, vet, lint, test
all: fmt vet lint test

## ci: Run CI checks (fmt, vet, test, coverage)
ci: fmt vet test coverage-summary
