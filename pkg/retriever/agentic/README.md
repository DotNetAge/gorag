# AgenticRAG 检索器

AgenticRAG 代表了 RAG 的最高级形式，其中 LLM 扮演自主智能体（Agent）的角色，它根据自身推理决定何时进行搜索、如何搜索，以及何时获取了足够的信息来提供最终答案。

## 关键核心能力

1.  **自主工具使用 (Autonomous Tool Use)**：智能体可以在多种检索工具（如向量搜索、图谱搜索、网页搜索等）之间进行选择。
2.  **自我修正 (Self-Correction)**：如果初始检索结果不满意，智能体可以重新制定计划，尝试不同的检索策略。
3.  **递归任务分解 (Recursive Decomposition)**: 将复杂的查询拆分为多个子任务并逐步解决。
4.  **多轮推理回环**: 参与迭代式循环，不断精炼搜索和最终生成的答案。

## 支持的智能体工作流

- **CRAG (Corrective RAG)**: 评估检索质量，在内部数据不足时调用外部网页搜索。
- **Self-RAG**: 生成响应，并基于检索到的信息对其进行自我批判（Critique）和精炼（Refine）。
- **任务导向型智能体**: 为特定检索任务专门设计的智能体（例如代码搜索、医疗数据检索分析等）。

## 使用示例

```go
import (
    "github.com/DotNetAge/gorag/pkg/retriever/agentic"
)

retriever := agentic.NewRetriever(
    llm,
    tools, // 向量搜索工具、图谱搜索工具等
    // 其他选项...
)
```
