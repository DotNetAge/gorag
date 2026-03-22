# Citation Generator - 引用生成器

RAG 系统的引用生成模块，通过在上下文中注入文档标记，引导 LLM 生成带引用的答案。

## 是什么

引用生成器是 RAG 系统的重要组成部分，它确保 LLM 在生成答案时明确标注信息来源，提高答案的可信度和可追溯性。

### 核心原理

```
用户查询: "法国的首都是哪里？"
           ↓
      [文档标记注入]
           ↓
[doc_1] Paris is the capital of France...
[doc_2] France is a country in Europe...

           ↓
      [LLM 生成答案]
           ↓
答案: "法国的首都是巴黎 [doc_1]"
```

### 标记格式

- `[doc_1]`, `[doc_2]`, `[doc_3]` ...
- 每个标记对应一个检索到的文档块

---

## 有什么用

1. **提高答案可信度**：答案中的每个声明都有明确的文献来源
2. **便于事实核查**：用户可以快速定位原文验证信息
3. **避免幻觉**：强制 LLM 基于提供的文档回答
4. **学术/专业场景**：需要引用来源的报告或文档

---

## 怎么工作的

### 处理流程

```
1. 接收用户查询和检索到的文档块列表
           ↓
2. 为每个文档块分配标记 [doc_1], [doc_2], ...
           ↓
3. 构建带标记的上下文字符串
           ↓
4. 生成 Prompt，包含：
   - 明确的指令（必须使用文档标记）
   - 带标记的文档内容
   - 用户查询
           ↓
5. 调用 LLM 生成答案
           ↓
6. 返回包含引用标记的答案
```

### Prompt 模板

```
You are a professional assistant. Please answer the user's question based STRICTLY on the provided documents.
You MUST cite your sources using the exact document markers provided (e.g., [doc_1], [doc_2]).
If a claim cannot be supported by the documents, do not make it. If the documents don't contain the answer, say "I don't have enough information."

[Documents]
[doc_1]
<chunk_1_content>

[doc_2]
<chunk_2_content>

[Question]
<user_query>

Answer:
```

---

## 我们怎么实现的

### 核心结构

```go
type CitationGenerator struct {
    llm chat.Client
}
```

### 接口

```go
func NewCitationGenerator(llm chat.Client) *CitationGenerator

func (g *CitationGenerator) GenerateWithCitations(
    ctx context.Context,
    query string,
    chunks []*core.Chunk,
) (string, error)
```

### 配置选项

当前实现简洁直接，配置通过构造函数注入：
- `NewCitationGenerator(llm)` - 创建引用生成器

---

## 如何与项目集成

### 基本用法

```go
// 创建引用生成器
citationGen := generation.NewCitationGenerator(llmClient)

// 获取检索结果
chunks := []*core.Chunk{
    {ID: "chunk1", Content: "Paris is the capital of France..."},
    {ID: "chunk2", Content: "France is a country in Europe..."},
}

// 生成带引用的答案
answer, err := citationGen.GenerateWithCitations(ctx, "法国的首都是哪里？", chunks)
fmt.Println(answer)
// 输出: "法国的首都是巴黎 [doc_1]"
```

### 在 Pipeline 中集成

```go
p := pipeline.New[*core.RetrievalContext]()

p.AddStep(retriever.Search(...))
p.AddStep(generation.NewCitationGenerator(llm))  // 带引用生成
```

### 与标准生成器对比

| 特性 | Generator | CitationGenerator |
|------|-----------|------------------|
| 输出格式 | 普通文本 | 带 `[doc_N]` 标记 |
| 引用支持 | ❌ | ✅ |
| Prompt 模板 | 可自定义 | 固定（引用专用） |

---

## 适用于哪些场景

### ✅ 适合使用

- **学术写作**：需要引用来源的论文或报告
- **法律文档**：需要明确引用法律条文
- **医疗咨询**：引用医学文献支持诊断建议
- **客服系统**：回答可追溯，便于人工复核

### ❌ 不适合使用

- **闲聊场景**：不需要严格的来源引用
- **创意写作**：不需要事实依据
- **简单问答**：问题过于简单，直接回答即可

---

## API 参考

### `generation.NewCitationGenerator`

```go
func NewCitationGenerator(llm chat.Client) *CitationGenerator
```

创建一个新的引用生成器实例。

**参数**：
- `llm`: chat.Client 实例，用于生成答案

**返回值**：
- `*CitationGenerator`: 引用生成器实例

### `CitationGenerator.GenerateWithCitations`

```go
func (g *CitationGenerator) GenerateWithCitations(
    ctx context.Context,
    query string,
    chunks []*core.Chunk,
) (string, error)
```

使用文档引用生成答案。

**参数**：
- `ctx`: 上下文
- `query`: 用户查询
- `chunks`: 检索到的文档块列表

**返回值**：
- `string`: 带引用的答案
- `error`: 错误信息

---

## 测试

```bash
go test ./pkg/generation/citation/... -v
```

**测试覆盖**：
- `TestCitationGenerator_New` - 构造函数
- `TestCitationGenerator_GenerateWithCitations` - 基本生成
- `TestCitationGenerator_GenerateWithCitations_EmptyChunks` - 空文档处理
- `TestCitationGenerator_GenerateWithCitations_SingleChunk` - 单文档处理
