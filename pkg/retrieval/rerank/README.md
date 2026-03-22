# Cross-Encoder Rerank - 交叉编码器重排序

## 什么是交叉编码器重排序？

交叉编码器重排序是一种基于**深度语义交互**的结果重排序技术。它通过将查询和每个 Chunk 同时输入到 LLM 中，评估每个 Chunk 与查询之间的相关性，并按照相关性得分重新排序。

### 核心原理

```
用户查询 "如何学习 Go 语言？"
        ↓
    [检索阶段]
        ↓
  返回初始排序结果（可能不准确）
        ↓
    [重排序阶段]
        ↓
  将查询和每个 Chunk 配对输入 LLM
        ↓
  LLM 评估每个 Chunk 的相关性得分
        ↓
  按得分降序排列，返回最终结果
```

### 与向量检索的区别

| 特性 | 向量检索 | 交叉编码器重排序 |
|------|----------|-----------------|
| 计算方式 | 单次向量相似度 | 深度语义交互 |
| 查询-Chunk 关系 | 独立计算 | 配对评估 |
| 精度 | 较高 | 最高 |
| 延迟 | 低（毫秒级） | 较高（依赖 LLM） |

---

## 有什么作用？

1. **提升检索精度**：深度理解查询意图，返回更相关的结果
2. **优化排序质量**：将最相关的 Chunk 排在最前面
3. **弥补向量检索不足**：向量相似度不完美时，重排序作为补充

---

## 怎么工作的？

### 1. 重排序流程

```
初始检索结果（向量相似度排序）
    ↓
构建 Prompt（包含查询和所有 Chunk）
    ↓
调用 LLM 评估每个 Chunk 的相关性（0.0-1.0）
    ↓
按得分降序排列
    ↓
返回 Top-K 个 Chunk
```

### 2. 评分标准

LLM 根据以下标准对每个 Chunk 评分：

| 得分 | 含义 | 描述 |
|------|------|------|
| 1.0 | 完美匹配 | 完全回答用户查询 |
| 0.7-0.9 | 高度相关 | 很好地相关 |
| 0.4-0.6 | 部分相关 | 有一些帮助 |
| 0.1-0.3 | 轻微相关 | 勉强相关 |
| 0.0 | 完全无关 | 不相关 |

### 3. 输出格式

LLM 返回 JSON 格式的相关性得分数组：

```json
[0.9, 0.3, 0.7, 0.1]
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/rerank/
├── reranker.go      # CrossEncoder 实现
├── core.go          # Reranker 接口定义（已废弃）
└── rerank_test.go   # 单元测试
```

### 1. 统一接口（pkg/core/interfaces.go）

```go
type Reranker interface {
    Rerank(ctx context.Context, query *Query, chunks []*Chunk) ([]*Chunk, error)
}
```

### 2. CrossEncoder 实现

基于 LLM 的 Cross-Encoder 重排序：

```go
reranker := rerank.NewCrossEncoder(
    myLLM,
    rerank.WithRerankTopK(10),           // 返回 Top-K 结果
    rerank.WithRerankLogger(logger),     // 日志记录
    rerank.WithRerankCollector(metrics), // 指标收集
)
```

**特性**：
- **LLM 驱动**：使用 LLM 深度理解语义
- **可配置 Top-K**：控制返回结果数量
- **可观测性**：内置日志和指标收集

---

## 如何与项目集成？

### 方式一：简单开关（推荐）

通过 `WithReranker` 选项启用：

```go
// Advanced RAG 默认已开启重排序
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
)

// 显式启用重排序
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithReranker(true),
)

// 禁用重排序
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithReranker(false),
)
```

### 方式二：手动 Pipeline 组装（高级用法）

```go
// 创建 Pipeline
p := pipeline.New[*core.RetrievalContext]()

// 添加检索 Step
p.AddStep(vector.Search(store, embedder, opts))

// 添加重排序 Step
p.AddStep(rerank.NewCrossEncoder(llm, opts...))

// 添加生成 Step
p.AddStep(generate.New(llm, logger))
```

---

## 使用效果

### 性能对比（模拟数据）

| 场景 | 仅检索 | 检索+重排序 |
|------|--------|-------------|
| 首次查询 | 500ms | 800ms |
| 重复查询 | 500ms | 800ms |
| 排序质量 | 中等 | **高** |

### 重排序效果

- **查询**："Go 语言如何处理并发？"
- **Chunk 1**："Go 语言使用 goroutine 实现并发" → 得分 0.9
- **Chunk 2**："Python 的多线程机制" → 得分 0.1
- **Chunk 3**："Go 语言的垃圾回收机制" → 得分 0.5

---

## 适用于哪些场景？

### ✅ 适合使用

- **高精度需求**：对检索结果质量要求高
- **语义复杂查询**：简单向量相似度无法准确匹配
- **竞品对比分析**：需要深度理解意图
- **技术文档问答**：需要准确的技术细节

### ❌ 不适合使用

- **低延迟要求**：每次重排序都需要 LLM 调用
- **大规模检索**：Chunk 数量非常多时成本高
- **简单 FAQ**：向量检索已足够准确
- **成本敏感**：LLM 调用成本较高

---

## 配置推荐

| 场景 | 推荐配置 |
|------|----------|
| 高质量问答 | 开启重排序，TopK=10 |
| 简单检索 | 关闭重排序 |
| 混合检索 | 向量检索 + 重排序 |

```go
// 高质量问答推荐
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
    gorag.WithReranker(true),
)
```
