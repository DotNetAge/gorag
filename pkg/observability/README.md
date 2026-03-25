# Observability (可观测性：监控与链路追踪)

`pkg/observability` 为 GoRAG 提供了工业级的监控指标（Metrics）和分布式链路追踪（Tracing）支持。

## 为什么可观测性对 RAG 至关重要？

RAG 系统通常涉及多个外部调用（如向量数据库、LLM API）。当用户反馈“回答太慢”或“回答不对”时，如果没有可观测性，排查将极其困难。
- **Metrics** 告诉你系统哪里慢、调用了多少次 Token。
- **Tracing** 告诉你具体的请求在哪个组件发生了延迟或错误。
- **Quality Analysis** 自动评估 RAG 回答的忠诚度（Faithfulness）与相关性（Relevance）。

## 1. 监控指标 (Metrics)

### 核心接口：`core.Metrics`
我们提供了一个符合 **Cloud Native** 标准的 **Prometheus** 实现。

### 导出指标清单
| 指标名称 | 类型 | 描述 | 标签 (Labels) |
| :--- | :--- | :--- | :--- |
| `gorag_queries_total` | Counter | **QPS 监控**：累计接收到的查询总数 | `engine` |
| `gorag_search_duration_seconds` | Histogram | **性能监控**：向量搜索/生成耗时分布 | `engine` |
| `gorag_qa_quality_score` | Histogram | **质量评估**：RAGAS 指标评分 (0.0-1.0) | `metric` (faithfulness, relevance) |
| `gorag_llm_tokens_total` | Counter | **成本监控**：累计消耗的 Token 数量 | `model`, `type` |
| `gorag_search_result_count` | Counter | **召回监控**：每次搜索命中的 Chunk 数量 | `engine` |
| `gorag_indexing_duration_seconds` | Histogram | **工程监控**：文档解析与入库耗时 | `parser` |
| `gorag_embedding_count` | Counter | 累计生成的向量数量 | - |

### 如何使用
```go
// 启动一个指标服务器在 :8080/metrics 供 Prometheus 抓取
metrics := observability.DefaultPrometheusMetrics(":8080")

// 在构建 RAG 时注入
rag, _ := gorag.DefaultNativeRAG(
    gorag.WithMetrics(metrics),
)
```

## 2. 链路追踪 (Tracing)

### 核心接口：`Tracer` & `Span`
我们提供了一个符合工业标准 **OpenTelemetry (OTel)** 的实现。

### 功能特点
- **分布式上下文传递**：支持将 Trace ID 从业务上层传递到最底层的向量数据库调用。
- **导出器支持**：支持导出到 Jaeger, Zipkin, Honeycomb 或任何 OTel 兼容的后端。
- **自动语义映射**：将 RAG 内部的“分块”、“召回”、“重排序”自动映射为 Trace 中的不同阶段。

### 如何使用
```go
// 初始化 OTel 追踪并导出到本地 Jaeger 代理
tracer, _ := observability.DefaultOpenTelemetryTracer(ctx, "localhost:4317", "MyRAGApp")

// 在构建 RAG 时注入
rag, _ := gorag.DefaultNativeRAG(
    gorag.WithTracer(tracer),
)
```

---

## 我们是如何计算质量指标的？

GoRAG 采用 **异步评估机制**：
1. 当你调用 `rag.Search` 获得答案后。
2. 如果你在容器中注册了 `RAGEvaluator` 接口。
3. 框架会启动一个后台 Goroutine，利用 LLM 作为 Judge 自动计算 **Faithfulness**（是否忠实于原文，无幻觉）和 **Relevance**（是否精准回答了用户问题）。
4. 结果将实时上报到 Prometheus 的 `gorag_qa_quality_score`。

> **提示**：建议配合 Grafana 面板监控 `gorag_qa_quality_score` 的 P95 值，确保你的知识库质量始终在线。
