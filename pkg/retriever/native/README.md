# NativeRAG 检索器

NativeRAG（基础 RAG）是最简单的 RAG 流水线形式。它遵循直接的“检索-生成”模式，不包含复杂的查询转换或重排序过程。

## 流水线结构

NativeRAG 流水线由以下步骤组成：

1.  **向量搜索 (Vector Search)**：根据用户查询的嵌入向量，在向量数据库中搜索相关的文档分块。
2.  **提示词生成 (Prompt Generation)**：将检索到的上下文与用户查询组合成最终的提示词。
3.  **生成 (Generation)**：将提示词发送给 LLM，生成最终答案。

## 适用场景

- 对延迟要求极高的实时问答。
- 知识库内容简单、查询意图明确的场景。
- 作为高级 RAG 方案的基准线（Baseline）。

## 使用示例

```go
import (
    "github.com/DotNetAge/gorag/pkg/retriever/native"
)

retriever := native.NewRetriever(
    vectorStore,
    embedder,
    llm,
    // 其他选项...
)

response, err := retriever.Retrieve(ctx, "什么是 GoRAG？")
```
