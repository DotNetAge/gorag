# Retrieval Enhancement - 检索增强

## 什么是检索增强？

检索增强是一种在向量检索后对结果进行后处理的技术。它通过过滤、扩展、重排序等手段提升检索质量，让最终送入 LLM 的上下文更加精准和相关。

### 核心原理

```
用户查询
    ↓
向量检索（Retriever）
    ↓
检索结果 Chunks
    ↓
[增强处理层]
    ├── ContextPruner: LLM 评估相关性，按分数排序后选择 top chunks
    ├── FilterExtractor: 从查询中提取元数据过滤条件
    └── SentenceWindowExpander: 扩展 surrounding sentences
    ↓
增强后的 Chunks
    ↓
LLM 生成答案
```

### 与普通检索的区别

| 特性 | 普通检索 | 检索增强 |
|------|----------|----------|
| 结果质量 | 依赖向量相似度 | 额外语义理解 |
| 上下文长度 | 不可控 | 可按 token 限制 |
| 周边信息 | 无 | 自动扩展 |
| 元数据过滤 | 需手动指定 | 自动提取 |

---

## 有什么作用？

1. **ContextPruner（上下文裁剪器）**
   - 使用 LLM 评估每个 chunk 与查询的相关性
   - 按相关性分数排序，选择 token 限制内的 top chunks
   - 避免无关内容占用上下文窗口

2. **FilterExtractor（过滤器提取器）**
   - 从自然语言查询中自动提取元数据过滤条件
   - 支持年份、作者、文档类型、公司名称等
   - 将隐式过滤需求转化为显式 filter

3. **SentenceWindowExpander（句子窗口扩展器）**
   - 通过 `full_document` metadata 定位 chunk 在原文中的位置
   - 自动扩展 surrounding sentences（默认 2 句）
   - 保留更多上下文信息，提升回答连贯性

---

## 怎么工作的？

### 1. ContextPruner 流程

```
检索得到的 Chunks
    ↓
构造 Prompt（包含 query + 所有 chunks）
    ↓
调用 LLM 打分（0.0-1.0）
    ↓
按分数降序排序
    ↓
累加 token，直到达到 maxTokens 限制
    ↓
返回 top chunks
```

### 2. FilterExtractor 流程

```
用户查询 "2023年关于 AI 的论文"
    ↓
构造提取 Prompt
    ↓
调用 LLM 解析
    ↓
返回 {"year": 2023, "topic": "AI"}
    ↓
用于后续向量检索的 filters 参数
```

### 3. SentenceWindowExpander 流程

```
原始 Chunk（包含 full_document metadata）
    ↓
在 full_document 中定位 chunk 内容
    ↓
找到所在句子索引
    ↓
扩展 windowSize 句（前后各 N 句）
    ↓
返回扩展后的 chunk
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/enhancement/
├── context_pruner.go          # ContextPruner 实现
├── filter_extractor.go        # FilterExtractor 实现
└── sentence_window_expander.go # SentenceWindowExpander 实现
```

### 1. 统一接口（pkg/core/interfaces.go）

```go
type ResultEnhancer interface {
    Enhance(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}

type FilterExtractor interface {
    Extract(ctx context.Context, query *Query) (map[string]any, error)
}
```

### 2. ContextPruner

使用 LLM 评估 chunk 相关性并裁剪：

```go
pruner := enhancement.NewContextPruner(
    llmClient,
    enhancement.WithPrunerMaxTokens(2000),  // 最大 token 数
    enhancement.WithPrunerLogger(logger),   // 日志
    enhancement.WithPrunerCollector(metrics), // 指标收集
)

// 在 Pipeline 中使用
enhancedChunks, err := pruner.Enhance(ctx, query, chunks)
```

**评分 Prompt 示例**：
- 给每个 chunk 打 0.0-1.0 分
- 1.0 = 完全相关，直接包含答案
- 0.0 = 完全无关

### 3. FilterExtractor

从查询中提取元数据过滤条件：

```go
extractor := enhancement.NewFilterExtractor(llmClient)

// 提取过滤条件
filters, err := extractor.Extract(ctx, query)
// 例如: {"year": 2023, "author": "张三"}

// 用于向量检索
results, scores, err := vectorStore.Search(ctx, embedding, topK, filters)
```

