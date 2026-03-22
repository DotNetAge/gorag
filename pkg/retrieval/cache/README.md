# Semantic Cache - 语义缓存

## 什么是语义缓存？

语义缓存是一种基于**向量相似度**的查询缓存技术。它通过比较用户查询与已缓存查询的语义相似度来判断是否命中缓存，而不是传统的精确匹配。

### 核心原理

```
用户查询 "如何学习 Go 语言？"
        ↓
    [语义缓存层]
        ↓
  计算查询向量
        ↓
  与缓存中的向量比较（余弦相似度）
        ↓
  相似度 ≥ 阈值（默认 0.98）→ 命中缓存，直接返回
  相似度 < 阈值 → 未命中，继续检索流程
```

### 与传统缓存的区别

| 特性 | 传统缓存 | 语义缓存 |
|------|----------|----------|
| 匹配方式 | 精确匹配（相同查询） | 语义相似（相近意图） |
| 示例 | "Go 语言" ≠ "Golang" | "Go 语言" ≈ "Golang" |
| 命中率 | 低（问法稍有不同就 miss） | 高（相同意图可命中） |

---

## 有什么作用？

1. **节省 LLM 调用**：命中缓存时无需调用 LLM 生成答案
2. **降低延迟**：缓存命中时响应时间从秒级降至毫秒级
3. **节省成本**：减少 token 消耗

---

## 怎么工作的？

### 1. 缓存查询流程

```
用户查询
    ↓
检查缓存（计算向量相似度）
    ↓
    ├── 命中 → 返回缓存的答案
    │
    └── 未命中
            ↓
        调用 Retriever 检索
            ↓
        调用 LLM 生成答案
            ↓
        将查询-答案存入缓存
            ↓
        返回答案
```

### 2. 缓存结构

每个缓存条目包含：
- **QueryText**: 原始查询文本
- **Embedding**: 查询的向量表示
- **Response**: LLM 生成的回答
- **CreatedAt**: 创建时间（用于 TTL）
- **LastHitAt**: 最后访问时间（用于 LRU）
- **HitCount**: 访问次数（用于 LFU）

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/cache/
├── memory.go      # InMemorySemanticCache 实现
├── bolt.go        # BoltSemanticCache 实现
└── retriever.go  # Retriever 缓存包装器
```

### 1. 统一接口（pkg/core/interfaces.go）

```go
type SemanticCache interface {
    CheckCache(ctx context.Context, query *Query) (*CacheResult, error)
    CacheResponse(ctx context.Context, query *Query, answer *Result) error
}

type CacheResult struct {
    Hit    bool
    Answer string
}
```

### 2. 内存缓存（InMemorySemanticCache）

纯内存实现，适合中小规模应用：

```go
cache := cache.NewInMemorySemanticCache(
    myEmbedder,
    cache.WithMaxSize(10000),           // 最大条目数
    cache.WithTTL(time.Hour),            // 过期时间
    cache.WithThreshold(0.98),          // 相似度阈值
    cache.WithEvictPolicy(cache.EvictLRU), // 淘汰策略
)
```

**特性**：
- **TTL**: 自动过期，防止过期数据占用内存
- **容量限制**: 超过 MaxSize 时自动淘汰
- **LRU/FIFO/LFU**: 多种淘汰策略可选

### 3. 持久化缓存（BoltSemanticCache）

基于 BoltDB（纯 Go，零依赖），适合需要持久化的场景：

```go
cache, err := cache.NewBoltSemanticCache(
    myEmbedder,
    cache.WithDBPath("./data/cache.db"),
)
```

**特性**：
- **持久化**: 重启后缓存不丢失
- **零外部依赖**: 使用纯 Go 的 BoltDB
- 与 govector 配置方式一致

### 4. Retriever 包装器

包装任意 Retriever，自动处理缓存逻辑：

```go
wrappedRetriever := cache.NewRetrieverWithCache(
    originalRetriever,
    myCache,
    logger,
)
```

---

## 如何与项目集成？

### 方式一：简单开关（推荐）

通过 `WithSemanticCache` 选项启用：

```go
// Advanced RAG 默认已开启语义缓存
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
)

// 显式指定缓存类型
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithSemanticCache(true, "memory"),  // 内存缓存
)

// 使用 Bolt 持久化缓存
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithSemanticCache(true, "bolt"),
)

// 禁用缓存
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithSemanticCache(false),
)
```

### 各 RAG 入口的缓存默认值

| RAG 入口 | 默认缓存 | 理由 |
|----------|---------|------|
| `DefaultNativeRAG` | 关闭 | 简单场景不需要 |
| `DefaultAdvancedRAG` | 开启 | 高性能场景，缓存价值高 |
| `DefaultAgenticRAG` | 开启 | 复杂推理，节省 token |
| `DefaultGraphRAG` | 关闭 | 图搜索缓存语义不明显 |

### 方式二：手动 Pipeline 组装（高级用法）

```go
// 创建 Pipeline
p := pipeline.New[*core.RetrievalContext]()

// 添加缓存检查 Step
p.AddStep(cache.Check(myCache, logger, metrics))

// 添加检索 Step
p.AddStep(vector.Search(store, embedder, opts))

// 添加缓存存储 Step
p.AddStep(cache.Store(myCache, logger, metrics))

// 添加生成 Step
p.AddStep(generate.New(llm, logger))
```

---

## 使用效果

### 性能对比（模拟数据）

| 场景 | 无缓存 | 有缓存 |
|------|--------|--------|
| 首次查询 | 500ms | 500ms |
| 重复查询（命中） | 500ms | **5ms** |
| 相近问题（语义命中） | 500ms | **5ms** |

### 命中条件

- **相似度阈值**: 默认 0.98（可通过 `WithThreshold` 调整）
- **查询相似**: "如何学习 Go" ≈ "Golang 入门教程"（高概率命中）

---

## 适用于哪些场景？

### ✅ 适合使用

- **FAQ 场景**: 大量重复或相近问题
- **产品文档**: 用户反复咨询相同功能
- **客服机器人**: 高频常见问题
- **企业内部知识库**: 重复性查询多

### ❌ 不适合使用

- **一次性问题**: 每个查询都不同
- **实时性要求高**: 缓存可能返回过期信息
- **数据敏感**: 不希望相似查询被关联

---

## 配置推荐

| 场景 | 推荐配置 |
|------|----------|
| 本地开发/测试 | 内存缓存，默认阈值 |
| 小规模生产 | Bolt 缓存，TTL=1小时 |
| 大规模高并发 | Bolt 缓存 + 大容量 |

```go
// 生产环境推荐
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
    gorag.WithSemanticCache(true, "bolt"),
)
```
