<div align="center">
  <h1>GoRAG</h1>
  <p><b>本地知识库检索增强生成（RAG）工具包</b></p>

  [![Go Version](https://img.shields.io/badge/go-1.25%2B-blue.svg)](https://golang.org)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

GoRAG 是一个本地优先的检索底座，提供 CLI 与 Go API 两套使用方式。它同时服务两种检索形态：

- **文档路径（Agentic RAG）**：面向外部资料的完整流水线——20 余种格式归一化、分块、语义 / 图 / 混合索引、结果融合重排；
- **条目路径（Native RAG）**：面向系统内事实（技能注册表、工具目录、配置项）的极简语义索引——`Add` 即索引，`Search` 即答案，无需落盘。

两条路径共享同一块基座（本地嵌入推理 + bbolt 存储），分叉只发生在中间层。

---

## 特色总览

| 特色 | 说明 | 代码落点 |
| --- | --- | --- |
| **双路径检索底座** | 文档流水线与条目语义索引共享同一 Embedder / 存储基座，索引端各自纯粹，查询端可融合 | `native/` + `indexer/` |
| **完全本地运行** | 嵌入用本地 ONNX 推理（BGE 量化模型，含中文），存储用 bbolt + SQLite，无外部向量库、无 embedding API 依赖，可离线 | `embedder/`、`store/` |
| **语义 + 图谱双线** | HyperIndexer 编排语义线与关系线，实体关系写入 GraphStore，Region 提供目录级层级 | `indexer/hyper.go`、`core/graph.go` |
| **多格式归一化** | PDF / DOCX / XLSX / PPTX / EPUB / EML / HTML / Markdown / YAML / 图片 / 代码（tree-sitter）等，按 mimetype 自动路由 | `document/` |
| **CLI 与守护进程共存** | bbolt 锁管理：daemon 持写锁时 CLI 只读打开并快速失败，互不阻塞 | `store/vector/govector/`、`store/meta/` |
| **存储层工程化** | 写入攒批（合并 fsync）、SQ8 量化、HNSW / Flat 可选、payload 过滤 | `store/vector/govector/` |
| **增量与断点** | mtime+size+hash 变更检测、按分片状态断点续处理、内容变更自动重处理 | `indexer/`、`store/meta/` |
| **LLM 增强** | 分片自动生成标题 / 摘要 / 标签，按 Schema 提取实体与关系 | `llm/` |
| **接口分离** | IndexerCloser / Flusher / MetadataUpdater / GraphSearcher 等小接口按需 type-assert，不强迫实现不需要的方法 | `indexer/interfaces.go` |
| **零 CGO** | 纯 Go 实现，无痛交叉编译 | — |

---

## 安装

### Homebrew

```bash
brew install DotNetAge/homebrew-gorag/gorag
```

### 从源码

```bash
go install github.com/DotNetAge/gorag/v2/cmd@latest
```

### 下载预编译二进制

从 [GitHub Releases](https://github.com/DotNetAge/gorag/releases) 下载对应平台的归档包。

---

## 快速开始

### 使用 CLI

```bash
# 1. 在项目目录中初始化 RAG 库（默认 hyper 混合索引）
cd my-project
grag init

# 2. 索引文件
grag index .

# 3. 语义检索
grag query "GoRAG 是什么"

# 4. 查看库状态
grag status

# 5. 可选：启用 LLM 摘要与实体抽取
export GORAG_API_KEY=sk-xxx
grag update . --llm-url https://api.openai.com/v1 --llm-model gpt-4o-mini

# 6. 目录级图探索
grag nodes ./src -n 2

# 7. 查看目录树
grag tree
```

### 使用 Go API

```go
import gorag "github.com/DotNetAge/gorag/v2"

// 创建服务
svc, err := gorag.NewRAGService("./my-project.rag")
if err != nil {
    log.Fatal(err)
}
defer svc.Stop()

// 索引目录
ctx := context.Background()
svc.IndexerSvc().Index(ctx, "./docs")

// 语义检索
hit, _ := svc.Querier().Query(ctx, "RAG 架构设计", "")

// 图探索
result, _ := svc.Explorer().Nodes(ctx, "./docs", 2)
```

### 条目语义索引（Native RAG）

适合技能注册表、工具目录、配置项这类「键值条目 + 语义检索」场景，不走文档流水线：

```go
import "github.com/DotNetAge/gorag/v2/native"

// 族 = Collection，一库多族；dbPath 必须为绝对路径
seg, err := native.NewSegIndexer("/abs/path/native.db", embedder)
if err != nil {
    log.Fatal(err)
}
defer seg.Close()

// meta 的每个键值对都会生成一条子键向量，任一维度命中即可召回
_ = seg.Add(ctx, "skill", "websearch", map[string]string{
    "name":        "websearch",
    "title":       "网页搜索",
    "description": "搜索网络上内容",
})

hits, _ := seg.Search(ctx, "skill", "搜索", 1)
// hits[0].Value == "websearch"，hits[0].Meta 为完整元数据
```

详见 [native/README.md](./native/README.md)。

---

## CLI 命令总览

| 命令                                | 说明                             |
| ----------------------------------- | -------------------------------- |
| `grag init [-t type]`               | 初始化 RAG 库                    |
| `grag index [path]`                 | 索引文件或目录                   |
| `grag update [path] [llm-options]`  | 增量更新 + LLM 增强              |
| `grag query <text> [-f] [-k]`       | 语义检索（多关键词用 `\|` 分隔） |
| `grag chunks [-p] [-s] [-f]`        | 分页列出 Chunk                   |
| `grag nodes [dir] [-n]`             | 目录级多跳图查询                 |
| `grag cypher <query>`               | 执行 Cypher 图查询               |
| `grag status [-s] [-f] [--summary]` | 查看索引与 LLM 处理进度          |
| `grag tree`                         | 查看目录树                       |
| `grag info`                         | 查看库信息                       |
| `grag doctor`                       | 诊断配置                         |
| `grag logs`                         | 查看日志                         |

---

## 核心概念

### .rag 库

每个 RAG 项目对应一个 `.rag` 目录，内部包含：

```
.rag/
├── config.yml          # 配置（索引器类型、模型路径、LLM 等）
├── meta.db             # SQLite 元数据存储（文档/分片状态）
├── vectors/            # 向量存储
├── graph/              # 图存储（仅 graph/hyper 索引器）
├── logs/               # 运行日志
└── model/              # Embedding 模型文件
```

### 索引器类型

| 类型       | 说明                      |
| ---------- | ------------------------- |
| `semantic` | 纯向量语义索引            |
| `graph`    | 纯图结构索引              |
| `hyper`    | 语义 + 图混合索引（默认） |

### Chunk

最小索引单元，包含 Title、Summary、Content、Tags、Source、RegionID 等属性。

### Region（区域）

目录级语义抽象，每个被索引的目录自动映射为一个 Region 节点：
- **RegionID**：目录绝对路径的 SHA256 哈希
- **README 自动生成**：索引阶段结束后为无 README.md 的目录自动生成摘要

---

## 架构设计

![GoRAG 架构](./docs/architecture.svg)

- **SemanticIndexer**：分块 → 向量化 → 写入 VectorStore
- **GraphIndexer**：实体/关系 → 写入 GraphStore
- **HyperIndexer**：编排语义线与关系线，支持 Summarizer / Refiller 注入

除文档流水线外，`native` 包提供与上述体系并行的条目路径：`SegIndexer` 直连 Embedder 与 govector（族 = Collection），不经过 document / Chunker。两条路径共享存储基座，融合只在查询端发生。

---

## LLM 增强

`grag update` 命令支持两阶段增量 LLM 处理：

1. **Summarizer**：为文档类分片生成 Title / Summary / Tags
2. **Refiller**：基于注册的 Schema 提取实体和关系，写入 GraphStore

需要配置 LLM：

```bash
grag update . \
  --llm-key <API_KEY> \
  --llm-url https://api.openai.com/v1 \
  --llm-model gpt-4o-mini \
  --schema ./schemas
```

环境变量：`GORAG_API_KEY`

---

## 文档

- [CLI 使用说明](./docs/v2/CLI.md) — 完整命令参考
- [服务层说明](./docs/v2/Services.md) — Go API 指南
- [Region 机制详解](./docs/v2/Region.md) — 区域抽象设计

---

## 许可证

GoRAG 基于 [MIT 许可证](./LICENSE) 发布。
