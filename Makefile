.PHONY: build build-all install clean test test-short coverage coverage-summary \
        lint fmt vet deps mod-tidy mod-verify bench integration ci help

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
	@echo "Building gorag CLI..."
	@mkdir -p bin
	go build -o bin/gorag ./cmd
	@echo "✓ Binary built: bin/gorag"

## build-all: Build for multiple platforms to bin/
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build -o bin/gorag-linux-amd64 ./cmd
	GOOS=darwin GOARCH=amd64 go build -o bin/gorag-darwin-amd64 ./cmd
	GOOS=darwin GOARCH=arm64 go build -o bin/gorag-darwin-arm64 ./cmd
	GOOS=windows GOARCH=amd64 go build -o bin/gorag-windows-amd64.exe ./cmd
	@echo "✓ All binaries built in bin/"

## install: Install the CLI binary to GOPATH/bin
install:
	@echo "Installing gorag CLI..."
	go install ./cmd
	@echo "✓ Installed to $(GOPATH)/bin/gorag"

## clean: Clean build artifacts and test files
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	rm -rf .test/
	@echo "✓ Cleaned"

## test: Run all tests
test:
	@echo "Running tests..."
	go test -v -timeout 120s ./...

## test-short: Run short tests without integration tests
test-short:
	@echo "Running short tests..."
	go test -v -short ./...

## coverage: Run tests with coverage report
coverage:
	@echo "Generating coverage report..."
	go test -cover ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

## coverage-summary: Show coverage summary
coverage-summary:
	go test -cover ./...

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	golangci-lint run

## fmt: Format code with go fmt
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Code formatted"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## deps: Download module dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	@echo "✓ Dependencies downloaded"

## mod-tidy: Tidy go modules
mod-tidy:
	@echo "Tidy modules..."
	go mod tidy
	@echo "✓ Modules tidied"

## mod-verify: Verify go modules
mod-verify:
	@echo "Verifying modules..."
	go mod verify
	@echo "✓ Modules verified"

## bench: Run benchmarks
bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

## integration: Run integration tests (requires Docker)
integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./...

## ci: Run CI checks (fmt, vet, test, coverage)
ci: fmt vet test coverage-summary
	@echo "✓ CI checks passed"

## all: Run fmt, vet, lint, test
all: fmt vet lint test
	@echo "✓ All checks passed"

## dev: Build and run quick test
dev: build
	@echo "Running quick test..."
	./bin/gorag --help
	@echo "✓ Development build ready"
