# GoRAG

[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**GoRAG** - Production-ready RAG (Retrieval-Augmented Generation) framework for Go

## Features

- **High Performance** - Built for production with low latency and high throughput
- **Modular Design** - Pluggable parsers, vector stores, and LLM providers
- **Cloud Native** - Kubernetes friendly, single binary deployment
- **Type Safe** - Full type safety with Go's strong typing
- **Production Ready** - Observability, metrics, and error handling built-in

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    "github.com/DotNetAge/gorag/rag"
    "github.com/DotNetAge/gorag/parser/text"
    "github.com/DotNetAge/gorag/vectorstore/memory"
    "github.com/DotNetAge/gorag/embedding/openai"
    "github.com/DotNetAge/gorag/llm/openai"
)

func main() {
    ctx := context.Background()
    
    // Create RAG engine
    engine, err := rag.New(
        rag.WithParser(text.NewParser()),
        rag.WithVectorStore(memory.NewStore()),
        rag.WithEmbedder(openai.NewEmbedder(apiKey)),
        rag.WithLLM(openai.NewClient(apiKey)),
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

## Documentation

- [Getting Started](docs/getting-started.md)
- [API Reference](docs/api.md)
- [Examples](examples/)
- [Contributing](CONTRIBUTING.md)

## Roadmap

- [ ] More vector store integrations (Milvus, Qdrant)
- [ ] Advanced retrieval strategies (Hybrid, Reranking)
- [ ] Streaming responses
- [ ] Multi-modal support (Image, Audio)
- [ ] Plugin system
- [ ] CLI tool

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.
