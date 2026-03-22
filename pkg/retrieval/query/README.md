# Query Processing - 查询处理

## 什么是查询处理？

查询处理是 RAG 系统的前处理模块，负责对用户查询进行一系列智能分析与转换，以提高检索质量和准确性。

### 核心组件

| 组件 | 文件 | 功能 |
|------|------|------|
| IntentRouter | classifier.go | 将查询分类为 5 种意图 |
| Decomposer | decomposer.go | 将复杂问题分解为子问题 |
| EntityExtractor | extractor.go | 从查询中提取关键实体 |
| HyDE | hyde.go | 生成假设性文档 |
| Rewriter | rewriter.go | 重写和优化查询 |
| StepBack | stepback.go | 生成抽象背景问题 |

### 核心原理

```
用户查询 "How does the semantic cache improve RAG performance?"
        ↓
    [查询处理管道]
        ↓
    ┌─ IntentRouter: domain_specific（领域特定查询）
    ├─ EntityExtractor: [semantic cache, RAG]
    ├─ Decomposer: 拆分为子问题
    ├─ Rewriter: 优化查询表达
    └─ StepBack: 生成背景问题
        ↓
    [增强后的查询用于向量检索]
```

---

## 有什么作用？

### 1. IntentRouter（意图路由）

- **智能分类**：将查询分为 5 种意图
  - `chat`: 闲聊问答
  - `domain_specific`: 领域知识检索
  - `relational`: 关系型查询（图检索）
  - `global`: 全局总结
  - `fact_check`: 事实核查

- **动态路由**：根据意图选择最适合的检索策略

### 2. Decomposer（问题分解）

- **复杂问题拆分**：将多跳问题分解为 2-5 个独立子问题
- **并行检索**：子问题可并行检索，提高效率
- **完整覆盖**：确保复杂查询的各个方面都被覆盖

### 3. EntityExtractor（实体提取）

- **关键实体识别**：提取查询中的人名、地点、概念等
- **关系发现**：为图检索提供节点信息
- **检索增强**：实体信息可用于精确匹配

### 4. HyDE（假设性文档）

- **答案预生成**：生成假设性的高质量答案
- **检索增强**：用假设答案去找相似文档
- **克服语义鸿沟**：假设答案可能包含原查询未明确表达的关键词

### 5. Rewriter（查询重写）

- **消除歧义**：将模糊表述改为清晰描述
- **去除冗余**：移除无意义的闲聊词汇
- **指代消解**：将代词解析为具体实体

### 6. StepBack（回退问题）

- **抽象提升**：生成更高层次的背景问题
- **双重检索**：既检索原问题，也检索背景问题
- **防止遗漏**：确保重要基础概念被检索到

---

## 怎么工作的？

### 1. 查询处理流程

```
用户查询
    ↓
IntentRouter 分类
    ↓
    ├── chat/fact_check → 直接生成答案（跳过检索）
    └── domain_specific/relational/global
            ↓
        EntityExtractor 提取实体
            ↓
        Decomposer 分解问题（可选）
            ↓
        Rewriter 重写查询（可选）
            ↓
        StepBack 生成背景问题（可选）
            ↓
        HyDE 生成假设文档（可选）
            ↓
        多路检索
            ↓
        答案生成
```

### 2. 组件协作模式

```
┌─────────────┐     ┌─────────────┐
│IntentRouter │────▶│  Router     │
└─────────────┘     │  Decision   │
                    └──────┬──────┘
                           │
         ┌─────────────────┼─────────────────┐
         ▼                 ▼                 ▼
┌─────────────┐     ┌─────────────┐   ┌─────────────┐
│ Decomposer  │     │   HyDE      │   │  Rewriter   │
└─────────────┘     └─────────────┘   └─────────────┘
         │                 │                 │
         └─────────────────┼─────────────────┘
                           ▼
                    ┌─────────────┐
                    │   检索增强   │
                    └─────────────┘
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/query/
├── classifier.go    # IntentRouter 意图路由
├── decomposer.go    # Decomposer 问题分解
├── extractor.go     # EntityExtractor 实体提取
├── hyde.go          # HyDE 假设性文档
├── rewriter.go      # Rewriter 查询重写
└── stepback.go      # StepBack 回退问题
```

### 1. IntentRouter（classifier.go）

```go
router := query.NewIntentRouter(llm,
    query.WithDefaultIntent(core.IntentDomainSpecific),
    query.WithMinConfidence(0.7),
)

// 分类查询
result, err := router.Classify(ctx, core.NewQuery("1", "What is RAG?", nil))
// result.Intent: domain_specific
// result.Confidence: 0.95
```

### 2. Decomposer（decomposer.go）

