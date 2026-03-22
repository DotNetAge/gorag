# Parent Document Expansion - 父文档扩展

## 什么是父文档扩展？

父文档扩展是一种**检索增强**技术。当检索返回子文档块（chunks）时，将其替换为完整的父文档，从而为 LLM 提供更丰富的上下文信息。

### 核心原理

```
检索返回子文档块 ["Go 语言的数据类型...", "Go 语言的并发编程..."]
                            ↓
                    [父文档扩展层]
                            ↓
            查询 DocumentStore 获取完整父文档
                            ↓
                替换为完整父文档内容
                            ↓
            返回完整父文档给后续处理
```

### 子文档 vs 父文档

| 特性 | 子文档块 | 父文档 |
|------|----------|--------|
| 来源 | 文档切分产生 | 原始完整文档 |
| 内容 | 部分内容 | 完整内容 |
| 上下文 | 有限 | 完整 |
| 适用场景 | 精确匹配 | 理解完整语义 |

---

## 有什么作用？

1. **丰富上下文**：为 LLM 提供完整的文档内容，而非碎片化的 chunks
2. **提升回答质量**：基于完整文档的推理更准确
3. **保留 Metadata**：合并父子文档的 Metadata 信息

---

## 怎么工作的？

### 1. 扩展流程

```
检索返回 chunks
        ↓
    遍历 chunks
        ↓
    ├── 无 ParentID → 直接保留
    │
    └── 有 ParentID
            ↓
        检查是否已处理过（去重）
            ↓
        ├── 已处理 → 跳过
        │
        └── 未处理
                ↓
            查询 DocumentStore 获取父文档
                ↓
            替换 chunk 内容
                ↓
            合并 Metadata
                ↓
            标记为已处理
```

### 2. 去重机制

同一个父文档可能有多个子文档块被检索到。扩展时只保留第一个，后面的重复父文档会被跳过：

```
检索结果: [chunk_1, chunk_2, chunk_3]
                    ↓
        chunk_1.ParentID = "doc_A"
        chunk_2.ParentID = "doc_A"  ← 重复
        chunk_3.ParentID = "doc_B"

扩展结果: [expanded_doc_A, expanded_doc_B]
```

### 3. Metadata 合并

父子文档的 Metadata 会被合并，子文档的 Metadata 优先级更高：

```
父文档 Metadata: {"author": "Alice", "version": "1.0"}
子文档 Metadata: {"chunk_index": 3, "author": "Bob"}

合并结果: {"author": "Bob", "version": "1.0", "chunk_index": 3}
```

---

## 我们怎么实现的？

### 包结构

```
pkg/retrieval/expand/
├── parent_doc.go    # ParentDoc 实现
└── expand_test.go  # 测试
```

### 1. DocumentStore 接口

```go
type DocumentStore interface {
    GetByID(ctx context.Context, id string) (*core.Document, error)
}
```

任何实现了 `GetByID` 方法的存储都可以作为父文档的数据源。

### 2. ParentDoc 结构

```go
type ParentDoc struct {
    docStore  DocumentStore
    logger    logging.Logger
    collector observability.Collector
}
```

实现了 `core.ResultEnhancer` 接口，可以无缝接入 RAG Pipeline。

### 3. 核心逻辑

```go
func (e *ParentDoc) Enhance(ctx context.Context, query *core.Query, chunks []*core.Chunk) ([]*core.Chunk, error) {
    // 1. 遍历每个 chunk
    // 2. 无 ParentID 的 chunk 直接保留
    // 3. 有 ParentID 的 chunk 查询 DocumentStore
    // 4. 用父文档内容替换子文档内容
    // 5. 合并 Metadata（子文档优先级更高）
    // 6. 通过 seenDocs map 实现去重
}
```

### 4. 配置选项

```go
expander := expand.NewParentDoc(
    myDocStore,
    expand.WithParentDocLogger(logger),        // 日志记录
    expand.WithParentDocCollector(collector), // 指标采集
)
```

---

## 如何与项目集成？

### 方式一：作为 ResultEnhancer 使用

ParentDoc 实现了 `core.ResultEnhancer` 接口，可以直接用于 Pipeline：

```go
// 创建父文档扩展器
expander := expand.NewParentDoc(
    myDocStore,
    expand.WithParentDocLogger(logger),
    expand.WithParentDocCollector(metrics),
)

// 在 Pipeline 中使用
p := pipeline.New[*core.RetrievalContext]()

// 添加检索步骤
p.AddStep(vector.Search(store, embedder, opts))

// 添加父文档扩展步骤
p.AddStep(expander.Enhance)

// 添加生成步骤
p.AddStep(generate.New(llm, logger))
```

### 方式二：与其他 Enhancer 组合

可以与 Metadata 增强、摘要生成等组合使用：

```go
p.AddStep(vector.Search(store, embedder, opts))
p.AddStep(expand.NewParentDoc(myDocStore))     // 父文档扩展
p.AddStep(enrich.WithSummary(llm))              // 摘要增强
p.AddStep(generate.New(llm, logger))           // 生成答案
```

---

## 使用效果

### 效果对比

| 场景 | 无扩展 | 有父文档扩展 |
|------|--------|--------------|
| LLM 收到的内容 | 碎片化的 chunks | 完整父文档 |
| 上下文完整性 | 低 | 高 |
| 回答相关性 | 可能缺失关键信息 | 完整理解文档意图 |

---

## 适用于哪些场景？

### ✅ 适合使用

- **长文档检索**：需要完整理解文档内容
- **法律/合同文档**：上下文完整性要求高
- **技术文档**：完整代码或教程片段
- **学术论文**：需要完整章节而非段落

### ❌ 不适合使用

- **简单问答**：FAQ 场景不需要完整文档
- **高度碎片化检索**：每个 chunk 独立有价值
- **对延迟敏感**：扩展增加一次存储查询
- **成本敏感**：更长的上下文意味着更高的 token 消耗
