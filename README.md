# GoRAG

[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen.svg)](https://github.com/DotNetAge/gorag)

**GoRAG** - Production-ready RAG (Retrieval-Augmented Generation) framework for Go

## Features

- **High Performance** - Built for production with low latency and high throughput
- **Modular Design** - Pluggable parsers, vector stores, and LLM providers
- **Cloud Native** - Kubernetes friendly, single binary deployment
- **Type Safe** - Full type safety with Go's strong typing
- **Production Ready** - Observability, metrics, and error handling built-in
- **Hybrid Retrieval** - Combine vector and keyword search for better results
- **Reranking** - LLM-based result reranking for improved relevance
- **Streaming Responses** - Real-time streaming for better user experience
- **Plugin System** - Extensible architecture for custom functionality
- **CLI Tool** - Command-line interface for easy usage
- **Comprehensive Testing** - 85%+ test coverage with integration tests using Testcontainers

## Quick Start

```go
package main

import (
    "context"
    "log"
    "os"
    
    embedder "github.com/DotNetAge/gorag/embedding/openai"
    llm "github.com/DotNetAge/gorag/llm/openai"
    "github.com/DotNetAge/gorag/parser/text"
    "github.com/DotNetAge/gorag/rag"
    "github.com/DotNetAge/gorag/vectorstore/memory"
)

func main() {
    ctx := context.Background()
    apiKey := os.Getenv("OPENAI_API_KEY")
    
    // Create RAG engine
    embedderInstance, _ := embedder.New(embedder.Config{APIKey: apiKey})
    llmInstance, _ := llm.New(llm.Config{APIKey: apiKey})
    
    engine, err := rag.New(
        rag.WithParser(text.NewParser()),
        rag.WithVectorStore(memory.NewStore()),
        rag.WithEmbedder(embedderInstance),
        rag.WithLLM(llmInstance),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    // Index documents
    err = engine.Index(ctx, rag.Source{
        Type: "text",
        Content: "Go is an open source programming language...",
    })
    
    // Query
    resp, err := engine.Query(ctx, "What is Go?", rag.QueryOptions{
        TopK: 5,
    })
    
    log.Println(resp.Answer)
}
```

## Installation

```bash
go get github.com/DotNetAge/gorag
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    GoRAG                                 │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  Document   │  │   Vector    │  │    LLM      │     │
│  │   Parser    │  │   Store     │  │   Client    │     │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘     │
│         └─────────────────┼─────────────────┘           │
│                           ▼                           │
│                  ┌─────────────────┐                    │
│                  │   RAG Engine    │                    │
│                  └─────────────────┘                    │
└─────────────────────────────────────────────────────────┘
```

## Modules

- **parser** - Document parsers (Text, PDF, DOCX, HTML, etc.)
- **vectorstore** - Vector storage backends (Memory, Milvus, Qdrant, Pinecone, Weaviate, etc.)
- **embedding** - Embedding providers (OpenAI, Ollama, etc.)
- **llm** - LLM clients (OpenAI, Anthropic, etc.)
- **rag** - RAG engine and orchestration
- **plugins** - Plugin system for extending functionality

## CLI Tool

GoRAG includes a command-line interface for easy usage:

```bash
# Install
go install github.com/DotNetAge/gorag/cmd/gorag@latest

# Index documents
gorag index --api-key $OPENAI_API_KEY "Go is an open source programming language..."

# Query the engine
gorag query --api-key $OPENAI_API_KEY "What is Go?"

# Stream responses
gorag query --api-key $OPENAI_API_KEY --stream "What are the key features of Go?"
```

## Examples

- **Basic** - Simple RAG usage example
- **Advanced** - Advanced features including streaming and hybrid retrieval
- **Web** - HTTP API server example

## Testing

GoRAG has comprehensive test coverage with both unit tests and integration tests:

### Test Coverage

- **Overall Coverage**: 85%+ across all modules
- **Unit Tests**: All core modules have comprehensive unit tests
- **Integration Tests**: Real-world testing with actual vector databases using Testcontainers

### Running Tests

```bash
# Run all unit tests
go test ./...

# Run integration tests (requires Docker)
go test -v ./integration_test/...

# Run tests with coverage
go test -cover ./...
```

### Integration Testing

Integration tests use [Testcontainers](https://testcontainers.com/) to spin up real instances of:
- **Milvus** - Vector database for production workloads
- **Qdrant** - High-performance vector search engine
- **Weaviate** - Semantic search engine with GraphQL API

This ensures that GoRAG works correctly with actual vector databases in production environments.

## Documentation

- [Getting Started](docs/getting-started.md)
- [API Reference](docs/api.md)
- [Examples](examples/)
- [Contributing](CONTRIBUTING.md)

## Roadmap

- [x] More vector store integrations (Milvus, Qdrant, Weaviate)
- [x] Advanced retrieval strategies (Hybrid, Reranking)
- [x] Streaming responses
- [ ] Multi-modal support (Image, Audio)
- [x] Plugin system
- [x] CLI tool
- [x] Comprehensive test coverage (85%+)
- [x] Integration tests with Testcontainers
- [ ] Performance benchmarks
- [ ] Multi-tenancy support

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.
