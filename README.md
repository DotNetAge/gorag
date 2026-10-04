<div align="center">
  <h1>GoRAG</h1>
  <p><b>A Local RAG (Retrieval-Augmented Generation) Toolkit</b></p>

  [![Go Version](https://img.shields.io/badge/go-1.25%2B-blue.svg)](https://golang.org)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

GoRAG is a local-first retrieval foundation with both CLI and Go API. It serves two retrieval shapes:

- **Document path (Agentic RAG)**: a full pipeline for external material — 20+ format normalization, chunking, semantic / graph / hybrid indexing, result fusion and reranking;
- **Entry path (Native RAG)**: minimal semantic indexing for structured facts inside your system (skill registries, tool catalogs, config entries) — `Add` to index, `Search` to retrieve, no file landing required.

Both paths share the same foundation (local embedding inference + bbolt storage); they only diverge in the middle layer.

---

## Feature Highlights

| Feature | Description | Code |
| --- | --- | --- |
| **Dual retrieval paths** | Document pipeline and entry semantic indexing share one Embedder / storage foundation; indexers stay pure, fusion happens at query time | `native/` + `indexer/` |
| **Fully local** | Local ONNX embedding inference (quantized BGE, Chinese-capable), bbolt + SQLite storage — no external vector DB, no embedding API, works offline | `embedder/`, `store/` |
| **Semantic + graph dual line** | HyperIndexer orchestrates the semantic and relation lines, entity/relation writing to GraphStore, Region hierarchy for directories | `indexer/hyper.go`, `core/graph.go` |
| **Multi-format normalization** | PDF / DOCX / XLSX / PPTX / EPUB / EML / HTML / Markdown / YAML / images / code (tree-sitter), routed by mimetype sniffing | `document/` |
| **CLI & daemon coexistence** | bbolt locking: CLI opens read-only and fails fast while a daemon holds the write lock | `store/vector/govector/`, `store/meta/` |
| **Storage engineering** | Buffered batched writes (merged fsync), SQ8 quantization, HNSW / Flat selectable, payload filtering | `store/vector/govector/` |
| **Incremental & resumable** | mtime+size+hash change detection, per-chunk checkpoint resume, auto reprocess on change | `indexer/`, `store/meta/` |
| **LLM enhancement** | Auto title / summary / tags per chunk, schema-driven entity & relation extraction | `llm/` |
| **Interface segregation** | Small interfaces (IndexerCloser / Flusher / MetadataUpdater / GraphSearcher) consumed via type-assertion — never forced to implement what you don't need | `indexer/interfaces.go` |
| **Zero CGO** | Pure Go, painless cross-compilation | — |

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

### Entry semantic indexing (Native RAG)

For skill registries, tool catalogs, config entries — any "key-value entries + semantic lookup" scenario, no document pipeline involved:

```go
import "github.com/DotNetAge/gorag/v2/native"

// family = Collection, one db hosts many families; dbPath must be absolute
seg, err := native.NewSegIndexer("/abs/path/native.db", embedder)
if err != nil {
    log.Fatal(err)
}
defer seg.Close()

// every meta key-value pair becomes a sub-key vector; a hit on any dimension recalls the entry
_ = seg.Add(ctx, "skill", "websearch", map[string]string{
    "name":        "websearch",
    "title":       "Web Search",
    "description": "Search the web for content",
})

hits, _ := seg.Search(ctx, "skill", "web searching", 1)
// hits[0].Value == "websearch", hits[0].Meta holds the full metadata
```

See [native/README.md](./native/README.md).

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

![GoRAG Architecture](./docs/architecture.svg)

- **SemanticIndexer**: Chunk → vectorize → write to VectorStore
- **GraphIndexer**: Entities/relationships → write to GraphStore
- **HyperIndexer**: Orchestrates semantic + graph pipelines, supports Summarizer / Refiller injection

Alongside the document pipeline, the `native` package provides a parallel entry path: `SegIndexer` talks directly to the Embedder and govector (family = Collection), bypassing document / Chunker. Both paths share the storage foundation; fusion happens only at query time.

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
