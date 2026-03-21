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

## 🚀 Quick Start: Build Industrial RAG in 1 Minute

GoRAG provides a unified **`RAG` application interface** that bundles Indexing and Retrieval into one seamless entity. Choose your preset and start building.

### 1. NativeRAG (Perfect for AI Agents & Local Knowledge Bases)
*Pure Go, zero-dependencies (SQLite + GoVector). One-line setup.*

```go
import "github.com/DotNetAge/gorag"

// 1. One line to create a complete local RAG app
app, _ := gorag.DefaultNativeRAG(gorag.WithWorkDir("./my_kb"))

// 2. Feed it documents
app.IndexDirectory(ctx, "./docs", true)

// 3. Ask questions!
res, _ := app.Search(ctx, "What is GoRAG?", 5)
fmt.Println(res.Answer)
```

### 2. AdvancedRAG (Enterprise-Grade / High Recall)
*Designed for distributed scale. Bundles RAG-Fusion + RRF best practices.*

```go
// Connect to enterprise Milvus and start an Advanced RAG app
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithMilvus("milvus:19530", "kb_collection"),
    gorag.WithOpenAI("sk-xxxx"),
)

app.IndexDirectory(ctx, "./enterprise_docs", true)
res, _ := app.Search(ctx, "Compare architecture A vs B", 10)
```

### 3. GraphRAG (Deep Reasoning / Relational)
*Automated Knowledge Graph construction with hybrid vector-graph search.*

```go
// Bundles Neo4j relationship reasoning with Vector search
app, _ := gorag.DefaultGraphRAG(
    gorag.WithMilvus("milvus:19530", "kb_collection"),
    gorag.WithNeo4j("neo4j://localhost", "user", "pass"),
)

app.IndexFile(ctx, "corporate_report.pdf")
res, _ := app.Search(ctx, "How are entity X and Y related?", 5)
```

---

## 🔭 Built-in Industrial Observability

Stop flying blind. GoRAG natively supports **Prometheus** and **OpenTelemetry** to monitor your RAG pipelines in production.

```go
idx, _ := indexer.DefaultAdvancedIndexer(vStore, dStore, 
    indexer.WithZapLogger("./logs/rag.log", 100, 30, 7, true), // Industrial Logging
    indexer.WithPrometheusMetrics(":8080"),                   // Metrics
    indexer.WithOpenTelemetryTracer(ctx, "jaeger:4317", "RAG"),// Distributed Tracing
)
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
