.PHONY: build install clean test test-short coverage coverage-summary fmt vet lint \
        mod-tidy mod-verify bench dev release

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Build the CLI binary to bin/
build:
	@echo "Building grag CLI..."
	@mkdir -p bin
	go build -o bin/grag ./cmd
	@echo "Done: bin/grag"

## install: Install the CLI binary as `grag` to GOPATH/bin
install:
	@mkdir -p "$(shell go env GOPATH)/bin"
	go build -o "$(shell go env GOPATH)/bin/grag" ./cmd
	@echo "Done: $(shell go env GOPATH)/bin/grag"

## clean: Clean build artifacts and test files
clean:
	rm -rf bin/ coverage.out coverage.html

## test: Run all tests
test:
	go test -v -timeout 120s ./...

## test-short: Run short tests without heavy integration
test-short:
	go test -v -short ./...

## coverage: Run tests with coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## coverage-summary: Show coverage summary
coverage-summary:
	go test -cover ./...

## fmt: Format code with go fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...

## vet: Run go vet
vet:
	go vet ./...

## lint: Run golangci-lint
lint:
	golangci-lint run

## mod-tidy: Tidy go modules
mod-tidy:
	go mod tidy

## mod-verify: Verify go modules
mod-verify:
	go mod verify

## bench: Run benchmarks
bench:
	go test -bench=. -benchmem ./...

## dev: Build and show help
dev: build
	./bin/grag --help

## release: Simulate a local release with goreleaser (dry-run)
release:
	goreleaser release --snapshot --clean
