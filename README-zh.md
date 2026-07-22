<div align="center">
  <h1>GoRAG</h1>
  <p><b>本地知识库检索增强生成（RAG）工具包</b></p>

  [![Go Version](https://img.shields.io/badge/go-1.25%2B-blue.svg)](https://golang.org)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

GoRAG 是一个本地优先的 RAG（检索增强生成）工具包，提供 CLI 与 Go API 两套使用方式。支持语义向量检索、图结构检索以及两者的混合索引。

---

## 功能特性

- **语义检索**：基于向量嵌入的多维度语义匹配（Title / Summary / Content）
- **图检索**：基于知识图谱的多跳邻居查询，支持 Cypher 原生查询
- **混合索引**（Hyper）：语义线 + 图线双线编排，搜索结果可融合
- **LLM 增强**：自动为分片生成标题/摘要/标签，提取实体与关系
- **区域（Region）机制**：目录自动映射为 Region 节点，自动生成目录摘要 README
- **增量索引**：基于 mtime+size+hash 的文件变更检测，只索引变更内容
- **增量 LLM 处理**：按分片状态跟踪，断点续处理，内容变更自动重处理
- **文件进度跟踪**：SQLite 元数据存储，实时查看索引与 LLM 处理状态
- **多格式支持**：PDF / DOCX / HTML / EPUB / PPTX / Markdown / CSV / XLSX / JSON / YAML / 图片 / 代码等
- **零 CGO**：纯 Go 实现，无痛交叉编译

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

```
┌──────────────────────────────────────────┐
│              CLI (grag)                  │
│   cmd/main.go + cmd/info.go             │
└────────────────┬─────────────────────────┘
                 │
┌────────────────▼─────────────────────────┐
│         IndexingService (聚合根)          │
│  ┌─────────────────────────────────────┐  │
│  │  IndexerSvc · QuerySvc · GraphSvc  │  │
│  │  AdminSvc  · RegionSvc · LLMSvc    │  │
│  └─────────────────────────────────────┘  │
└────────────────┬─────────────────────────┘
                 │
    ┌────────────┼────────────┐
    ▼            ▼            ▼
 Semantic    Graph       Hyper(编排)
 Indexer    Indexer     ┌────┴────┐
                        ▼         ▼
                    Semantic   Graph
                    Indexer    Indexer
```

- **SemanticIndexer**：分块 → 向量化 → 写入 VectorStore
- **GraphIndexer**：实体/关系 → 写入 GraphStore
- **HyperIndexer**：编排语义线与关系线，支持 Summarizer / Refiller 注入

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
