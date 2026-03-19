# AdvancedRAG 检索器集合

AdvancedRAG 并不是一个庞大且复杂的“万能检索器”，而是一个**针对不同应用场景和痛点提供“开箱即用”最佳实践的检索器集合**。

我们坚信：**专一才能做到极致**。因此，我们将复杂的高级 RAG 概念拆分为多个职责明确、标准化的管线实现。开发者可以根据具体的业务痛点，做到“丰俭由人、按需选用”。

## 设计哲学与原则

1. **场景导向 (Scenario-Driven)**：每一种 Retriever 都有明确的适用场景（例如：解决查询太短、解决召回率不足等）。
2. **开箱即用 (Out-of-the-box)**：隐藏复杂的内部编排，对外提供极其简单的实例化接口（`NewXxxRetriever`）。
3. **基于 Pipeline 构建**：所有标准的 AdvancedRAG 均由底层统一的 `pipeline` 机制组装而成。如果现有的标准实现无法满足需求，用户可以轻易地复制并定制自己的管线。
4. **多模态与原生 LLM**：所有实现原生支持纯文本与多模态切换，且直接依赖 `gochat/pkg/core.Client`。

## 标准 AdvancedRAG 实现列表

我们将提供以下几种标准的高级检索器：

### 1. HyDERetriever (假设性文档检索器)
*   **适用场景**：用户的提问通常极其简短（如关键词搜索），或者包含大量“只可意会”的隐式知识。
*   **工作原理**：先让 LLM 根据简短问题“瞎编”一个答案（假设性文档），然后用这个包含丰富语义的假设性文档去向量库检索真实的文档。

### 2. RewriteRetriever (查询重写检索器)
*   **适用场景**：用户的提问口语化严重、存在错别字、或者表意不清（例如口语化的客服场景）。
*   **工作原理**：利用 LLM 将原始查询重写为正规、清晰、利于向量检索的查询语句，然后再进行检索。

### 3. StepbackRetriever (后退一步检索器)
*   **适用场景**：用户直接询问极其底层的细节问题，而模型缺乏宏观背景知识导致回答错误。
*   **工作原理**：提取问题背后的抽象原理（后退一步），同时检索“宏观原理”和“微观细节”，结合两者给出答案。

### 4. FusionRetriever (多路融合检索器)
*   **适用场景**：复杂的综合性问题，单次查询召回的信息往往片面（单一视角的局限性）。
*   **工作原理**：利用 LLM 将原问题拆解/扩展为多个不同视角的子查询，并行检索后，使用 RRF (倒数排名融合) 算法对多路结果进行去重和重新打分。

### 5. MultiRouteRetriever / EnsembleRetriever (多路集成检索器)
*   **适用场景**：终极解决方案，同时包含向量检索（Dense）和关键词检索（Sparse/BM25）或图谱检索的混合场景。
*   **工作原理**：聚合多种底层检索器（如 Native + Graph），通过重排序（Reranker）统一对结果进行二次评分和截断。

## 使用示例

```go
import (
    "github.com/DotNetAge/gorag/pkg/retriever/advanced"
)

// 场景 A: 发现用户提问太短，使用 HyDE 检索器
hydeRetriever := advanced.NewHyDERetriever(vectorStore, embedder, llmClient)
res, err := hydeRetriever.Retrieve(ctx, "GoRAG 性能")

// 场景 B: 面对复杂问题，使用 RAG-Fusion 检索器
fusionRetriever := advanced.NewFusionRetriever(vectorStore, embedder, llmClient)
res, err := fusionRetriever.Retrieve(ctx, "对比微服务和单体架构在电商系统中的优劣势")
```

## 高阶定制

如果您需要的不仅仅是这些标准实现，您可以直接使用底层的 `pipeline` 机制，自由组合我们在 `pkg/steps` 中提供的各种原子步骤（如 `stepback`, `hyde`, `rerank`, `fuse` 等），打造属于您自己的专属 Retriever。
