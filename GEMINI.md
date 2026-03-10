# Gemini Context for GoRAG

## Project Overview

**GoRAG** is a production-ready, high-performance RAG (Retrieval-Augmented Generation) framework written in Go. It is designed with a modular, cloud-native architecture that supports pluggable parsers, vector stores, and LLM providers. 

Key technical features include:
- **Pluggable Architecture**: Components for document parsing (`parser/`), embedding generation (`embedding/`), LLM clients (`llm/`), and vector databases (`vectorstore/`).
- **Advanced RAG Patterns**: Supports Multi-hop RAG, Agentic RAG, Semantic Chunking, HyDE (Hypothetical Document Embeddings), RAG-Fusion, and Context Compression.
- **High Performance**: Features built-in concurrent directory indexing (10 concurrent workers), streaming large file parsers (O(1) memory efficiency), and batch processing optimizations.
- **Robustness**: Includes circuit breaking, connection pooling, cache management, and graceful degradation for production readiness.
- **CLI**: A built-in command-line tool (`cmd/gorag`) for indexing, querying, and managing documents.

## Building and Running

The project relies on standard Go tooling and a `Makefile` for common tasks.

### Key Make Commands:
- **Build**: `make build` (compiles the CLI binary to `bin/gorag`)
- **Test**: `make test` (runs all tests)
- **Short Tests**: `make test-short` (runs tests without integration tests)
- **Integration Tests**: `make integration` (runs integration tests using Testcontainers, requires Docker)
- **Coverage**: `make coverage` / `make coverage-summary`
- **Lint**: `make lint` (runs `golangci-lint`)
- **Format & Check**: `make fmt` (gofmt) and `make vet` (go vet)
- **Benchmarks**: `make bench`

## Development Conventions

- **Language**: Go 1.24+ (as per `go.mod`).
- **Code Style**: Strictly follow standard Go conventions. Always format code with `go fmt` and ensure `go vet` passes.
- **Testing**: Maintain high test coverage (currently 85%+). New features must include comprehensive unit tests. Use Testcontainers for any integration tests requiring external services (e.g., Milvus, Qdrant, Weaviate).
- **Commit Messages**: Follow the Conventional Commits specification (e.g., `feat: ...`, `fix: ...`, `docs: ...`).
- **Architecture**: When adding new functionality (like a new LLM provider or vector store), adhere to the existing interface abstractions and plugin system. Keep components modular and isolated. Heavy dependencies (like CGO for PDF/Audio/Video) are typically separated into independent plugins.