**特性**：
- 支持自然语言查询中的隐式过滤条件
- 返回标准 map[string]any 格式
- 无过滤条件时返回空 map

### 4. SentenceWindowExpander

扩展 chunk 的上下文窗口：

```go
expander := enhancement.NewSentenceWindowExpander(
    enhancement.WithWindowSize(2),    // 前后各扩展 2 句
    enhancement.WithMaxChars(2000),   // 最大字符数
    enhancement.WithExpanderLogger(logger),
    enhancement.WithExpanderCollector(metrics),
)

// 扩展 chunks
expandedChunks, err := expander.Enhance(ctx, query, chunks)
```

**要求**：
- Chunk.Metadata 必须包含 `full_document` 字段（string 类型）
- 用于定位 chunk 在原文中的位置

---

## 如何与项目集成？

### 方式一：独立使用

```go
// ContextPruner
pruner := enhancement.NewContextPruner(llm)
chunks, _ := pruner.Enhance(ctx, query, retrievedChunks)

// FilterExtractor
extractor := enhancement.NewFilterExtractor(llm)
filters, _ := extractor.Extract(ctx, query)

// SentenceWindowExpander
expander := enhancement.NewSentenceWindowExpander(
    enhancement.WithWindowSize(2),
)
expandedChunks, _ := expander.Enhance(ctx, query, chunks)
```

### 方式二：Pipeline 组装（推荐）

```go
p := pipeline.New[*core.RetrievalContext]()

// 添加检索 Step
p.AddStep(vector.Search(store, embedder, opts))

// 添加增强 Steps
p.AddStep(enhancement.NewSentenceWindowExpander())
p.AddStep(enhancement.NewContextPruner(llm))

// 添加生成 Step
p.AddStep(generate.New(llm, logger))
```

### 方式三：自动 Pipeline

```go
// Advanced RAG 默认已集成增强组件
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
)

// 自定义增强配置
app, _ := gorag.AdvancedRAG(
    gorag.WithWorkDir("./data"),
    gorag.WithEnhancers(
        enhancement.NewSentenceWindowExpander(
            enhancement.WithWindowSize(3),
        ),
        enhancement.NewContextPruner(
            llm,
            enhancement.WithPrunerMaxTokens(1500),
        ),
    ),
)
```

---

## 适用于哪些场景？

### ✅ ContextPruner 适合

- **长查询场景**：需要从大量检索结果中筛选最相关的
- **Token 限制严格**：上下文窗口有限，需精打细算
- **检索结果噪声大**：向量检索返回很多不太相关的结果

### ✅ FilterExtractor 适合

- **结构化数据查询**：需要按年份、作者、类别过滤
- **多租户场景**：需按租户 ID 过滤
- **精准检索**：用户明确指定过滤条件

### ✅ SentenceWindowExpander 适合

- **回答需要上下文**：单个 chunk 不足以回答
- **文档切片场景**：按固定长度切片导致句子被截断
- **提升回答连贯性**：需要完整句子而非碎片

### ❌ 不适合使用

- **简单检索**：检索结果已经很精准
- **低延迟要求**：LLM 调用增加延迟
- **成本敏感**：增强组件需要额外 LLM 调用

---

## 配置推荐

| 场景 | 推荐配置 |
|------|----------|
| 通用 RAG | SentenceWindowExpander (window=2) |
| 长上下文 LLM | ContextPruner (maxTokens=4000) |
| 结构化知识库 | FilterExtractor + 元数据索引 |
| 高质量问答 | 全功能：Expander + Pruner |

```go
// 生产环境推荐：全功能增强
app, _ := gorag.AdvancedRAG(
    gorag.WithWorkDir("./data"),
    gorag.WithEnhancers(
        enhancement.NewSentenceWindowExpander(
            enhancement.WithWindowSize(2),
            enhancement.WithMaxChars(2000),
        ),
        enhancement.NewContextPruner(
            llm,
            enhancement.WithPrunerMaxTokens(2000),
        ),
    ),
)
```
