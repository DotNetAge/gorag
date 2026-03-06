# GoRAG

[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen.svg)](https://github.com/DotNetAge/gorag)
[![Go Version](https://img.shields.io/badge/go-1.20%2B-blue.svg)](https://golang.org)

**GoRAG** - Production-ready RAG (Retrieval-Augmented Generation) framework for Go

**[English](README.md)** | [中文文档](README-zh.md)

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
- **Multi-modal Support** - Process images and other media types
- **Configuration Management** - Flexible YAML and environment variable configuration
- **Custom Prompt Templates** - Create custom prompt formats with placeholders
- **Performance Benchmarks** - Built-in benchmarking for performance optimization

### 🎯 Out-of-the-Box Support

#### Document Parsers (9 types)
- **Text** - Plain text and markdown files
- **PDF** - PDF documents
- **DOCX** - Microsoft Word documents
- **HTML** - HTML web pages
- **JSON** - JSON data files
- **YAML** - YAML configuration files
- **Excel** - Microsoft Excel spreadsheets (.xlsx)
- **PPT** - Microsoft PowerPoint presentations (.pptx)
- **Image** - Images with OCR support

#### Embedding Providers (2 providers)
- **OpenAI** - OpenAI embeddings (text-embedding-ada-002, text-embedding-3-small, text-embedding-3-large)
- **Ollama** - Local embedding models (bge-small-zh-v1.5, nomic-embed-text, etc.)

#### LLM Clients (5 clients)
- **OpenAI** - GPT-3.5, GPT-4, GPT-4 Turbo, GPT-4o
- **Anthropic** - Claude 3 (Opus, Sonnet, Haiku)
- **Azure OpenAI** - Azure OpenAI Service
- **Ollama** - Local LLMs (Llama 3, Qwen, Mistral, etc.)
- **Compatible** - OpenAI API compatible services (supports domestic LLMs: qwen, seed2, minmax, kimi, glm5, deepseek, etc.)

#### Vector Stores (5 backends)
- **Memory** - In-memory store for development and testing
- **Milvus** - Production-grade vector database
- **Qdrant** - High-performance vector search engine
- **Pinecone** - Fully managed vector database
- **Weaviate** - Semantic search engine with GraphQL API

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
    
    // Query with custom prompt template
    resp, err := engine.Query(ctx, "What is Go?", rag.QueryOptions{
        TopK: 5,
        PromptTemplate: "You are a helpful assistant. Based on the following context:\n\n{context}\n\nAnswer the question: {question}",
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

- **parser** - Document parsers (9 types: Text, PDF, DOCX, HTML, JSON, YAML, Excel, PPT, Image)
- **embedding** - Embedding providers (OpenAI, Ollama)
- **llm** - LLM clients (OpenAI, Anthropic, Azure OpenAI, Ollama, Compatible API)
- **vectorstore** - Vector storage backends (Memory, Milvus, Qdrant, Pinecone, Weaviate)
- **rag** - RAG engine and orchestration
- **plugins** - Plugin system for extending functionality
- **config** - Configuration management system

## CLI Tool

GoRAG includes a command-line interface for easy usage:

```bash
# Install
go install github.com/DotNetAge/gorag/cmd/gorag@latest

# Index documents
gorag index --api-key $OPENAI_API_KEY "Go is an open source programming language..."

# Index from file
gorag index --api-key $OPENAI_API_KEY --file README.md

# Query the engine
gorag query --api-key $OPENAI_API_KEY "What is Go?"

# Stream responses
gorag query --api-key $OPENAI_API_KEY --stream "What are the key features of Go?"

# Use custom prompt template
gorag query --api-key $OPENAI_API_KEY --prompt "You are a helpful assistant. Answer the question: {question}"

# Export indexed documents
gorag export --api-key $OPENAI_API_KEY --file export.json

# Import documents
gorag import --api-key $OPENAI_API_KEY --file export.json
```

## Configuration

GoRAG supports flexible configuration through YAML files and environment variables:

### YAML Configuration

Create a `config.yaml` file:

```yaml
rag:
  topK: 5
  chunkSize: 1000
  chunkOverlap: 100

embedding:
  provider: "openai"
  openai:
    apiKey: "your-api-key"
    model: "text-embedding-ada-002"

llm:
  provider: "openai"
  openai:
    apiKey: "your-api-key"
    model: "gpt-4"

vectorstore:
  type: "milvus"
  milvus:
    host: "localhost"
    port: 19530

logging:
  level: "info"
  format: "json"
```

### Environment Variables

```bash
export GORAG_RAG_TOPK=5
export GORAG_EMBEDDING_PROVIDER=openai
export GORAG_LLM_PROVIDER=openai
export GORAG_VECTORSTORE_TYPE=memory
export GORAG_OPENAI_API_KEY=your-api-key
export GORAG_ANTHROPIC_API_KEY=your-api-key
export GORAG_PINECONE_API_KEY=your-api-key
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
- **Performance Benchmarks**: Built-in benchmarks for Index and Query operations

### Running Tests

```bash
# Run all unit tests
go test ./...

# Run integration tests (requires Docker)
go test -v ./integration_test/...

# Run tests with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./rag/
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
- [Production Deployment Guide](docs/deployment.md)
- [Plugin Development Guide](docs/plugin-development.md)
- [Examples](examples/)
- [Contributing](CONTRIBUTING.md)

## Roadmap

### Completed (v0.5.0)
- [x] Document parsers (9 types)
- [x] Vector stores (5 backends)
- [x] Embedding providers (2 providers)
- [x] LLM clients (5 clients)
- [x] Hybrid retrieval and reranking
- [x] Streaming responses
- [x] Multi-modal support
- [x] Plugin system
- [x] CLI tool
- [x] Comprehensive test coverage (85%+)
- [x] Integration tests with Testcontainers
- [x] Configuration management
- [x] Custom prompt templates
- [x] Performance benchmarks
- [x] Production deployment guides
- [x] Plugin development guide

### Planned (v0.6.0)
- [ ] More parsers (Markdown, CSV, RTF, EPUB)
- [ ] More vector stores (Chroma, Elasticsearch, pgvector, Redis)
- [ ] More embedding providers (Cohere, HuggingFace, Jina, Voyage)
- [ ] More LLM clients (Gemini, Bedrock, Cohere, Mistral)

### Planned (v0.7.0)
- [ ] Advanced chunking strategies
- [ ] Query enhancement (rewriting, expansion, multi-query, HyDE)
- [ ] Retrieval optimization (MMR, filtering, time-weighted)

### Planned (v0.8.0)
- [ ] Performance optimization
- [ ] Developer tools
- [ ] RAG evaluation metrics

### Planned (v0.9.0)
- [ ] Document versioning and incremental indexing
- [ ] Multi-modal enhancement (audio, video)
- [ ] Framework integrations (LangChain, LlamaIndex)

### Future
- [ ] Graph RAG support
- [ ] Adaptive retrieval strategies
- [ ] Plugin marketplace

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and changes.
