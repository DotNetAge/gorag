<div align="center">
  <h1>🦖 GoRAG</h1>
  <p><b>工业级、高性能、模块化 Go 语言 RAG 专家框架</b></p>
  
  [![Go Report Card](https://goreportcard.com/badge/github.com/DotNetAge/gorag)](https://goreportcard.com/report/github.com/DotNetAge/gorag)
  [![Go Reference](https://pkg.go.dev/badge/github.com/DotNetAge/gorag.svg)](https://pkg.go.dev/github.com/DotNetAge/gorag)
  [![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)
  [![Go Version](https://img.shields.io/badge/go-1.24%2B-blue.svg)](https://golang.org)
  
  [**English**](./README.md) | [**中文文档**](./README-zh.md)
</div>

---

**GoRAG** 是一个专为大规模 AI 工程设计的生产级 RAG（检索增强生成）框架。不同于复杂的“黑盒”框架，GoRAG 提供了一个 **透明、基于流水线 (Pipeline) 的架构**，将 Go 的原生并发优势与前沿的 RAG 模式深度结合。

从具备自动三元组提取能力的 **GraphRAG**，到具备自我修正能力的 **Agentic RAG**，GoRAG 致力于帮助开发者将 AI 应用从“实验脚本”快速推进到“生产服务”。

## ✨ 为什么选择 GoRAG？

- 🚀 **性能至上**: 内置高性能并发 Worker 和 `O(1)` 内存效率的流式解析器。轻松应对 TB 级知识库索引。
- 🏗️ **透明流水线架构**: 基于 `gochat/pkg/pipeline`。每个检索步骤都清晰、可追踪且可插拔，告别深层继承与黑盒逻辑。
- 🧠 **智能意图路由**: 根据用户意图，自动在向量检索 (Vector)、图检索 (Graph) 或全局汇总 (Global) 策略间进行最优调度。
- 🕸️ **进阶 GraphRAG**: 原生支持 **Neo4j**、**SQLite (Zero-CGO)** 和 **BoltDB**。内置 LLM 驱动的知识图谱自动构建引擎。
- 🔭 **全链路可观测性**: 针对所有核心 Retriever 和 Step 的分布式追踪。精准掌握每一毫秒时间与每一个 Token 的去向。
- 📊 **企业级评测协议**: 内置标准化的评测指标计算 (RAGAS 风格)，包括 **忠实度 (Faithfulness)**、**答案相关性** 和 **上下文精准度**。

---

## 🧰 RAG “专家”生态

GoRAG 不仅仅提供工具，更将**最佳实践**固化为标准组件：

| 检索策略 | 适用场景 | 核心特性 |
|----------|-------------|--------------|
| **Native RAG** | 标准语义搜索 | 纯向量、高速、低成本 |
| **Graph RAG** | 复杂关系推理 | 实体、三元组、多跳推理 |
| **Self-RAG** | 高精度要求场景 | 自我反思、幻觉检测 |
| **CRAG** | 处理模糊查询 | 质量评估、自动回退至 Web 搜索 |
| **Fusion RAG**| 多维度复杂问题 | 查询重写、RRF 排序融合 |
| **Smart Router**| 动态工作负载 | 基于意图的自动分流调度 |

---

## 🚀 快速开始：1 分钟构建工业级 RAG

GoRAG 针对不同的工业应用规模，提供了 **成对 (Paired)** 且经过优化的预设方案。无需手动组装复杂的 Pipeline，只需选择适合你的层级即可。

### 1. NativeRAG (最适合 AI Agent & 本地知识库)
*纯 Go 实现，零依赖 (SQLite + GoVector)。支持一键开启多模态能力。*

```go
import (
    "github.com/DotNetAge/gorag/pkg/indexer"
    "github.com/DotNetAge/gorag/pkg/retriever/native"
)

// 1. 索引文档 (Native 模式使用本地 SQLite/GoVector)
idx, _ := indexer.DefaultNativeIndexer("./data", false) 
idx.IndexDirectory(ctx, "./docs", true)

// 2. 与你的知识库对话
r, _ := native.DefaultNativeRetriever("./data", embedder, llm)
results, _ := r.Retrieve(ctx, []string{"什么是 GoRAG?"}, 5)
fmt.Println(results[0].Answer)
```

### 2. AdvancedRAG (企业级 / 高召回率)
*面向分布式架构 (Milvus/Qdrant)。内置 **RAG-Fusion** 最佳实践。*

```go
import (
    "github.com/DotNetAge/gorag/pkg/indexer"
    "github.com/DotNetAge/gorag/pkg/retriever/advanced"
)

// 1. 并发索引至生产级向量库 (如 Milvus)
idx, _ := indexer.DefaultAdvancedIndexer(milvusStore, sqliteDocStore)
idx.IndexDirectory(ctx, "./kb/enterprise", true)

// 2. 使用 RAG-Fusion + RRF 算法进行高精度检索
r := advanced.DefaultAdvancedRetriever(milvusStore, embedder, llm)
results, _ := r.Retrieve(ctx, []string{"对比架构 A 与架构 B 的优劣"}, 10)
```

### 3. GraphRAG (深度推理 / 复杂关系)
*自动构建知识图谱 (Neo4j)，支持向量与图谱的混合检索。*

```go
import (
    "github.com/DotNetAge/gorag/pkg/indexer"
    "github.com/DotNetAge/gorag/pkg/retriever/graph"
)

// 1. 索引并自动提取 (主体, 谓语, 客体) 三元组
idx, _ := indexer.DefaultGraphIndexer(vStore, docStore, neo4jStore, extractor)
idx.IndexFile(ctx, "financial_report.pdf")

// 2. 混合检索 (相似度检索 + 图谱多跳关联)
r := graph.DefaultGraphRetriever(vStore, neo4jStore, embedder, llm)
results, _ := r.Retrieve(ctx, []string{"实体 X 与实体 Y 之间有什么关联？"}, 5)
```

---

## 🔭 内置工业级可观测性

拒绝“盲飞”。GoRAG 原生支持 **Prometheus** 和 **OpenTelemetry**，助你实时监控生产环境中的 RAG 性能。

```go
idx, _ := indexer.DefaultAdvancedIndexer(vStore, dStore, 
    indexer.WithZapLogger("./logs/rag.log", 100, 30, 7, true), // 工业级日志
    indexer.WithPrometheusMetrics(":8080"),                   // 监控指标
    indexer.WithOpenTelemetryTracer(ctx, "jaeger:4317", "RAG"),// 链路追踪
)
```

---

## ⚡ 技术规范与标准
...

---

## ⚡ 技术规范与标准

- **Go 1.24+**: 拥抱最新的 Go 语言特性。
- **Zero-CGO SQLite**: 采用 `modernc.org/sqlite`，实现无痛苦的跨平台交叉编译。
- **整洁架构**: 严格分离接口 (`pkg/core`) 与具体实现。
- **模块化 Step**: 所有核心步骤（如 `hyde`, `rerank`, `fuse`, `prune`）均可在自定义 Pipeline 中复用。

## 🤝 参与贡献
我们致力于构建 Go 生态最强大的 AI 基础设施。无论是增加新的 `VectorStore` 驱动，还是改进 `Parser`，我们都欢迎您的 PR！
- 请参考 [贡献指南](CONTRIBUTING.md)。

## 📄 许可协议
GoRAG 采用 [MIT License](LICENSE) 开源协议。
