# Observability (可观测性：监控与链路追踪)

`pkg/observability` 为 GoRAG 提供了工业级的监控指标（Metrics）和分布式链路追踪（Tracing）支持。

## 为什么可观测性对 RAG 至关重要？

RAG 系统通常涉及多个外部调用（如向量数据库、LLM API）。当用户反馈“回答太慢”或“回答不对”时，如果没有可观测性，排查将极其困难。
- **Metrics** 告诉你系统哪里慢、调用了多少次 Token。
- **Tracing** 告诉你具体的请求在哪个组件发生了延迟或错误。

## 1. 监控指标 (Metrics)

### 核心接口：`core.Metrics`
我们提供了一个符合 **Cloud Native** 标准的 **Prometheus** 实现。

### 导出指标清单
| 指标名称 | 类型 | 描述 | 标签 (Labels) |
| :--- | :--- | :--- | :--- |
| `gorag_search_duration_seconds` | Histogram | 向量搜索耗时分布 | `engine` |
| `gorag_indexing_duration_seconds` | Histogram | 文档解析与入库耗时 | `parser` |
| `gorag_embedding_count` | Counter | 累计生成的向量/Token 数量 | - |
| `gorag_search_error_count` | Counter | 搜索失败次数 | `engine` |

### 如何使用
```go
// 启动一个指标服务器在 :8080/metrics 供 Prometheus 抓取
metrics := observability.DefaultPrometheusMetrics(":8080")
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
```

## 3. 在 GoRAG 中一站式集成

在工业实践中，建议通过 `indexer` 的 Functional Options 一键开启这些能力：

```go
idx, _ := indexer.NewBuilder().
    // 开启 Prometheus 监控
    WithPrometheusMetrics(":8080").
    // 开启分布式追踪 (Jaeger/Zipkin)
    WithOpenTelemetryTracer(ctx, "jaeger-collector:4317", "gorag-service").
    Build()
```

> **注意**：如果不进行任何配置，系统将默认使用 `Noop` 实现，确保在不需要观测的场景下保持零性能开销。
