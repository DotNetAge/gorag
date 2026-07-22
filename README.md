<div align="center">
  <h1>GoRAG</h1>
  <p><b>A Local RAG (Retrieval-Augmented Generation) Toolkit</b></p>

  [![Go Version](https://img.shields.io/badge/go-1.25%2B-blue.svg)](https://golang.org)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

GoRAG is a local-first RAG toolkit with both CLI and Go API, supporting semantic vector search, graph-based retrieval, and hybrid indexing.

---

## Features

- **Semantic Search**: Multi-dimension vector matching on Title / Summary / Content
- **Graph Search**: Multi-hop neighbor traversal via knowledge graph, native Cypher support
- **Hybrid Indexing** (Hyper): Dual pipeline orchestration of semantic + graph, fused search results
- **LLM Enhancement**: Automatic title/summary/tag generation for chunks, entity and relation extraction
- **Region System**: Automatic directory-to-Region mapping, auto-generated README summaries
- **Incremental Indexing**: mtime+size+hash change detection, re-index only changed files
- **Incremental LLM Processing**: Per-chunk status tracking, resume from breakpoint, auto-reprocess on content change
- **Progress Tracking**: SQLite metadata store for real-time index and LLM status
- **Multi-format Support**: PDF / DOCX / HTML / EPUB / PPTX / Markdown / CSV / XLSX / JSON / YAML / images / code
- **Zero CGO**: Pure Go, painless cross-compilation

---

## Installation

### Homebrew

```bash
brew install DotNetAge/homebrew-gorag/gorag
```

### From source

```bash
go install github.com/DotNetAge/gorag/v2/cmd@latest
```

### Pre-built binaries

Download from [GitHub Releases](https://github.com/DotNetAge/gorag/releases).

---

## Quick Start

### CLI

```bash
# 1. Initialize a RAG library in your project
cd my-project
grag init

# 2. Index files
grag index .

# 3. Semantic search
grag query "What is GoRAG"

# 4. Check status
grag status

# 5. Optional: enable LLM enhancement
export GORAG_API_KEY=sk-xxx
grag update . --llm-url https://api.openai.com/v1 --llm-model gpt-4o-mini

# 6. Graph exploration
grag nodes ./src -n 2

# 7. Directory tree
grag tree
```

### Go API

```go
import gorag "github.com/DotNetAge/gorag/v2"

svc, err := gorag.NewRAGService("./my-project.rag")
if err != nil {
    log.Fatal(err)
}
defer svc.Stop()

ctx := context.Background()
svc.IndexerSvc().Index(ctx, "./docs")

hit, _ := svc.Querier().Query(ctx, "RAG architecture design", "")
result, _ := svc.Explorer().Nodes(ctx, "./docs", 2)
```

---

## CLI Reference

| Command                             | Description                               |
| ----------------------------------- | ----------------------------------------- |
| `grag init [-t type]`               | Initialize a RAG library                  |
| `grag index [path]`                 | Index files or directories                |
| `grag update [path] [llm-options]`  | Incremental update + LLM enhancement      |
| `grag query <text> [-f] [-k]`       | Semantic search (multi-keyword with `\|`) |
| `grag chunks [-p] [-s] [-f]`        | Paginated chunk listing                   |
| `grag nodes [dir] [-n]`             | Directory-level multi-hop graph query     |
| `grag cypher <query>`               | Run Cypher graph query                    |
| `grag status [-s] [-f] [--summary]` | Index and LLM processing status           |
| `grag tree`                         | Directory tree view                       |
| `grag info`                         | Library information                       |
| `grag doctor`                       | Configuration diagnostics                 |
| `grag logs`                         | View logs                                 |

---

## Core Concepts

### .rag Library

Each RAG project corresponds to a `.rag` directory:

```
.rag/
├── config.yml          # Configuration (indexer type, model path, LLM, etc.)
├── meta.db             # SQLite metadata store (document/chunk status)
├── vectors/            # Vector store
├── graph/              # Graph store (graph/hyper indexer only)
├── logs/               # Runtime logs
└── model/              # Embedding model file
```

### Indexer Types

| Type       | Description                                |
| ---------- | ------------------------------------------ |
| `semantic` | Pure vector semantic indexing              |
| `graph`    | Pure graph structure indexing              |
| `hyper`    | Semantic + graph hybrid indexing (default) |

### Chunk

The smallest indexable unit with Title, Summary, Content, Tags, Source, RegionID.

### Region

A directory-level semantic abstraction. Each indexed directory maps to a Region node:
- **RegionID**: SHA256 hash of the absolute directory path
- **Auto-README**: System generates summary README.md for directories without one

---

## Architecture

```
┌──────────────────────────────────────────┐
│              CLI (grag)                  │
│   cmd/main.go + cmd/info.go             │
└────────────────┬─────────────────────────┘
                 │
┌────────────────▼─────────────────────────┐
│         IndexingService (Aggregate)       │
│  ┌─────────────────────────────────────┐  │
│  │  IndexerSvc · QuerySvc · GraphSvc  │  │
│  │  AdminSvc  · RegionSvc · LLMSvc    │  │
│  └─────────────────────────────────────┘  │
└────────────────┬─────────────────────────┘
                 │
    ┌────────────┼────────────┐
    ▼            ▼            ▼
 Semantic    Graph       Hyper(Orchestrator)
 Indexer    Indexer     ┌────┴────┐
                        ▼         ▼
                    Semantic   Graph
                    Indexer    Indexer
```

- **SemanticIndexer**: Chunk → vectorize → write to VectorStore
- **GraphIndexer**: Entities/relationships → write to GraphStore
- **HyperIndexer**: Orchestrates semantic + graph pipelines, supports Summarizer / Refiller injection

---

## LLM Enhancement

`grag update` runs a two-phase incremental LLM pipeline:

1. **Summarizer**: Generates Title / Summary / Tags for document-class chunks
2. **Refiller**: Extracts entities and relationships based on registered Schemas, writes to GraphStore

Configuration:

```bash
grag update . \
  --llm-key <API_KEY> \
  --llm-url https://api.openai.com/v1 \
  --llm-model gpt-4o-mini \
  --schema ./schemas
```

Environment variable: `GORAG_API_KEY`

---

## Documentation

- [CLI Reference](./docs/v2/CLI.md) — Complete command reference
- [Service Layer Guide](./docs/v2/Services.md) — Go API documentation
- [Region System](./docs/v2/Region.md) — Region abstraction design

---

## License

GoRAG is released under the [MIT License](./LICENSE).
