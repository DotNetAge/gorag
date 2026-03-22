# Retrieval Service - 向量检索服务

## 什么是检索服务？

向量检索服务是 RAG（检索增强生成）系统的核心组件，负责根据用户查询从向量数据库中检索最相关的文档片段（Chunks）。它通过将文本转换为向量表示，利用向量相似度算法找到与查询语义最接近的内容。

### 核心原理

```
用户查询 "如何学习 Go 语言？"
        ↓
    [嵌入模型 Embedder]
        ↓
    计算查询向量
        ↓
    [向量数据库 VectorStore]
        ↓
    相似度搜索（余弦相似度）
        ↓
    返回 top-K 个最相关片段
```

### 与传统搜索的区别

| 特性 | 传统关键词搜索 | 向量检索服务 |
|------|----------------|--------------|
| 匹配方式 | 精确匹配（相同关键词） | 语义相似（相近意图） |
| 示例 | "Go 语言" ≠ "Golang" | "Go 语言" ≈ "Golang" |
| 理解能力 | 低（无法理解同义词） | 高（理解语义关系） |
| 适用场景 | 已知确切查询 | 多样化表达 |

---

## 有什么作用？

1. **精准检索**：从大规模文档库中找到与查询语义最相关的内容
2. **支持多查询**：并行处理多个查询，提高检索效率
3. **可扩展性**：通过集成不同的 Embedder 和 VectorStore 适应各种场景
4. **性能监控**：内置指标收集，支持系统 observability

---

## 怎么工作的？

### 检索流程

```
用户查询（单个或多个）
        ↓
    [Retriever 检索服务]
        ↓
    嵌入模型转换查询为向量
        ↓
    向量数据库执行相似度搜索
        ↓
    汇总结果并返回
```

### 单查询 vs 多查询

- **单查询**：直接执行嵌入+搜索，简单高效
- **多查询**：并行执行多个搜索，充分利用并发

```
单查询流程：
Query → Embed → Search → Result

多查询流程：
Query1 → Embed → Search ─┐
Query2 → Embed → Search ─┼→ 汇总 → Results
Query3 → Embed → Search ─┘
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/service/
├── retriever.go      # Retriever 接口实现
└── retriever_test.go # 单元测试
```

### 1. 统一接口（pkg/core/interfaces.go）

```go
type Retriever interface {
    Retrieve(ctx context.Context, queries []string, topK int) ([]*RetrievalResult, error)
}

type RetrievalResult struct {
    Chunks []*Chunk   // 检索到的文档片段
    Scores []float32  // 对应的相似度分数
}
```

### 2. Retriever 实现（service/retriever.go）

核心检索器，负责协调 Embedder 和 VectorStore：

```go
retriever := service.New(
    myVectorStore,   // 向量数据库
    myEmbedder,      // 嵌入模型
    service.WithTopK(10),        // 默认返回 10 条
    service.WithLogger(logger),  // 日志
    service.WithCollector(collector), // 指标收集
)
```

### 3. 核心组件接口

#### Embedder（嵌入模型）

```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
}
```

负责将文本转换为向量表示。常用的嵌入模型包括：
- OpenAI Embeddings
- BGE Embeddings
- M3 Embeddings

#### VectorStore（向量数据库）

```go
type VectorStore interface {
    Upsert(ctx context.Context, vectors []*Vector) error
    Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*Vector, []float32, error)
    Delete(ctx context.Context, id string) error
    Close(ctx context.Context) error
}
```

负责存储向量并执行相似度搜索。常用的向量数据库包括：
- Chroma
- Milvus
- Qdrant
- Weaviate

---

## 如何与项目集成？

### 方式一：创建独立检索服务

```go
// 创建检索服务
retriever := service.New(
    vectorStore,  // your VectorStore
    embedder,     // your Embedder
    service.WithTopK(10),
    service.WithLogger(logger),
)

// 执行检索
results, err := retriever.Retrieve(ctx, []string{"Go 语言教程"}, 5)
if err != nil {
    // 处理错误
}

// 处理结果
for _, result := range results {
    for i, chunk := range result.Chunks {
        fmt.Printf("Chunk: %s, Score: %.4f\n", chunk.Content, result.Scores[i])
    }
}
```

### 方式二：在 Pipeline 中使用

```go
// 创建 Pipeline
p := pipeline.New[*core.RetrievalContext]()

// 添加检索步骤
p.AddStep(vector.Search(store, embedder, opts))

// 添加重排序步骤（可选）
p.AddStep(rerank.NewCrossEncoder(llm))

// 添加生成步骤
p.AddStep(generate.New(llm, logger))
```

### 方式三：与语义缓存结合

```go
// 创建带缓存的检索器
wrappedRetriever := cache.NewRetrieverWithCache(
    originalRetriever,
    myCache,
    logger,
)
```

---

## 性能特性

### 并行检索

当传入多个查询时，Retriever 会并行执行检索：

```go
// 并行检索 3 个查询
results, _ := retriever.Retrieve(ctx, []string{
    "Go 语言基础",
    "Go 并发编程",
    "Go 性能优化",
}, 5)
```

### 性能指标

Retriever 内置以下指标收集：

| 指标 | 描述 |
|------|------|
| `retrieval.duration` | 检索耗时 |
| `retrieval.success` | 成功次数 |
| `retrieval.error` | 错误次数 |

---

## 适用于哪些场景？

### ✅ 适合使用

- **RAG 应用**：作为问答系统的检索组件
- **知识库问答**：从文档库中检索相关内容
- **语义搜索**：需要理解查询意图的搜索场景
- **多跳推理**：分解复杂问题为多个子查询

### ❌ 不适合使用

- **精确关键词搜索**：传统 BM25 等更合适
- **结构化查询**：SQL 类查询场景
- **实时性极高**：向量检索有固有延迟

---

## 配置推荐

| 场景 | 推荐配置 |
|------|----------|
| 简单问答 | topK=5, 单查询 |
| 复杂推理 | topK=10-20, 多查询 |
| 大规模检索 | topK=20+, 结合重排序 |
| 低延迟场景 | 减少 topK, 优化 Embedder |