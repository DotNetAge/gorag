# Answer Generator - 答案生成器

## 什么是答案生成器？

答案生成器是 RAG 系统的核心组件，负责根据用户查询和检索到的上下文文档生成最终答案。它将检索阶段获取的相关文档片段与用户问题结合，通过 LLM 生成自然语言回答。

### 核心原理

```
用户查询 "如何学习 Go 语言？"
        ↓
    [检索阶段]
        ↓
  获取相关文档片段
        ↓
    [生成阶段]
        ↓
  构建 Prompt（上下文 + 问题）
        ↓
  调用 LLM 生成答案
        ↓
  返回最终回答
```

### 与检索的关系

| 阶段 | 职责 | 输出 |
|------|------|------|
| 检索阶段 | 找到相关文档 | 相关 Chunk 列表 |
| 生成阶段 | 生成最终答案 | 最终回答文本 |

---

## 有什么作用？

1. **生成自然语言答案**：将检索到的文档片段整合为流畅的回答
2. **上下文理解**：结合多个相关文档生成综合答案
3. **HyDE 支持**：通过假设文档技术提升检索质量
4. **可观测性**：内置指标收集，记录生成耗时和状态

---

## 怎么工作的？

### 1. 标准生成流程

```
用户查询 + 检索到的文档片段
        ↓
    构建 Prompt
        ↓
    调用 LLM（Chat 接口）
        ↓
    返回生成的答案
```

### 2. HyDE 生成流程

```
用户查询 "如何学习 Go 语言？"
        ↓
    生成假设文档
    "Go 语言学习指南：..."
        ↓
    用假设文档去检索
        ↓
    找到真实相关文档
        ↓
    基于真实文档生成答案
```

### 3. Prompt 模板

默认模板包含参考文档区域和用户问题区域：

```
You are a helpful and professional AI assistant.
Please answer the user's question based on the provided reference documents.
If the documents do not contain the answer, say "I don't know based on the provided context."

[Reference Documents]
{上下文文档}

[User Question]
{用户问题}

Answer:
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/answer/
└── generator.go      # Generator 实现
```

### 1. 核心结构

```go
type Generator struct {
    llm            chat.Client      // LLM 客户端
    promptTemplate string           // Prompt 模板
    logger         logging.Logger   // 日志记录
    collector      observability.Collector // 指标收集
}
```

### 2. 创建生成器

```go
generator := answer.New(
    myLLM,
    answer.WithPromptTemplate(customTemplate), // 可选：自定义模板
    answer.WithLogger(myLogger),               // 可选：日志
    answer.WithCollector(myCollector),         // 可选：指标
)
```

### 3. 生成答案

```go
result, err := generator.Generate(ctx, query, chunks)
if err != nil {
    return err
}
fmt.Println(result.Answer)
```

### 4. HyDE 生成

```go
// 生成假设文档（用于 HyDE 检索）
hypotheticalDoc, err := generator.GenerateHypotheticalDocument(ctx, query)
if err != nil {
    return err
}
// 用假设文档进行向量检索
```

### 5. 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithPromptTemplate` | 自定义 Prompt 模板 | 内置默认模板 |
| `WithLogger` | 日志记录器 | NoopLogger |
| `WithCollector` | 指标收集器 | NoopCollector |

---

## 如何与项目集成？

### 方式一：通过 RAG 入口自动集成（推荐）

Advanced RAG 默认已集成答案生成器：

```go
app, _ := gorag.DefaultAdvancedRAG(
    gorag.WithWorkDir("./data"),
)
// 生成器已在内部配置好
```

### 方式二：手动 Pipeline 组装（高级用法）

```go
// 创建 Pipeline
p := pipeline.New[*core.RetrievalContext]()

// 添加检索 Step
p.AddStep(vector.Search(store, embedder, opts))

// 添加生成 Step
p.AddStep(answer.New(llm, logger, collector))
```

### 方式三：HyDE 检索流程

```go
// 生成假设文档
hydeDoc, _ := generator.GenerateHypotheticalDocument(ctx, query)

// 使用假设文档进行检索（可能获得更好的召回）
chunks, _ := retriever.Retrieve(ctx, &core.Query{
    Text: hydeDoc,
})

// 基于真实文档生成答案
result, _ := generator.Generate(ctx, query, chunks)
```

---

## 使用效果

### 性能指标

| 指标 | 说明 |
|------|------|
| `generation_duration` | 答案生成耗时 |
| `generation_success` | 生成成功计数 |
| `generation_error` | 生成失败计数 |

### 返回结果

```go
type Result struct {
    Answer string  // 生成的答案文本
}
```

---

## 适用于哪些场景？

### ✅ 适合使用

- **RAG 系统**：需要结合检索和生成的实际问答
- **智能问答**：基于文档的自动问答系统
- **HyDE 检索**：需要提升向量检索质量的场景
- **文档摘要**：基于上下文的答案整合

### ❌ 不适合使用

- **纯生成任务**：不需要检索上下文（如创意写作）
- **简单匹配**：只需返回检索结果，无需生成
- **实时性要求极高**：LLM 调用有固定延迟

---

## 配置推荐

| 场景 | 推荐配置 |
|------|----------|
| 简单问答 | 默认 Prompt 模板 |
| 正式文档 | 自定义专业语气模板 |
| 快速原型 | 默认 + NoopLogger |
| 生产环境 | 自定义模板 + 完整日志 + 指标收集 |

```go
// 生产环境推荐
generator := answer.New(
    myLLM,
    answer.WithPromptTemplate(myProductionTemplate),
    answer.WithLogger(myLogger),
    answer.WithCollector(myCollector),
)
```
