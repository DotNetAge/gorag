# HyperIndexer 的定位

语义索引器（SemanticIndexer）与图索引器（GraphIndexer）是分属两种不同数据类型的索引器，更注重于处理单个文件。

而 HyperIndexer 则是两者的融合体，进行集中的分片与实体提取，并且还应该处理：

- 多文件间的**实体关系发现**——针对实体提取后，拟补不同文件之间的实体关系。
- 对提取的分片进行集中的加工：对于文档型分片，集中一次性地提取摘要、标签。
- 注入外部的实体 Schema 集，用于对文档中的实体进行有效提取。
- 拟补实体区域信息，这是 GoRAG 的重要约束之一：每个目录必须有一个 README.md 作为该目录下的说明性（摘要性）文件，也是用于作为实体区域定义的关键节点。

---

## 1. Summarizer 归入 HyperIndexer

### 现状

SemanticIndexer 当前注入了 `llm.Summarizer`，在 `Save` 方法内部逐个分片调用 LLM 补充 title/summary。这存在两个问题：

1. **职责错位**——SemanticIndexer 的核心职责是向量化+存储，摘要增强属于离线加工，不应内嵌在索引器中。
2. **成本不合理**——逐个分片调用 LLM，每个分片一次 API 请求，大量时间消耗在请求往返上。

### 决策

- **Summarizer 从 SemanticIndexer 移除**：`semanticIndexer` 不再持有 `summarizer` 字段，`WithSemanticSummarizer` 选项删除，`Save` 方法中不再包含 Summarizer 调用逻辑。
- **Summarizer 注入到 HyperIndexer**：HyperIndexer 新增 `summarizer llm.Summarizer` 字段和 `WithHyperSummarizer` 选项。
- **调用时机**：HyperIndexer.AddFile 完成分块后、调用语义线 Save 之前，集中执行一次 Summarizer。

### Summarizer 接口设计

**原有接口保留**：`Summarize(ctx, []core.Chunk) ([]core.Chunk, error)`，逐分片调用 LLM。原有实现 `gochatSummarizer` 保留作为 fallback。

**新增批量接口**：

```go
type BatchSummarizer interface {
    // SummarizeBatch 一次 LLM 请求处理所有分片。
    // 内部将 chunks 序列化为 JSON 数组，要求 LLM 一次性填充所有 title/summary。
    SummarizeBatch(ctx context.Context, chunks []core.Chunk) ([]core.Chunk, error)
}
```

**批量模式流程**：

```
输入：[]core.Chunk（每个 chunk 带有 ID、Content）
内部流程：
  1. 将分片数组序列化为 JSON 数组字符串，每条含：
     {
       "chunk_id": "xxx",
       "content": "原文",
       "title": "",
       "summary": ""
     }
  2. 一次 LLM 调用，要求 LLM 为每条填充 title 和 summary
  3. 解析 LLM 返回的 JSON 数组，按 chunk_id 反喂回 Chunks
输出：[]core.Chunk（title 和 summary 已填充）
```

**`gochatBatchSummarizer` 同时实现 `Summarizer` 和 `BatchSummarizer`**：
- `Summarize(ctx, chunks)` — 保留现有逐分片逻辑，作为备用
- `SummarizeBatch(ctx, chunks)` — 主打模式，一次 LLM 请求

这样做的好处：
- N 个分片只需 1 次 LLM 请求，而非 N 次
- LLM 能感知上下文边界，避免同一份内容的重复摘要
- API 成本降低约 N-1 倍
- 调用方可根据场景自由选择

**触发条件**：仅对 `RawDocDoc`（文档类）且 Content 长度 >= 100 字符的分片运行。代码块、图片块、数据块跳过。

**调用点**：HyperIndexer.AddFile 在第 4 步（分块完成）和第 6 步（语义线 Save）之间插入：

```
AddFile 流程（修改后）：
  1. document.Open(filePath) → RawDoc
  2. core.NewStructuredDoc(raw) → StructuredDoc
  3. 路由 Chunker → chunkerImpl.Chunk(raw) → ChunkResult
  4. doc.SetChunks/SetNodes/SetEdges
  [4.5] Summarizer 调用：HyperIndexer 自行调用 summarizer.Summarize(ctx, doc.Chunks())
        对文档类分片批量摘要，结果写回 doc
  5. semantic.Save(doc)（向量化 + 写入 VectorStore）
  6. graph.Save(doc)（实体 + CONTAINS 边，若 graph 存在）
```

