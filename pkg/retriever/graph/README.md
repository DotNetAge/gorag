# GraphRAG 检索器

GraphRAG 将基于向量的检索与知识图谱的结构化信息相结合，能够提供更具上下文准确性和全面性的答案，尤其适用于复杂或涉及多条关系路径的查询。

## 流水线结构

GraphRAG 的工作流程通常包含以下步骤：

1.  **实体提取 (Entity Extraction)**：识别用户查询中的关键实体。
2.  **向量搜索**: 搜索相关的文档分块（同传统 RAG）。
3.  **图谱遍历 (Graph Traversal)**: 在 GraphStore 中搜索相关实体、关系以及子图结构。
4.  **上下文增强 (DocStore Enrichment)**: 利用 DocStore 根据检索到的分块 ID 或实体关联，召回完整的 **父文档 (Parent Document)** 或原文证据，提供更丰富的背景。
5.  **生成**: 利用合并后的信息生成更全面的响应。

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
