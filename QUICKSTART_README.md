# QuickStart - 10 行代码拉起 RAG 服务

## 🚀 快速开始

### 最简单的使用方式

```go
package main

import (
	"github.com/DotNetAge/gorag/infra/indexer"
)

func main() {
	// 创建索引器并启动监控
	idx := indexer.DefaultIndexer(
		indexer.WithAllParsers(),           // 加载所有 20+ 解析器
		indexer.WithWatchDir("./data/docs"), // 监控目录
	)
	
	// 执行索引（Init 会自动调用）
	err := idx.Index()  // 增量索引，无需手动调用 Init
	if err != nil {
		log.Fatal(err)
	}
	// 或者 idx.IndexAll() // 全量索引
	// 或者 idx.Start() // 启动持续监控模式（阻塞式）
}
```

**就这么简单！** 不到 10 行代码，系统会自动：
- ✅ 读取 `./data/docs` 下的所有文件
- ✅ 根据文件扩展名自动选择合适的解析器
- ✅ 分块、向量化
- ✅ 存储到默认的向量数据库（govector）
- ✅ **自动初始化环境**（无需手动调用 Init）

---

## 📦 核心组件

### Parser 注册表

支持 22 种文件格式的自动解析：

```go
// 按类型选择解析器
parsers := types.Parsers(types.TEXT, types.MARKDOWN, types.GOCODE, types.PDF)

// 或者加载所有解析器（20+）
allParsers := types.AllParsers()

// 创建单个解析器
parser, err := types.NewParser(types.MARKDOWN)
```

**支持的 Parser 类型**：
- TEXT, MARKDOWN
- GOCODE, JAVACODE, PYCODE, TSCODE, JSCODE
- PDF, DOCX, EXCEL, CSV, JSON, XML, YAML
- LOG, HTML, IMAGE, EMAIL, PPT, DBSCHEMA

---

### VectorStore 工厂

```go
// 默认 govector（本地 SQLite）
store, err := vectorstore.DefaultVectorStore("./data/vectorstore/govector")

// 内存存储（测试用）
store := vectorstore.NewMemoryStore()

// Qdrant
store, err := vectorstore.NewQdrantStore("localhost:6334", "api-key", "gorag")

// Milvus
store, err := vectorstore.NewMilvusStore("localhost:19530", "user", "pass", "gorag")

// Pinecone
store, err := vectorstore.NewPineconeStore("api-key", "gcp-starter", "gorag")

// Weaviate
store, err := vectorstore.NewWeaviateStore("localhost:8080", "http", "api-key", "GoRAG")
```

---

### GraphStore 工厂

```go
// 默认 Neo4j
store, err := graphstore.DefaultGraphStore(
	"bolt://localhost:7687", 
	"neo4j", 
	"password",
)

// Neo4j
store, err := graphstore.NewNeo4JStore(
	"bolt://localhost:7687", 
	"neo4j", 
	"password",
)
```

---

### Embedding 模型

```go
// BGE 模型（自动下载）
embedder, err := embedding.WithBEG(
	"bge-small-zh-v1.5", 
	"./models/bge-small-zh-v1.5",
)

// BERT 模型（自动下载）
embedder, err := embedding.WithBERT(
	"all-mpnet-base-v2", 
	"./models/all-mpnet-base-v2",
)
```

**特性**：
- ✅ 自动检查模型是否存在
- ✅ 不存在时自动下载到 `~/.embedding/` 目录
- ✅ 复用 gochat 的 Downloader 机制

---

### Ollama 默认客户端

```go
// 默认使用 qwen3.5:0.8b 模型
client, err := ollama.DefaultOllamaClient()
```

---

### SemanticChunker 工厂

```go
// 默认语义分块器
chunker := semantic.DefaultSemanticChunker(embedder)

// 自定义参数
chunker := semantic.NewSemanticChunker(
	embedder, 
	100,   // min chunk size
	1000,  // max chunk size
	0.85,  // similarity threshold
)
```

---

### GraphExtractor 工厂

```go
// 默认图提取器
extractor := graph.DefaultGraphExtractor(llmClient)
```

---

## ⚙️ DefaultIndexer 配置选项

### 基础配置

```go
idx := indexer.DefaultIndexer(
	indexer.WithAllParsers(),              // 加载所有解析器
	indexer.WithWatchDir("./data/docs"),   // 监控目录
)
```

### 自定义解析器

```go
// 方式 1: 自定义解析器列表
myParser := NewMyCustomParser()
idx := indexer.DefaultIndexer(
	indexer.WithParsers(myParser, myParser2),
	indexer.WithWatchDir("./data/docs"),
)

// 方式 2: 指定特定类型的解析器
specificParsers := types.Parsers(
	types.TEXT, types.LOG, types.MARKDOWN, types.GOCODE,
)
idx := indexer.DefaultIndexer(
	indexer.WithParsers(specificParsers...),
	indexer.WithWatchDir("./data/docs"),
)
```

### 更改 VectorStore

```go
customStore, err := vectorstore.DefaultVectorStore(
	"./data/vectorstore/custom",
)

idx := indexer.DefaultIndexer(
	indexer.WithAllParsers(),
	indexer.WithWatchDir("./data/docs"),
	indexer.WithStore(customStore),
)
```

### 启用 GraphRAG

```go
graphStore, err := graphstore.DefaultGraphStore(
	"bolt://localhost:7687", 
	"neo4j", 
	"password",
)

idx := indexer.DefaultIndexer(
	indexer.WithAllParsers(),
	indexer.WithWatchDir("./data/docs"),
	indexer.WithGraph(graphStore),
)
```