---

## 2. 实体区域信息（README.md 约束）

### 背景

GoRAG 有一个重要约束：**每个目录必须有一个 README.md 作为该目录下的说明性（摘要性）文件，也是用于作为实体区域（Region）定义的关键节点。**

RegionID 同时横跨 Chunk（语义线）和 Node（关系线），由目录路径的 SHA256 哈希生成，通过 Chunk.Source 或 Node.Properties["source_file"] 的目录部分推导。

### 决策

- 此约束由应用端强制，HyperIndexer 作为最贴近应用端的入口，在内部封装 Region 提取逻辑。
- RegionID 的提取逻辑独立为 HyperIndexer 的内部方法（如 `resolveRegionID(filePath string) string`），在 AddFile 过程中调用。
- History：该逻辑原属于 structurizer 包（已删除），现在迁入 HyperIndexer。

### 实现要点

- 入口参数：文件绝对路径
- RegionID = SHA256(dirname of filePath) → hex[:16]
- 无需依赖 README.md 是否存在（应用层保证其存在性）

---

## 3. 多文件实体关系发现（Update 方法）

### 背景

GraphIndexer 的 AddFile 是按文件逐个处理的。多文件间的实体关系发现需要跨文件实体匹配（如同名实体、同源实体合并）。

### 决策

- HyperIndexer 新增 `Update(ctx context.Context) error` 方法，作为多文件实体关系发现的触发点。
- Update 的职责：遍历所有已索引的实体，执行跨文件关系发现（实体合并、关系推断）。
- 此方法可能需要 LLM 参与（如判断两个同名实体是否为同一实体），但也可能仅用规则（如词汇相似度、共现频率）。具体实现方案待讨论。
- Update 不是 AddFile 的隐式步骤，而是**显式调用**——用户或调度器在合适的时机（如全部文件索引完成后、或定期维护时）主动触发。

### 调用时机示例

```go
idx, _ := NewHyperIndexer(semanticIdx, graphIdx)
idx.AddFile(ctx, "doc1.md")
idx.AddFile(ctx, "doc2.md")
idx.AddFile(ctx, "doc3.md")
// 全部索引完成后，触发跨文件实体关系发现
idx.Update(ctx)
```

---

## 4. 外部实体 Schema 注入（AddSchemas 方法）

### 现状

`llm/schema.go` 中已有 `EntitySchema` 类型，包含 Type、Prompt、JSONSchema 三个字段。Refiller 直接消费 `[]EntitySchema`。

外部 Schema 定义文件位于 `mindx/runtime/schemas/`，按领域分类（enterprise、finance、general、journalism、media、medical、research、tech、writing），可通过 `LoadEntitySchemasFromDir` 函数加载为 `[]EntitySchema`。

### 决策：注册端

- HyperIndexer 新增 `AddSchemas(path string, schemas []llm.EntitySchema)` 方法。
- **`path` 是文件系统路径**，指向该组 Schema 的源目录（如 `"schemas/general"`），保留溯源语义——调用方和技术人员可以追踪这些实体定义来自哪个配置文件目录，domain 是 path 的目录名。
- `schemas` 使用 `llm/schema.go` 中已定义的 `EntitySchema` 类型，经由 `LoadEntitySchemasFromDir` 或自定义加载函数解析后的产物。
- **path 不用于文件读取**（`schemas` 已经是解析好的），仅用于标识和追踪——HyperIndexer 将 `path` 作为标识，透传给 GraphIndexer，使实体定义可溯源到其源目录。

### 消费端：Refiller 实体提取兜底

Schema 注册的最终目的是供 **Refiller** 消费。Refiller 已是 Graph 线的实体提取兜底机制，类似于 Summarizer 是语义线的摘要兜底。

**现有 `llm.Refiller` 接口**：

```go
type Refiller interface {
    Refill(ctx context.Context, result chunker.ChunkResult, schemas []EntitySchema) (chunker.ChunkResult, error)
}
```

