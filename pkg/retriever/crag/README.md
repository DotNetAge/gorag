# CRAG (Corrective RAG) 检索器

CRAG 引入了一个轻量级的检索评估器，用于判断所检索到的文档内容与查询的相关性。根据评估结果，CRAG 会在生成的流水线中决定采用不同的策略：
- **Correct (准确)**：直接使用检索内容生成回答。
- **Incorrect (错误)**：舍弃检索内容，并调用外部 Web 搜索工具获取新信息。
- **Ambiguous (模糊)**：同时结合内部检索和外部搜索的信息。

## 流水线结构

1.  **向量搜索**: 进行初步的内部文档召回。
2.  **检索评估 (Retrieval Evaluation)**: 使用 LLM 或评估模型打分。
3.  **自适应行动 (Action)**:
    - 触发 **Web Search** 进行补救。
    - 触发 **DocStore 集成** 进行本域内证据深度挖掘。
4.  **最终生成**: 结合多源信息生成最终答案。

## 核心优化：DocStore 集成

通过集成 `DocStore`，CRAG 具备更强的**本域补救**能力：
*   **上下文补全 (PDR)**：当检索评估为“模糊”或“部分相关”时，不仅仅是召回分块，CRAG 还可以通过 `DocStore` 召回该文档下的所有分块或对应的父文档原文，尝试在不依赖外部互联网的情况下提升准确率。
*   **证据回传**：在切换到外部搜索前，利用 `DocStore` 进行最后一次深度检查。

## 使用示例

```go
import (
    "github.com/DotNetAge/gorag/pkg/retriever/crag"
)

retriever := crag.NewRetriever(
    vectorStore,
    embedder,
    evaluator,
    llm,
    crag.WithWebSearcher(webSearcher),
    crag.WithDocStore(docStore), // 启用 DocStore 增强
)
```
