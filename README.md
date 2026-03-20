<div align="center">
  <h1>🦖 GoRAG</h1>
  <p><b>The Expert-Grade, High-Performance Modular RAG Framework for Go</b></p>
  
  [![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
  [![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://golang.org)
  
  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

**GoRAG** is a production-ready Retrieval-Augmented Generation (RAG) framework built for high-scale AI engineering. Unlike complex "black-box" frameworks, GoRAG provides a **transparent, pipeline-based architecture** that combines Go's native concurrency with advanced RAG patterns.

From **GraphRAG** with automated triple extraction to **Agentic RAG** with self-correction, GoRAG is designed to move your AI applications from "prototype" to "production" with zero friction.

## ✨ Why GoRAG?

- 🚀 **Performance First**: Built-in concurrent workers and streaming parsers with `O(1)` memory efficiency. Perfect for indexing TB-scale knowledge bases.
- 🏗️ **Pipeline-Based Architecture**: Powered by `gochat/pkg/pipeline`. Every retrieval step is explicit, traceable, and pluggable. No more "hidden magic" or deep inheritance hell.
- 🧠 **Smart Intent Routing**: Automatically dispatches queries to the most suitable retrieval strategy (Vector, Graph, or Global) based on user intent.
- 🕸️ **Advanced GraphRAG**: Native support for **Neo4j**, **SQLite (Zero-CGO)**, and **BoltDB**. Includes automated LLM-driven knowledge graph construction.
- 🔭 **Built-in Observability**: Comprehensive distributed tracing across all core retrievers and steps. See exactly where your time and tokens go.
- 📊 **Enterprise-Grade Evaluation**: Built-in benchmarking protocol for **Faithfulness**, **Answer Relevance**, and **Context Precision** (RAGAS-style).

---

## 🧰 The RAG "Expert" Ecosystem

GoRAG doesn't just give you tools; it gives you **pre-optimized strategies** as first-class citizens:

| Strategy | When to use | Key Features |
|----------|-------------|--------------|
| **Native RAG** | Standard semantic search | Vector-only, fast, low cost |
| **Graph RAG** | Complex relationship reasoning | Entities, Triples, Multi-hop reasoning |
| **Self-RAG** | High accuracy requirements | Self-reflection, Hallucination detection |
| **CRAG** | Handling ambiguous queries | Quality evaluation, fallback to Web Search |
| **Fusion RAG**| Multi-faceted queries | Query rewriting, RRF fusion |
| **Smart Router**| Dynamic workloads | Intent-based automatic dispatching |

---

## 🚀 Quick Start

### Installation

```bash
go get github.com/DotNetAge/gorag
```

### 1. The Smart Router: Intent-Based Retrieval
Automatically choose between Vector Search for domain facts and Graph Search for relationship reasoning:

```go
package main

import (
    "context"
    "github.com/DotNetAge/gorag/pkg/retriever/agentic"
    "github.com/DotNetAge/gorag/pkg/retriever/graph"
    "github.com/DotNetAge/gorag/pkg/retriever/native"
)

func main() {
    // 1. Setup retrievers
    vectorRet := native.NewRetriever(vectorStore, embedder, llm)
    graphRet := graph.NewRetriever(vectorStore, graphStore, embedder, llm)

    // 2. Create a Smart Router
    router := agentic.NewSmartRouter(
        classifier, 
        map[core.IntentType]core.Retriever{
            core.IntentRelational: graphRet,  // Use Graph for relationship queries
            core.IntentDomain:     vectorRet, // Use Vector for specific facts
        },
        vectorRet, // Default fallback
        logger,
    )

    // 3. Just ask! The router handles the "how"
    results, _ := router.Retrieve(ctx, []string{"How are Project X and Person Y related?"}, 5)
    fmt.Println(results[0].Answer)
}
```

### 2. Automated Knowledge Graph Indexing
Turn unstructured text into a queryable knowledge graph with one click:

```go
// Initialize the triple-based indexing step
triplesStep := indexing.NewTriplesStep(llm, graphStore)

// Process your documents - GoRAG extracts (S, P, O) automatically
err := indexer.IndexDirectory(ctx, "./docs", true)
```

### 3. Production Observability & Benchmarking
Track every step and measure quality using built-in tools:

```go
// Run a benchmark against your dataset
report, _ := evaluation.RunBenchmark(ctx, retriever, judge, testCases, 5)
fmt.Println(report.Summary())
// Output: Avg Faithfulness: 0.92, Avg Relevance: 0.88, Avg Precision: 0.85
```

---

## ⚡ Technical Integrity & Standards

- **Go 1.24+**: Leveraging the latest language features.
- **Zero-CGO SQLite**: Using `modernc.org/sqlite` for painless cross-compilation.
- **Clean Architecture**: Strict separation of interfaces (`pkg/core`) and implementations.
- **Modular Steps**: Reuse `hyde`, `rerank`, `fuse`, or `prune` steps in any custom pipeline.

## 🤝 Contributing
We aim to build the most robust AI infrastructure for the Go ecosystem. Whether it's a new `VectorStore` driver or an improved `Parser`, your PRs are welcome! 
- Check our [Contributing Guidelines](CONTRIBUTING.md).

## 📄 License
GoRAG is licensed under the [MIT License](LICENSE).