它已经实现了所需流程：
1. 序列化 ChunkResult 的 Chunks 为 JSON 数组字符串
2. 将 Schema 注入 LLM 系统提示词
3. 一次 LLM 调用提取实体和关系
4. 返回补充了 Nodes/Edges 的 ChunkResult

**在 HyperIndexer.AddFile 中的位置**：

```
AddFile 流程（完整）：
  1. document.Open(filePath) → RawDoc
  2. core.NewStructuredDoc(raw) → StructuredDoc
  3. 路由 Chunker → chunkerImpl.Chunk(raw) → ChunkResult
  4. doc.SetChunks/SetNodes/SetEdges

  4.5 [语义线加工] Summarizer: 对文档类分片批量摘要，结果写回 doc

  5. semantic.Save(doc)（向量化 + 写入 VectorStore）

  5.5 [关系线加工] Refiller:
      a. 从 doc.Chunks() 获取当前文件的分片
      b. 从 HyperIndexer 的注册表中获取已传入的 Schema 列表
      c. 调用 refiller.Refill(ctx, ChunkResult, schemas) → 返回补充后的 result
      d. 将新增的 Nodes/Edges 合并到 doc（doc.SetNodes/SetEdges）

  6. graph.Save(doc)（实体 + CONTAINS 边 + 写入 GraphStore，若 graph 存在）
```

**可选注入**：Refiller 同 Summarizer 一样是可选组件。HyperIndexer 新增 `refiller llm.Refiller` 字段和 `WithHyperRefiller` 选项。未注入时，关系线只走 Chunker 代码解析器产出的 Nodes/Edges，不走 LLM 兜底。

## 5. 事件扩展点（Hooks）

HyperIndexer 的 AddFile 管线是固定的，但调用方可以通过事件 Hook 在管线关键节点插入自定义逻辑，无需修改核心代码。

### 管线与 Hook 点

```
Open → [OnFileOpened] → Chunk → [OnChunk]⁺
      → Summarizer
      → [OnBeforeSemanticSave] → semantic.Save → [OnAfterSemanticSave]
      → Refiller
      → [OnBeforeGraphSave] → graph.Save → [OnAfterGraphSave]
      → [OnIndexComplete]
```

### Hook 规约

| Hook                   | 触发时机                                     | 可修改的数据                          | 语义线/关系线 | 典型用途                       |
| ---------------------- | -------------------------------------------- | ------------------------------------- | ------------- | ------------------------------ |
| `OnFileOpened`         | document.Open 之后, Chunker 之前             | `RawDoc`                              | 公共          | 文件类型白名单过滤             |
| `OnChunk`              | 每个 Chunk 产生后（遍历 Chunks 时触发 N 次） | `*core.Chunk`                         | 公共          | 敏感词过滤、补充标签、内容截断 |
| `OnBeforeSemanticSave` | Summarizer 之后, semantic.Save 之前          | `StructuredDoc`（可修改 Chunks）      | 语义线        | 批量审核、外部 API 增强        |
| `OnAfterSemanticSave`  | semantic.Save 成功之后                       | `StructuredDoc`（只读）               | 语义线        | 向量化审计、指标采集           |
| `OnBeforeGraphSave`    | Refiller 之后, graph.Save 之前               | `StructuredDoc`（可修改 Nodes/Edges） | 关系线        | 实体去重、关系验证             |
| `OnAfterGraphSave`     | graph.Save 成功之后                          | `StructuredDoc`（只读）               | 关系线        | 图写入审计、指标采集           |
| `OnIndexComplete`      | AddFile 所有步骤完成                         | `[]*core.Chunk`（最终产物）           | 公共          | 通知下游、缓存预热             |

语义线和关系线在 Hook 点上完全对称——各有一个预存 Hook 和一个后置 Hook。

### 接口设计

小接口组合，每个 Hook 独立成接口：

