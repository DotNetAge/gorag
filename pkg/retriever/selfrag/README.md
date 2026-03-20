# Self-RAG 检索器

Self-RAG (Self-Reflective RAG) 是一种具备自我批判和精炼能力的检索架构。它不仅仅直接输出检索结果，还会评估检索分块的相关度、回答的忠实度以及有用性。如果生成的回答未通过质量检查，Self-RAG 会触发迭代式的重写或重新检索。

## 工作流：生成 -> 反思 -> 迭代

1.  **初步检索**: 从向量库获取 top-K 相关分块。
2.  **生成回答**: LLM 首次根据分块内容生成初步答案。
3.  **反思评估 (Self-Reflection)**:
    - 评估回答是否被分块支持（忠实度）。
    - 评估回答是否真的解决了用户问题（有用性）。
4.  **循环迭代**: 如果评估分数低于阈值，系统根据反馈（Feedback）进行精炼或重新检索。

## 核心优化：DocStore 集成

通过集成 `DocStore`，Self-RAG 在**精炼（Refinement）阶段**具有显著优势：
*   **召回全量父文档 (PDR)**：如果由于分块过碎导致回答在反思阶段被判断为“不忠实”，系统可以通过 `DocStore` 获取分块所在的整篇 **父文档** 内容进行重写，从而确保回答的语义完整性。
*   **上下文一致性验证**：通过对比 DocStore 中的原始结构，确认生成的回复没有断章取义。

## 使用示例

```go
import (
    "github.com/DotNetAge/gorag/pkg/retriever/selfrag"
)

retriever := selfrag.NewRetriever(
    vectorStore,
    embedder,
    evaluator,
    llm,
    selfrag.WithThreshold(0.8),
    selfrag.WithMaxRetries(3),
    selfrag.WithDocStore(docStore), // 启用 DocStore 增强
)
```
