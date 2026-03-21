.PHONY: test coverage lint build clean install help models

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## models: Sequentially download all required models
models: model-text model-multimodal

## model-text: Download the Chinese BGE text embedding model (Grounding)
model-text:
	@echo ">>> Queuing Text Model: bge-small-zh-v1.5..."
	@mkdir -p .test/models
	./bin/gorag download --model bge-small-zh-v1.5 --output .test/models

## model-multimodal: Download the CLIP multimodal model (Grounding)
model-multimodal:
	@echo ">>> Queuing Multimodal Model: clip-vit-base-patch32..."
	@mkdir -p .test/models
	./bin/gorag download --model clip-vit-base-patch32 --output .test/models

## check: Diagnose environment and models
check:
	./bin/gorag check

## test: Run all tests with diagnostic timeout
test: check
	@# Setting a strict 120s timeout to catch hanging tests
	go test -v -timeout 120s ./...

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
	rm -rf .models/

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