```go
decomposer := query.NewDecomposer(llm,
    query.WithMaxSubQueries(5),
)

// 分解复杂问题
result, err := decomposer.Decompose(ctx, core.NewQuery("1", complexQuery, nil))
// result.SubQueries: ["子问题1", "子问题2", ...]
// result.IsComplex: true
```

### 3. EntityExtractor（extractor.go）

```go
extractor := query.NewEntityExtractor(llm)

// 提取实体
result, err := extractor.Extract(ctx, core.NewQuery("1", "Who is the CEO of Apple?", nil))
// result.Entities: ["Apple", "CEO"]
```

### 4. HyDE（hyde.go）

```go
hyde := query.NewHyDE(llm)

// 生成假设文档
doc, err := hyde.Generate(ctx, core.NewQuery("1", "What is semantic cache?", nil))
// doc: "A semantic cache is a ..."
```

### 5. Rewriter（rewriter.go）

```go
rewriter := query.NewRewriter(llm)

// 重写查询
newQuery, err := rewriter.Rewrite(ctx, core.NewQuery("1", "tell me about it", nil))
// newQuery.Text: "What is semantic cache?"
```

### 6. StepBack（stepback.go）

```go
stepback := query.NewStepBack(llm)

// 生成回退问题
backQuery, err := stepback.GenerateStepBackQuery(ctx, core.NewQuery("1", "How does Go channel work?", nil))
// backQuery.Text: "What are the concurrency primitives in Go?"
```

---

## 如何与项目集成？

### 方式一：独立使用各组件

```go
// 根据需要选择组件
router := query.NewIntentRouter(llm)
decomposer := query.NewDecomposer(llm)
rewriter := query.NewRewriter(llm)

// 单独使用
intent, _ := router.Classify(ctx, userQuery)
subs, _ := decomposer.Decompose(ctx, userQuery)
rewritten, _ := rewriter.Rewrite(ctx, userQuery)
```

### 方式二：组合使用（Pipeline）

```go
// 构建查询处理管道
steps := []pipeline.Step{
    query.Classify(router),
    query.Extract(extractor),
    query.Decompose(decomposer),
    query.Rewrite(rewriter),
    query.StepBack(stepback),
    query.GenerateHyDE(hyde),
}

p := pipeline.New[*core.Query]()
for _, s := range steps {
    p.AddStep(s)
}
```

### 方式三：在 RAG 中启用

```go
// Advanced RAG 可配置查询处理选项
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
    gorag.WithQueryDecomposition(true),   // 启用问题分解
    gorag.WithQueryRewriting(true),       // 启用查询重写
    gorag.WithStepBack(true),             // 启用回退问题
)
```

---

## 测试状态

| 组件 | 测试覆盖 | 状态 |
|------|---------|------|
| IntentRouter | 11 个测试 | ✅ 通过 |
| Decomposer | 5 个测试 | ✅ 通过 |
| Rewriter | 4 个测试 | ✅ 通过 |
| StepBack | 3 个测试 | ✅ 通过 |
| EntityExtractor | 无测试 | ⏳ 待补充 |
| HyDE | 无测试 | ⏳ 待补充 |

---

## 适用于哪些场景？

### ✅ IntentRouter 适用

- **多模态检索**：需要根据意图选择不同检索路径
- **智能路由**：自动判断查询类型
- **系统优化**：闲聊直接返回，跳过昂贵检索

### ✅ Decomposer 适用

- **多跳问答**：需要多步推理的复杂问题
- **全面检索**：确保各个方面都被检索到
- **组合问题**：可以分解为多个独立问题

### ✅ HyDE 适用

- **语义模糊查询**：查询与文档表述差异大
- **专业领域**：需要专业术语补充
- **冷启动场景**：没有太多相关文档时

### ✅ Rewriter 适用

- **口语化查询**：去除闲聊成分
- **指代不明确**：代词消解
- **检索效果差**：原始查询检索质量不佳

### ✅ StepBack 适用

- **专业问题**：需要背景知识支撑
- **深层理解**：防止遗漏基础概念
- **学术写作**：需要全面背景调研

### ❌ 不适合使用

- **简单查询**：单跳直接回答
- **实时性要求高**：额外 LLM 调用增加延迟
- **资源受限**：LLM 调用成本敏感

---

## 配置推荐

| 场景 | 推荐配置 |
|------|----------|
| 高质量检索 | 全组件启用 |
| 平衡模式 | IntentRouter + Rewriter |
| 快速响应 | 仅 IntentRouter |
| 复杂推理 | Decomposer + StepBack + HyDE |

```go
// 生产环境推荐（平衡质量与性能）
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
    gorag.WithQueryDecomposition(true),
    gorag.WithQueryRewriting(true),
)
```