```go
type (
    OnFileOpenedHook     interface{ OnFileOpened(ctx context.Context, doc document.RawDoc) (document.RawDoc, error) }
    OnChunkHook          interface{ OnChunk(ctx context.Context, chunk *core.Chunk) (*core.Chunk, error) }

    OnBeforeSemanticSave interface{ OnBeforeSemanticSave(ctx context.Context, doc core.StructuredDoc) (core.StructuredDoc, error) }
    OnAfterSemanticSave  interface{ OnAfterSemanticSave(ctx context.Context, doc core.StructuredDoc) error }

    OnBeforeGraphSave    interface{ OnBeforeGraphSave(ctx context.Context, doc core.StructuredDoc) (core.StructuredDoc, error) }
    OnAfterGraphSave     interface{ OnAfterGraphSave(ctx context.Context, doc core.StructuredDoc) error }

    OnIndexCompleteHook  interface{ OnIndexComplete(ctx context.Context, result []*core.Chunk) error }
)
```

### 注册方式

```go
type HyperIndexer struct {
    // ... 现有字段
    hooks struct {
        onFileOpened     []OnFileOpenedHook
        onChunk          []OnChunkHook
        onBeforeSemantic []OnBeforeSemanticSave
        onAfterSemantic  []OnAfterSemanticSave
        onBeforeGraph    []OnBeforeGraphSave
        onAfterGraph     []OnAfterGraphSave
        onIndexComplete  []OnIndexCompleteHook
    }
}

func WithHooks(hooks ...any) HyperOption {
    return func(h *HyperIndexer) {
        for _, hook := range hooks {
            switch v := hook.(type) {
            case OnFileOpenedHook:     h.hooks.onFileOpened = append(h.hooks.onFileOpened, v)
            case OnChunkHook:          h.hooks.onChunk = append(h.hooks.onChunk, v)
            case OnBeforeSemanticSave: h.hooks.onBeforeSemantic = append(h.hooks.onBeforeSemantic, v)
            case OnAfterSemanticSave:  h.hooks.onAfterSemantic = append(h.hooks.onAfterSemantic, v)
            case OnBeforeGraphSave:    h.hooks.onBeforeGraph = append(h.hooks.onBeforeGraph, v)
            case OnAfterGraphSave:     h.hooks.onAfterGraph = append(h.hooks.onAfterGraph, v)
            case OnIndexCompleteHook:  h.hooks.onIndexComplete = append(h.hooks.onIndexComplete, v)
            }
        }
    }
}
```

### 错误处理策略

- **修改型 Hook**（OnFileOpened、OnChunk、OnBeforeSemanticSave、OnBeforeGraphSave）：返回 error 时阻塞管线，调用方需处理错误。
- **通知型 Hook**（OnAfterSemanticSave、OnAfterGraphSave、OnIndexComplete）：返回 error 时仅记录日志，不阻塞管线。

### 初始实现

首次实现只覆盖最刚需的 4 个 Hook：OnFileOpened、OnChunk、OnBeforeSemanticSave、OnIndexComplete。其余 Hook（OnAfterSemanticSave、OnBeforeGraphSave、OnAfterGraphSave）在需要时追加。

---

## 附：当前代码对应关系（2026-07）

| 特性                        | 状态                           | 位置                 |
| --------------------------- | ------------------------------ | -------------------- |
| Chunk.Tags 字段             | ✅ 已添加                       | core/chunk.go:38     |
| Summarizer 接口（逐分片）   | ✅ 已存在                       | llm/summarizer.go    |
| BatchSummarizer 批量接口    | 🔄 待新增                       | llm/summarizer.go    |
| gochatBatchSummarizer 实现  | 🔄 待实现（批量 JSON 数组模式） | llm/summarizer.go    |
| Summarizer 从 semantic 移除 | 🔄 待实施                       | indexer/semantic.go  |
| Summarizer 注入 hyper       | 🔄 待实施                       | indexer/hyper.go     |
| EntitySchema 类型           | ✅ 已完成                       | llm/schema.go:15     |
| AddSchemas 方法             | 🔄 待实施                       | indexer/hyper.go     |
| Refiller 注入 HyperIndexer  | 🔄 待实施                       | indexer/hyper.go     |
| Update 方法                 | 🔄 待实施                       | indexer/hyper.go     |
| RegionID 逻辑               | 🔄 待迁入                       | indexer/hyper.go     |
| 事件 Hook 接口              | 🔄 待新增（OnHook 接口族）      | indexer/hyper.go     |
| WithHooks 注册选项          | 🔄 待新增                       | indexer/hyper.go     |
| TreeViewBuilder             | ⏳ 待定                         | indexer/hyper.go:391 |
