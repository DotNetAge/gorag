# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

GoRAG is a production-ready RAG (Retrieval-Augmented Generation) framework for Go. It provides a modular, high-performance system for document indexing, semantic search, and LLM-powered question answering.

**Key Differentiators:**
- Concurrent directory indexing with 10 workers (unique to GoRAG)
- Streaming parser support for 100M+ files
- Automatic parser selection by file extension
- Multi-hop RAG and Agentic RAG capabilities
- 85%+ test coverage with Testcontainers integration tests

## Build and Development Commands

### Building
```bash
# Build the CLI tool
go build -o gorag ./cmd/gorag

# Install CLI globally
go install github.com/DotNetAge/gorag/cmd/gorag@latest
```

### Testing
```bash
# Run all unit tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./rag/
go test ./parser/

# Run integration tests (requires Docker)
go test -v ./integration_test/...

# Run benchmarks
go test -bench=. ./rag/
go test -bench=. ./rag/ -benchmem
```

### Running Examples
```bash
# Basic example
cd examples/basic && go run main.go

# Advanced features (streaming, hybrid retrieval)
cd examples/advanced && go run main.go

# Concurrent directory indexing
cd examples/concurrent-indexing && go run main.go

# Web server
cd examples/web && go run main.go
```

### CLI Usage
```bash
# Index documents
./gorag index --api-key $OPENAI_API_KEY --file README.md

# Query
./gorag query --api-key $OPENAI_API_KEY "What is GoRAG?"

# Stream responses
./gorag query --api-key $OPENAI_API_KEY --stream "Explain RAG"

# Start HTTP server
./gorag serve --api-key $OPENAI_API_KEY --port 8080
```

## Architecture

### Core Components

**Engine (`rag/engine.go`)**: Central orchestrator that coordinates all RAG operations. Manages parsers, embedders, vector stores, and LLM clients.

**Modular Subsystems:**
- `rag/indexing/`: Document indexing with concurrent processing
- `rag/query/`: Query handling and response generation
- `rag/retrieval/`: Hybrid retrieval, reranking, multi-hop, and agentic RAG

### Module Structure

```
gorag/
├── core/              # Shared data structures (Chunk, Result, Source)
├── parser/            # 9 document parsers (text, pdf, docx, html, json, yaml, excel, ppt, image)
├── embedding/         # 4 embedding providers (openai, ollama, cohere, voyage)
├── llm/               # 5 LLM clients (openai, anthropic, azureopenai, ollama, compatible)
├── vectorstore/       # 5 vector stores (memory, milvus, qdrant, pinecone, weaviate)
├── rag/               # RAG engine and orchestration
│   ├── indexing/      # Document indexing logic
│   ├── query/         # Query processing
│   └── retrieval/     # Retrieval strategies (hybrid, multi-hop, agentic)
├── config/            # Configuration management (YAML + env vars)
├── observability/     # Metrics, logging, tracing
├── plugins/           # Plugin system
└── cmd/gorag/         # CLI tool
```

### Key Design Patterns

**Parser System:**
- Parsers are registered by file extension in a map
- `Engine.AddParser(ext, parser)` adds custom parsers
- Default parser (usually text) handles unknown types
- Streaming parsers implement `StreamingParser` interface for large files

**Concurrent Indexing:**
- `IndexDirectory()` uses 10 goroutines with worker pool pattern
- Errors are aggregated and returned together
- Context cancellation supported for graceful shutdown

**Retrieval Pipeline:**
1. Optional HyDE (Hypothetical Document Embeddings) for query enhancement
2. Optional RAG-Fusion for multi-perspective retrieval
3. Hybrid retrieval (vector + keyword search)
4. Optional LLM-based reranking
5. Optional context compression

**Multi-hop RAG:**
- Breaks complex questions into multiple retrieval steps
- Each hop refines the query based on previous results
- Configurable max hops (default: 3)

**Agentic RAG:**
- Autonomous decision-making for retrieval
- LLM decides what information to retrieve and when
- Iterative refinement until sufficient context is gathered

## Important Implementation Notes

### When Adding New Parsers
1. Implement `parser.Parser` interface in `parser/<format>/`
2. For large files, implement `parser.StreamingParser` interface
3. Register parser in `rag/parser_loader.go` `loadBuiltInParsers()`
4. Add file extension mapping in parser registration

### When Adding New Vector Stores
1. Implement `vectorstore.Store` interface in `vectorstore/<name>/`
2. Add integration test in `integration_test/vectorstore/<name>_test.go`
3. Use Testcontainers for real database testing
4. Update configuration schema in `config/`

### When Adding New Embedding Providers
1. Implement `embedding.Provider` interface in `embedding/<name>/`
2. Add configuration struct with API key and model fields
3. Support batch embedding for efficiency
4. Handle rate limiting and retries

### When Adding New LLM Clients
1. Implement `llm.Client` interface in `llm/<name>/`
2. Support both streaming and non-streaming responses
3. Implement proper error handling for API failures
4. Add timeout and retry logic

### Testing Guidelines
- Unit tests should mock external dependencies
- Integration tests use Testcontainers for real services
- Benchmarks should test both small and large-scale operations
- Test coverage target: 85%+

### Configuration
- YAML files for static configuration (`config.yaml`)
- Environment variables override YAML (prefix: `GORAG_`)
- Configuration loaded via `config/` package
- Support for multiple embedding/LLM providers

## Common Patterns

### Creating a RAG Engine
```go
engine, err := rag.New(
    rag.WithVectorStore(memory.NewStore()),
    rag.WithEmbedder(embedderInstance),
    rag.WithLLM(llmInstance),
    rag.WithParser(textParser), // Optional: default parser
)
```

### Adding Custom Parsers
```go
htmlParser := html.NewParser()
engine.AddParser("html", htmlParser)
engine.AddParser("htm", htmlParser)
```

### Concurrent Directory Indexing
```go
// Synchronous with 10 workers
err := engine.IndexDirectory(ctx, "./documents")

// Asynchronous background processing
err := engine.AsyncIndexDirectory(ctx, "./large-docs")
```

### Advanced Query Options
```go
resp, err := engine.Query(ctx, "question", rag.QueryOptions{
    TopK: 5,
    UseHyDE: true,
    UseRAGFusion: true,
    UseContextCompression: true,
    UseMultiHopRAG: true,
    MaxHops: 3,
    PromptTemplate: "Custom template: {context}\n{question}",
})
```

## Performance Considerations

- **Indexing**: ~20-76ms per document depending on scale
- **Query**: ~7-27s depending on document count and complexity
- **Concurrent workers**: 10 workers for directory indexing
- **Streaming**: Use streaming parsers for files >10MB
- **Batch embedding**: Embed multiple chunks together for efficiency

## Dependencies

- Go 1.24.0+
- Docker (for integration tests)
- External services: OpenAI, Anthropic, Ollama, Milvus, Qdrant, etc. (optional)

## Code Style

- Follow standard Go conventions
- Use interfaces for extensibility
- Prefer composition over inheritance
- Keep functions focused and testable
- Document exported types and functions
