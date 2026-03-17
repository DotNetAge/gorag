<div align="center">
  <h1>🦖 GoRAG</h1>
  <p><b>A Production-Ready, High-Performance Modular RAG Framework for Go</b></p>
  
  [![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
  [![codecov](https://codecov.io/gh/DotNetAge/gorag/graph/badge.svg?token=placeholder)](https://codecov.io/gh/DotNetAge/gorag)
  [![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://golang.org)
  
  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

**GoRAG** is an enterprise-grade Retrieval-Augmented Generation (RAG) framework written entirely in Go. Designed for developers who are tired of Python dependency hell and slow async loops, GoRAG brings **high concurrency, memory efficiency, and static type safety** to the AI engineering world. 

Whether you are building a simple document Q&A bot or a complex Agentic RAG system with multi-hop reasoning, GoRAG provides the foundational building blocks you need with zero bloat.

## ✨ Why GoRAG?

- 🚀 **Blazing Fast**: Built-in concurrent workers (10+ goroutines by default) and streaming parsers with `O(1)` memory footprint. Effortlessly index 100M+ scale document repositories.
- 🧩 **Lego-like Modularity**: Strictly follows Clean Architecture. Swap out LLMs, Vector Stores, or Document Parsers with a single line of code.
- 🧠 **Advanced RAG Patterns Built-in**: Out-of-the-box support for HyDE, RAG-Fusion, Semantic Chunking, Cross-Encoder Reranking, and Context Pruning.
- ☁️ **Cloud-Native & Production-Ready**: Compiles to a single binary. Features built-in circuit breakers, rate limiters, graceful degradation, and observability metrics.
- 📦 **Zero-Dependency Quickstart**: Deeply integrated with `govector` (a pure-Go embedded vector database) and `gochat` (a unified LLM SDK). Run a 100% local, privacy-first RAG pipeline without deploying external databases like Milvus or Qdrant.

## 🧰 Ecosystem & Integrations

### 🤖 LLM Providers (Powered by [`gochat`](https://github.com/DotNetAge/gochat))
- **Global**: OpenAI, Anthropic (Claude 3), Azure OpenAI.
- **Local/Open-Source**: Ollama (Llama 3, Qwen, Mistral, etc.).
- **Chinese AI**: Kimi, DeepSeek, GLM-4, Minimax, Baichuan, etc.

### 🗄️ Vector Databases
- **govector** 🌟 (Pure Go embedded vector store - Zero external dependencies!)
- **Milvus / Zilliz** (Enterprise standard)
- **Qdrant** (High-performance Rust engine)
- **Weaviate** (Leading semantic search)
- **Pinecone** (Fully managed cloud DB)

### 📄 Universal Parsers
Native streaming support for **16+ formats** including: Text, PDF, DOCX, Markdown, HTML, CSV, JSON, and source code (Go, Python, Java, TS/JS).

---

## 🚀 Quick Start

### Installation

```bash
go get github.com/DotNetAge/gorag
```

### 10 Lines to Your Private Knowledge Base
Using `Ollama` and our built-in `govector` engine, you can build a 100% local, privacy-first RAG system without any API keys or external database deployments:

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/DotNetAge/gochat/pkg/client/base"
    "github.com/DotNetAge/gochat/pkg/client/ollama"
    "github.com/DotNetAge/gorag/rag"
    "github.com/DotNetAge/gorag/vectorstore/govector"
)

func main() {
    ctx := context.Background()

    // 1. Init LLM Client (via gochat)
    llmClient, _ := ollama.New(ollama.Config{
        Config: base.Config{Model: "qwen:0.5b"},
    })

    // 2. Init Pure-Go Vector Store (Zero dependencies)
    vectorStore, _ := govector.NewStore(govector.Config{
        Dimension:  1536,
        Collection: "my_knowledge",
    })

    // 3. Build RAG Engine
    engine, _ := rag.New(
        rag.WithLLM(llmClient),
        rag.WithVectorStore(vectorStore),
    )

    // 4. Index your private data (Auto-chunking & vectorization)
    engine.Index(ctx, rag.Source{
        Type:    "text",
        Content: "GoRAG is a high-performance RAG framework written in pure Go.",
    })

    // 5. Query
    resp, _ := engine.Query(ctx, "What is GoRAG?", rag.QueryOptions{TopK: 3})
    fmt.Println("Answer:", resp.Answer)
}
```

### High-Concurrency Directory Indexing
Need to process a massive codebase or 50GB of company documents? GoRAG handles it concurrently:

```go
// 🚀 One-click index an entire directory! 
// Auto-detects .pdf, .go, .md, .docx, etc., and routes to the correct parser.
err := engine.IndexDirectory(ctx, "./my-company-docs")

// Stream the response back (Typewriter effect for frontend UX)
ch, _ := engine.QueryStream(ctx, "Summarize the Q3 financial report from the docs", rag.QueryOptions{
    Stream: true,
})

for resp := range ch {
    fmt.Print(resp.Chunk)
}
```

---

## ⚡ Advanced Capabilities

GoRAG is not just a glue framework; it implements cutting-edge retrieval paradigms natively:

- **Agentic RAG / CRAG**: Intelligent routing, self-reflection, and fallback mechanisms for complex queries.
- **RAG-Fusion & Multi-Query**: Rewrites user queries into multiple perspectives, retrieving and applying Reciprocal Rank Fusion (RRF) for higher accuracy.
- **Context Pruning & Cross-Encoder**: Extracts only the most relevant sentences from chunks and reranks them, saving LLM tokens and reducing hallucinations.
- **Graph RAG**: Native support for Neo4j and ArangoDB for cross-node multi-hop reasoning.

---

## 🛠️ CLI Tool
GoRAG comes with a powerful built-in CLI for rapid testing and administration:

```bash
# Install the CLI
go install github.com/DotNetAge/gorag/cmd/gorag@latest

# Index a file directly from the terminal
gorag index --api-key $OPENAI_API_KEY --file ./docs/architecture.md

# Query your knowledge base
gorag query --api-key $OPENAI_API_KEY "How does the circuit breaker work?"
```

## 📈 Roadmap
- [x] Core architecture and pluggable interfaces
- [x] Advanced Enhancers (HyDE, Context Pruning, Reranking)
- [x] Native Graph RAG integration (Neo4j, ArangoDB)
- [ ] Multi-modal RAG (Image & Video indexing)
- [ ] Enterprise Dashboard and API server

## 🤝 Contributing
We welcome contributions! Whether it's adding a new vector store driver, improving the documentation, or fixing a bug, please check out our [Contributing Guidelines](CONTRIBUTING.md).

Give us a ⭐️ if this project helped you build faster and safer AI applications!

## 📄 License
GoRAG is dual-licensed under the [MIT License](LICENSE).
