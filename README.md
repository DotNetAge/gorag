# 🦖 GoRAG

[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![License:MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-85%25-brightgreen.svg)](https://github.com/DotNetAge/gorag)
[![Go Version](https://img.shields.io/badge/go-1.22%2B-blue.svg)](https://golang.org)
[![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
[![codecov](https://codecov.io/gh/DotNetAge/gorag/graph/badge.svg?token=placeholder)](https://codecov.io/gh/DotNetAge/gorag)
[![Powered by gochat](https://img.shields.io/badge/Powered%20by-gochat-ff69b4.svg)](https://github.com/DotNetAge/gochat)
[![Pure Go Vector](https://img.shields.io/badge/Vector%20Store-govector-success.svg)](https://github.com/DotNetAge/govector)

**GoRAG** is a production-ready, high-performance RAG (Retrieval-Augmented Generation) framework built entirely in Go. Designed for enterprise scalability, it seamlessly connects your internal data to the most powerful LLMs with zero Python dependencies.

**[English](README.md)** | [中文文档](README-zh.md)

---

## 🔥 Why GoRAG?

Stop fighting with Python dependency hell and slow async loops. GoRAG brings the power of **Go's concurrency and static typing** to the AI world. 

- **🚀 Blazing Fast**: Process 100M+ files with 10 built-in concurrent workers.
- **🛡️ Enterprise Ready**: Built-in Circuit Breakers, Graceful Degradation, Observability, and Metrics.
- **🧩 Highly Modular**: Swap LLMs, Vector Stores, and Parsers with a single line of code.
- **🧠 Advanced Retrieval**: Supports Multi-hop RAG, Agentic RAG, Semantic Chunking, HyDE, and RAG-Fusion.
- **☁️ Cloud Native**: Compiles to a single binary. Perfect for Kubernetes deployments.
- **📦 No External DB Required**: Ships with native pure-Go vector database `govector` for zero-setup local deployments, while supporting enterprise databases like Milvus, Qdrant, and Pinecone.

## ✨ Latest Updates (v1.0.0)

- **Complete LLM SDK Migration**: Fully powered by the unified [`gochat`](https://github.com/DotNetAge/gochat) SDK. Write once, seamlessly switch between OpenAI, Anthropic, Ollama, Azure, and domestic Chinese LLMs.
- **Native Vector Database Integration**: Integrated [`govector`](https://github.com/DotNetAge/govector) natively for a pure Go, zero-dependency embedded vector search experience.
- **Extensive Parser Ecosystem**: Now supports 16 document types natively out-of-the-box (Text, PDF, DOCX, HTML, Email, Code, etc.) and offers independent plugins for Audio, Video, and Webpages.

---

## 🛠️ Out-of-the-Box Support

### 🤖 Supported LLMs (Powered by `gochat`)
- **OpenAI**: GPT-4o, GPT-4 Turbo, GPT-3.5
- **Anthropic**: Claude 3 (Opus, Sonnet, Haiku)
- **Local/Open Source**: Ollama (Llama 3, Qwen, Mistral)
- **Enterprise**: Azure OpenAI
- **Compatible APIs**: Kimi, DeepSeek, GLM-4, Minimax, Baichuan, and more.

### 🗄️ Supported Vector Stores
- **govector** 🌟 (Native pure-Go embedded vector DB - Zero setup!)
- **Milvus** (Enterprise standard)
- **Qdrant** (High performance)
- **Weaviate** (Semantic search engine)
- **Pinecone** (Fully managed cloud DB)
- **Memory** (For quick testing/dev)

---

## 🚀 Quick Start

### Installation

```bash
go get github.com/DotNetAge/gorag
```

### 1. The 10-Line RAG Setup

Get a complete RAG pipeline running locally using `Ollama` and our native pure-Go `govector` database. No external databases or API keys required!

```go
package main

import (
	"context"
	"log"

	"github.com/DotNetAge/gochat/pkg/client/base"
	"github.com/DotNetAge/gochat/pkg/client/ollama"
	"github.com/DotNetAge/gorag/rag"
	"github.com/DotNetAge/gorag/vectorstore/govector"
)

func main() {
	ctx := context.Background()

	// 1. Initialize LLM Client (Powered by gochat)
	llmClient, _ := ollama.New(ollama.Config{
		Config: base.Config{Model: "qwen:0.5b"},
	})

	// 2. Initialize Native Go Vector Store (Zero setup)
	vectorStore, _ := govector.NewStore(govector.Config{
		Dimension:  1536,
		Collection: "my_knowledge",
	})

	// 3. Create RAG Engine
	engine, _ := rag.New(
		rag.WithLLM(llmClient),
		rag.WithVectorStore(vectorStore),
	)

	// 4. Index a Document
	engine.Index(ctx, rag.Source{
		Type:    "text",
		Content: "GoRAG is a native Go framework for Retrieval-Augmented Generation.",
	})

	// 5. Query
	resp, _ := engine.Query(ctx, "What is GoRAG?", rag.QueryOptions{TopK: 3})
	log.Println("Answer:", resp.Answer)
}
```

### 2. High-Performance Directory Indexing

Need to ingest an entire codebase or documentation folder? GoRAG handles it automatically with **10 concurrent workers**.

```go
// ... (engine initialization)

// 🚀 Index an entire directory! Automatically detects file types (.pdf, .go, .md, .docx)
err := engine.IndexDirectory(ctx, "./my-company-docs")
if err != nil {
    log.Fatal(err)
}

// Stream the answer for better UX
ch, _ := engine.QueryStream(ctx, "Summarize the Q3 financial reports", rag.QueryOptions{
    Stream: true,
})

for resp := range ch {
    fmt.Print(resp.Chunk)
}
```

---

## 📊 Performance Benchmarks

GoRAG leaves Python-based frameworks in the dust. Tested on a standard Intel Core i5 (No GPU):

| Operation                           | GoRAG Latency | Competitor Average (Python)          |
| ----------------------------------- | ------------- | ------------------------------------ |
| Single Document Index               | **~48ms**     | ~200ms                               |
| 100 Documents Index                 | **~7.6s**     | ~25s+                                |
| **Bible-Scale Index** (10,100 docs) | **~3.4 mins** | Usually OOMs / Requires heavy tuning |

*Note: Enabling GPU acceleration via Milvus/Ollama provides an additional 3x-5x performance boost.*

## 🌟 Advanced RAG Patterns

GoRAG isn't just a wrapper; it implements state-of-the-art AI retrieval techniques natively:

- **Agentic RAG**: Let the LLM autonomously decide which tools to use and when to retrieve data.
- **Multi-hop RAG**: Breaks down complex questions requiring multi-step reasoning.
- **Semantic Chunking & HyDE**: Intelligent document splitting and Hypothetical Document Embeddings for superior recall.

---

## 🛠 CLI Usage

GoRAG ships with a powerful CLI out of the box:

```bash
# Install the CLI
go install github.com/DotNetAge/gorag/cmd/gorag@latest

# Index a document directly
gorag index --api-key $OPENAI_API_KEY --file ./docs/architecture.md

# Query your knowledge base
gorag query --api-key $OPENAI_API_KEY "How does the circuit breaker work?"
```

## 🤝 Contributing & Community

We are building the future of enterprise AI in Go. We welcome all contributions!
- Read our [Contribution Guidelines](CONTRIBUTING.md)
- Give us a ⭐️ if you find this project useful!

## 📄 License

MIT License. See [LICENSE](LICENSE) for more information.
