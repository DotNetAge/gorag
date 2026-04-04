# GraphRAG 检索器

GraphRAG 是一种基于知识图谱的检索增强生成技术，遵循 Microsoft GraphRAG 架构设计。它将文本数据转化为知识图谱，支持三种搜索模式：

| 搜索模式 | 适用场景 | 数据依赖 |
|---------|---------|---------|
| **Local** | 具体问题、实体关系查询 | 图遍历 |
| **Global** | 宏观主题、概览性问题 | 社区摘要 |
| **Hybrid** | 复杂问题、需要上下文 | 图 + 向量 + 社区 |

## 核心概念

### 图作为索引层

**关键设计**：图数据是原始文本的索引，而非独立存储。

```
原始文档 → 文本块 → 实体抽取 → 节点（绑定 SourceChunkIDs）
                      ↓
                关系抽取 → 边（绑定 SourceChunkIDs）
                      ↓
                社区检测 → 社区摘要
```

### 检索流程

```
用户查询 → 实体抽取 → [Local/Global/Hybrid] → 获取 SourceChunkIDs → 返回文本
```

## 搜索模式详解

### Local Search（局部搜索）

从查询中提取实体，在图中遍历获取相关节点和关系。

**适用场景**：
- "张三在哪个公司工作？"
- "项目 A 使用了哪些技术栈？"

```go
retriever, _ := graph.DefaultGraphRetriever(
    graph.WithGraphStore(graphStore),
    graph.WithSearchMode(core.SearchModeLocal),
    graph.WithDepth(2),  // 遍历深度
)
```

### Global Search（全局搜索）

基于社区摘要进行语义匹配，适用于宏观问题。

**适用场景**：
- "这份报告主要讲了哪些主题？"
- "项目中涉及的核心技术有哪些？"

```go
retriever, _ := graph.DefaultGraphRetriever(
    graph.WithGraphStore(graphStore),
    graph.WithEmbedder(embedder), // 必需
    graph.WithSearchMode(core.SearchModeGlobal),
)
```

### Hybrid Search（混合搜索）

融合 Local、Global 和 Vector 三路召回。

**适用场景**：
- "张三的项目涉及哪些技术？公司在这方面的投入如何？"

```go
retriever, _ := graph.DefaultGraphRetriever(
    graph.WithGraphStore(graphStore),
    graph.WithVectorStore(vectorStore),
    graph.WithEmbedder(embedder),
    graph.WithSearchMode(core.SearchModeHybrid),
)
```

## 实体提取策略

GraphRAG 支持多种实体提取策略，可根据可用资源和性能需求灵活选择：

| 策略 | 依赖 | 延迟 | 准确性 | 适用场景 |
|------|------|------|--------|---------|
| **LLM** | LLM 客户端 | 高 (500-2000ms) | 最高 (95%+) | 复杂查询、专业领域 |
| **Vector** | Embedder + GraphStore | 中 (20-100ms) | 中高 (70-80%) | 语义相似场景、实时系统 |
| **Keyword** | 无 | 低 (1-10ms) | 中等 (50-60%) | 高性能要求、简单查询 |

### 策略选择

```go
import (
    "github.com/DotNetAge/gorag/pkg/pattern"
    "github.com/DotNetAge/gorag/pkg/retriever/graph"
)

// 策略 1: LLM 提取（最准确，需要 LLM）
rag, _ := pattern.GraphRAG("my-graph",
    pattern.WithBGE("bge-small-zh-v1.5"),
    pattern.WithLLM(llmClient),
    pattern.WithExtractionStrategy(pattern.ExtractionStrategyLLM),
)

// 策略 2: 向量匹配（需要 Embedder，无需 LLM）
rag, _ := pattern.GraphRAG("my-graph",
    pattern.WithBGE("bge-small-zh-v1.5"),
    pattern.WithExtractionStrategy(pattern.ExtractionStrategyVector),
)

// 策略 3: 关键词提取（无依赖，最快）
rag, _ := pattern.GraphRAG("my-graph",
    pattern.WithBGE("bge-small-zh-v1.5"),
    pattern.WithExtractionStrategy(pattern.ExtractionStrategyKeyword),
)

// 策略 4: 自动选择（默认，根据可用资源自动选择最佳策略）
rag, _ := pattern.GraphRAG("my-graph",
    pattern.WithBGE("bge-small-zh-v1.5"),
    // 默认为 auto，无需显式设置
)
```

### 自动选择逻辑（默认）

当使用 `ExtractionStrategyAuto`（默认）时，系统按以下优先级选择策略：

1. **LLM 策略** - 如果配置了 LLM 客户端
2. **Vector 策略** - 如果配置了 Embedder 和 GraphStore
3. **Keyword 策略** - 作为兜底方案，无任何依赖

## 适用场景

- 涉及多实体、复杂关联关系的查询。
- 需要全局性概览而非局部性信息的场景。
- 依赖于领域内明确定义的语义关系的应用。

## 核心优化：DocStore 集成

通过集成 `DocStore`，GraphRAG 支持 **父子文档检索 (PDR)**：
*   **证据溯源**：当图谱找到两个实体的关系时，从 DocStore 召回提及该关系的原始长文本段落，增强回答的可信度。
*   **上下文补全**：防止由于分块过小（Chunking）导致的语义断层。

## 使用示例

```go
import (
    "github.com/DotNetAge/gorag/pkg/retriever/graph"
)

retriever := graph.NewRetriever(
    vectorStore,
    graphStore,
    docStore, // 传入 DocStore 以启用上下文增强
    embedder,
    llm,
    // 其他选项...
)
```

## 支持的图存储 (GraphStore)

GoRAG 提供多种图存储实现：

1.  **SQLite (嵌入式)**：推荐用于本地和轻量级应用。支持递归 CTE 遍历。
2.  **BoltDB (嵌入式)**：纯 Go 实现，极高性能的本地 K/V 索引。
3.  **Neo4j (工业级)**：支持大规模知识图谱，具备完整的 **Cypher** 查询能力。

### Neo4j 使用示例

```go
import "github.com/DotNetAge/gorag/pkg/indexing/store/neo4j"

graphStore, _ := neo4j.NewGraphStore("bolt://localhost:7687", "neo4j", "password", "neo4j")
```

## 使用 Cypher 模板进行深度推理

通过 `CypherStep`，你可以定义特定的图路径查询模板，帮助 LLM 发现文档中未直接描述的隐藏关系。

```go
import "github.com/DotNetAge/gorag/pkg/retriever/graph"

// 定义：查找某人入职公司的 CEO
const ceoFinderTemplate = `
    MATCH (p:Entity {id: $id})-[:WORKS_AT]->(c:Entity)<-[:CEO_OF]-(ceo:Entity)
    RETURN ceo.id as name
`

retriever := graph.NewRetriever(
    vectorStore,
    graphStore,
    docStore,
    embedder,
    llm,
    // 注入自定义 Cypher 推理步骤
    graph.WithCustomStep(graph.NewCypherStep(graphStore, ceoFinderTemplate, logger)),
)
```