### 自定义 Embedding 模型

```go
customEmbedder, err := embedding.WithBEG(
	"bge-small-zh-v1.5", 
	"./models/bge-small-zh-v1.5",
)

idx := indexer.DefaultIndexer(
	indexer.WithAllParsers(),
	indexer.WithWatchDir("./data/docs"),
	indexer.WithEmbedding(customEmbedder),
)
```

### 其他配置

```go
idx := indexer.DefaultIndexer(
	indexer.WithAllParsers(),
	indexer.WithWatchDir("./data/docs"),
	indexer.WithLLM(client),               // 自定义 LLM
	indexer.WithMetrics(myMetrics),        // 可观测性
	indexer.WithLogger(customLogger),      // 自定义日志
	indexer.WithContainer(di.New()),       // DI 容器注入
)
```

---

## 🔧 索引操作

```go
// 增量索引（只索引新增文件）
err := idx.Index()

// 全量索引（重新索引所有文件）
err := idx.IndexAll()

// 启动持续监控模式（阻塞式）
err := idx.Start()

// 索引指定目录（可指定是否递归）
err := idx.IndexDirectory(ctx, "./data/docs", true)

// 索引单个文件
err := idx.IndexFile(ctx, "./data/docs/file.pdf")

// 获取指标数据
metrics := idx.GetMetrics()
```

---

## 📁 项目结构

```
gorag/
├── infra/
│   ├── indexer/
│   │   ├── default_indexer.go       # DefaultIndexer 核心实现
│   │   ├── watcher.go               # 文件监控器（fsnotify）
│   │   └── default_indexer_test.go  # 单元测试
│   ├── indexing/
│   │   └── state.go                 # Pipeline State 定义
│   ├── steps/                       # Pipeline Steps（三阶段）
│   │   ├── parse_step.go            # 解析步骤
│   │   ├── chunk_step.go            # 分块步骤
│   │   ├── embed_step.go            # 嵌入步骤
│   │   └── store_step.go            # 存储步骤
│   ├── parser/
│   │   └── config/types/
│   │       ├── types.go             # ParserType iota 枚举
│   │       ├── registry.go          # Parser 注册表
│   │       └── registry_test.go     # 单元测试
│   ├── vectorstore/
│   │   ├── factory.go               # VectorStore 工厂
│   │   └── factory_test.go          # 单元测试
│   ├── graphstore/
│   │   ├── factory.go               # GraphStore 工厂
│   │   └── factory_test.go          # 单元测试
│   └── chunker/semantic/
│       └── factory.go               # SemanticChunker 工厂
├── pkg/
│   ├── domain/abstraction/
│   │   ├── metrics.go               # Metrics 接口
│   │   └── vectorstore.go           # VectorStore 接口
│   ├── usecase/dataprep/
│   │   └── indexer.go               # Indexer 接口定义
│   └── di/                          # 依赖注入容器
└── .docs/
    └── QUICKSTART_IMPLEMENTATION_SUMMARY.md  # 实施总结

gochat/
├── pkg/
│   ├── embedding/
│   │   ├── factory.go               # Embedding 工厂（含 WithBEG/WithBERT）
│   │   └── downloader.go            # 模型下载器
│   └── client/ollama/
│       └── client.go                # Ollama 客户端（含 DefaultOllamaClient）
└── examples/
    └── quickstart/
        └── main.go                  # QuickStart 示例代码
```

---

## 🎯 Spec 对照表

| Spec 要求 | 实现状态 | 文件位置 |
|----------|---------|---------|
| WithAllParsers() | ✅ | `default_indexer.go` |
| WithParsers(parsers...) | ✅ | `default_indexer.go` |
| types.Parsers(...) | ✅ | `types/registry.go` |
| WithWatchDir(dirs...) | ✅ | `default_indexer.go` |
| Index() / IndexAll() | ✅ | `default_indexer.go` |
| Start() 阻塞式监控 | ✅ | `default_indexer.go` + `watcher.go` |
| WithStore(store) | ✅ | `default_indexer.go` |
| DefaultVectorStore(path) | ✅ | `vectorstore/factory.go` |
| QdrantStore/MilvusStore等 | ✅ | `vectorstore/factory.go` |
| WithGraph(store) | ✅ | `default_indexer.go` |
| DefaultGraphStore(...) | ✅ | `graphstore/factory.go` |
| WithEmbedding(provider) | ✅ | `default_indexer.go` |
| WithBEG/WithBERT | ✅ | `gochat/embedding/factory.go` |
| WithLLM(client) | ✅ | `default_indexer.go` |
| DefaultOllamaClient() | ✅ | `gochat/ollama/client.go` |
| WithMetrics(metrics) | ✅ | `default_indexer.go` |
| Metrics 接口 | ✅ | `abstraction/metrics.go` |
| WithLogger(logger) | ✅ | `default_indexer.go` |
| WithContainer(di) | ✅ | `default_indexer.go` |
| Init() 返回 error | ✅ | `default_indexer.go` |

---

## 📊 统计信息

- **Parser 类型**: 22 种文件格式
- **工厂函数**: 20+ 个
- **VectorStore 支持**: 6 种向量数据库
- **GraphStore 支持**: 5 种图数据库
- **单元测试覆盖率**: 持续完善中

---

## 🚀 下一步

运行 QuickStart 示例：

```bash
cd gorag-examples/quickstart
go run main.go
```

查看完整示例代码：[gorag-examples/quickstart/main.go](../gorag-examples/quickstart/main.go)
