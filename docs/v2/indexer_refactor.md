# indexer 包重构设计

> 本文档是 indexer 包的最终设计稿，取代 `docs/v2/README.md` §7.5 / §7.7 / §8.6 中关于 indexer 包的所有 V2 设计。
> V2 阶段产生的 `v2_*.go` 文件全部删除，V1 文件原地升级为本文档定义的最终形态。

---

## 1. 重构背景

### 1.1 V1 的问题:接口违反 SRP

V1 `core.Indexer` 接口 11 个方法承担了 6 种职责,严重违反单一职责原则:

| 职责         | 方法                                         |
| ------------ | -------------------------------------------- |
| 元信息       | `Name` / `Type`(`Type` 与 `Name` 语义重复)   |
| 写入(含 LLM) | `Add` / `AddFile`(`Add` 接收字符串,无法溯源) |
| 写入(直存)   | `StoreChunk`                                 |
| 查询构造     | `NewQuery`                                   |
| 检索         | `Search`                                     |
| 浏览         | `List` / `GetChunks` / `Count`               |
| 维护         | `Remove` / `Clear`                           |

V1 `Add(ctx, content string)` 接收字符串输入,违反"索引系统必须使用绝对路径"的硬性约束:
- 字符串无法溯源,丢失 `source_file` / `region_id` 等元数据
- 字符串输入不经过 `document.Open`,绕过文件归一化流程
- 与 Region 层级(基于文件目录推导)冲突

V2 删除 `Add` 方法,**只支持文件输入**(`AddFile`),Indexer 职责单一化为"索引文件"。

### 1.2 V2(v2_*.go)的问题:拆分方式错误

V2 用 `Indexer` + `IndexerAdmin` 二分法,但按"是否核心"划分而非"职责类型"划分,导致:

| 方法              | V2 归属      | 问题                                                     |
| ----------------- | ------------ | -------------------------------------------------------- |
| `Index(ctx, doc)` | Indexer      | 把 LLM 剥离到调用方,职责走极端                           |
| `Remove`          | Indexer      | Remove 是维护操作,应与 Clear 同组                        |
| `StoreChunk`      | IndexerAdmin | StoreChunk 是写入操作,与 Index 同类                      |
| `Close`           | Indexer      | Close 是资源管理,不是索引职责                            |
| `SearchGraph`     | Indexer      | 只有 GraphIndexer 实现,SemanticIndexer 返回 nil,接口污染 |

V2 还犯了三个严重错误:
1. **LLM 调用从 Indexer 剥离**:把"读取→分块→写入"的全流程拆解,调用方必须手动编排 Chunker/Index 两步,API 复杂度爆炸
2. **CONTAINS 完全改为 BELONGS_TO**:破坏了 Region 作为区域节点的层级特性,且未在视图层补回 `Document → Chunk` 层级,导致导航树丢失分片子节点
3. **Chunk 从 GraphStore 移除后未提供视图层补偿**:Chunk 不作为 Node 写入 GraphStore 是正确方向,但 V2 未通过 TreeViewBuilder 从 VectorStore 动态组装 Document→Chunk 层级,使区域树成为断链

### 1.3 V1/V2 共存是错误的

成熟架构不会同时提供两个版本的对外接口。外部接口包是使用入口,必须单一明确。V1/V2 共存策略(`V2` 后缀)是技术债务的体现,把"用哪个/何时废弃/何时稳定"的决策成本转嫁给使用者。

---

## 2. 设计原则

1. **组合优于继承**:Go 风格的小接口组合,而非胖接口继承。参考 `io` 包的 `Reader`/`Writer`/`Closer`/`Seeker` 设计。
2. **接口分离原则(ISP)**:调用方不应该依赖它不需要的方法。只有部分索引器实现的能力(如 `SearchGraph` / `TreeViewBuilder`)独立为扩展接口,不污染 SemanticIndexer。
3. **按职责类型拆分**:不按"是否核心"拆分,而按"职责类型"拆分。写入、检索、浏览、维护、资源管理各自独立。
4. **只支持文件输入**:Indexer 只负责"索引文件",不支持字符串输入。所有内容必须先落盘为文件,通过 `AddFile(ctx, filePath)` 索引。这样:
   - 强制使用绝对路径(符合硬性约束)
   - 保留 `source_file` / `region_id` 等元数据
   - 经过 `document.Open` 文件归一化流程
   - 支持 Region 层级(基于文件目录推导)
5. **LLM 内置不剥离**:`AddFile` 内置 LLM 全流程(读取→分块→向量化/图结构化→写入),调用方一行完成索引。需要分步控制的场景走 Chunker/Summarizer 独立接口,不污染 Indexer。
6. **保留 Region→Document CONTAINS 层级**:`Region --CONTAINS--> Document` 是 Region 作为区域节点的核心特性,必须保留。`Document → Chunk` 的层级不在 GraphStore 中体现,由 `TreeViewBuilder` 在视图层通过 `VectorStore` 动态组装。
7. **保留多维度向量索引**:每个 Chunk 生成 1~3 条向量(Content/Title/Summary),提高召回率。这是 V2 中少数合理的设计,保留。向量化由 `SemanticIndexer` 负责,从 Chunk 字段提取文本后调用 `Embedder` 的纯文本向量接口。
8. **构造函数返回 `(Indexer, error)`**:保留 nil 检查,但返回接口而非具体类型。
9. **单一版本**:删除所有 `v2_*.go` 文件,V1 文件原地升级为最终形态。不提供多版本共存。

---

## 3. 接口设计

所有接口定义在 `indexer` 包,避免 `core` 包过度膨胀并方便组合。

### Indexer 包接口(6 个)

| 接口              | 方法                              | 说明                                    |
| ----------------- | --------------------------------- | --------------------------------------- |
| `Indexer`         | Name/AddFile/Search/NewQuery      | 核心(必实现)                            |
| `IndexerStore`    | Save                              | 存储(各 Indexer 实现自动路由到各自存储) |
| `IndexerAdmin`    | List/GetChunks/Count/Remove/Clear | 管理                                    |
| `IndexerCloser`   | Close                             | 资源管理                                |
| `TreeViewBuilder` | Tree                              | 导航(仅 HyperIndexer 实现)              |
| `GraphSearcher`   | SearchGraph                       | 图查询(仅 GraphIndexer 实现)            |

### 组件接口(2 个,定义在各自包)

| 接口         | 包      | 注入到          | 必/可选 |
| ------------ | ------- | --------------- | ------- |
| `Chunker`    | chunker | HyperIndexer    | 必      |
| `Summarizer` | llm     | SemanticIndexer | 可选    |

### 3.1 Indexer(核心接口)

索引器核心接口:负责文件索引和检索。所有索引器必须实现。

**只支持文件输入,不支持字符串输入**——所有内容必须先落盘为文件,通过 `AddFile` 索引。这是 V2 的核心特点,使 Indexer 职责单一化为"索引文件"。

```go
// Indexer 索引器核心接口:负责文件索引和检索。
// 所有索引器必须实现此接口。
//
// 设计要点:
//   - 只支持文件输入(AddFile),不支持字符串输入
//   - 所有内容必须先落盘为文件,通过 document.Open 归一化
//   - source_file / region_id 等元数据依赖文件路径
type Indexer interface {
    // Name 返回索引器名称(如 "semantic" / "graph" / "hyper")
    Name() string

    // AddFile 从文件读取内容后执行索引全流程。
    // 实现可能是 SemanticIndexer(分块+向量化+语义检索)、
    // GraphIndexer(分块+图结构化,独立使用为纯图谱模式,详见 §7.11)、
    // HyperIndexer(双线协同:语义线+关系线)。
    // filePath 必须为绝对路径。
    AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error)

    // Search 执行检索,返回命中的 Hit 容器(持有 Chunks/Nodes/Edges)。
    // Hit 与 StructuredDoc 对称——StructuredDoc 是存,Hit 是取(详见 §7.12)。
    Search(ctx context.Context, query core.Query) (*core.Hit, error)

    // NewQuery 构造查询对象。
    NewQuery(terms string) core.Query
}
```

### 3.2 IndexerStore(存储接口)

存储接口:保存 StructuredDoc 到各自存储。**各 Indexer 的 Save 实现自动路由到各自存储**——这是存储路由的关键。

**Save 的本质是"各 Indexer 保存各自需要的数据的入口"**——不返回 Chunks,因为 Chunks 只是中间产物,不是"保存的数据"。HyperIndexer 负责调用 Chunker 分块并把 Chunks/Nodes/Edges 存入 StructuredDoc,各 Indexer 从 doc 读取各自需要的数据。

```go
// IndexerStore 存储接口:保存 StructuredDoc 到各自存储。
//
// 各 Indexer 的 Save 实现自动路由到各自存储:
//   - SemanticIndexer.Save → 从 doc.Chunks() 读取,向量化,写入 VectorStore
//   - GraphIndexer.Save → 从 doc.Nodes()/Edges() 读取实体/关系,维护 Region→Document 的 CONTAINS 边,写入 GraphStore;Chunk 不作为 Node 写入 GraphStore
//
// 语义:
//   - Save 接收的 StructuredDoc 已完成"读文件+归一化+分块+结构化"
//   - Chunks/Nodes/Edges 已由 HyperIndexer 调用 Chunker 填充到 StructuredDoc
//   - 各 Indexer 从 doc 读取各自需要的数据(向量化/图结构化)
//   - Save 只负责"保存各自需要的数据",不返回 Chunks
type IndexerStore interface {
    // Save 保存已结构化的文档到各自存储。
    // 不返回 Chunks——Chunks 已在 doc 中,各 Indexer 只负责保存。
    Save(ctx context.Context, doc core.StructuredDoc) error
}
```

**与 V2 错误的 `Index(ctx, doc)` 的本质区别**:

| 维度     | V2 的 `Index(ctx, doc)`       | 新的 `Save(ctx, doc)`                                      |
| -------- | ----------------------------- | ---------------------------------------------------------- |
| 接口地位 | 主接口方法(必实现)            | 存储接口方法(实现 IndexerStore 的 Indexer 可被 Hyper 组合) |
| doc 内容 | 调用方填充 Chunks/Nodes/Edges | HyperIndexer 调用 Chunker 填充 Chunks/Nodes/Edges          |
| LLM 调用 | 剥离到调用方                  | 保留在 Indexer 内部(Summarizer/Chunker)                    |
| 返回值   | 返回 Chunks                   | 只返回 error                                               |
| 设计意图 | 强制调用方分步编排            | 存储路由——各 Indexer.Save 自动路由到各自存储               |

**合并 IndexExt 与原 IndexerStore 的理由**:
- `IndexExt.AddDoc(doc)` 和 `IndexerStore.StoreChunk(chunk)` 本质都是"保存数据到存储"
- 合并为 `IndexerStore.Save(doc)` 后,语义统一
- 各 Indexer 的 Save 实现自动路由到各自存储(VectorStore / GraphStore)
- 接口数从 7 个减少到 6 个,更精简

### 3.3 IndexerAdmin(管理接口)

索引器管理接口:浏览、统计、维护。调用方按需 type-assert。

```go
// IndexerAdmin 索引器管理接口:浏览、统计、维护。
// 调用方按需 type-assert: if a, ok := idx.(IndexerAdmin); ok { ... }
type IndexerAdmin interface {
    // List 分页浏览已索引的 Chunk。
    List(ctx context.Context, offset, limit int, filters []core.FilterCondition) ([]core.Chunk, int, error)

    // GetChunks 按 DocID 获取该文档的所有 Chunk。
    GetChunks(ctx context.Context, docID string) ([]*core.Chunk, error)

    // Count 返回已索引的 Chunk 总数。
    Count(ctx context.Context) (int, error)

    // Remove 按 ChunkID 移除索引项。
    // SemanticIndexer 删除 VectorStore 中的主向量及从属维度向量;
    // GraphIndexer 不实现 IndexerAdmin,图数据清理通过其他机制处理。
    Remove(ctx context.Context, chunkID string) error

    // Clear 清空索引。
    Clear(ctx context.Context) error
}
```

### 3.4 IndexerCloser(资源管理接口)

资源管理接口:释放底层资源。

```go
// IndexerCloser 资源管理接口:释放底层存储资源。
type IndexerCloser interface {
    // Close 释放底层资源。
    Close(ctx context.Context) error
}
```

### 3.5 TreeViewBuilder(导航接口)

知识库导航接口:构建 `Region → Document → Chunk` 层级树。由 **HyperIndexer** 实现,因为该树需要同时访问 GraphStore 中的 `Region → Document` 层级和 VectorStore 中的 `Document → Chunk` 层级。

```go
// TreeViewBuilder 知识库导航接口:构建 Region→Document→Chunk 层级树。
// 由 HyperIndexer 实现;GraphIndexer 只维护 Region→Document 的 CONTAINS 边,不直接提供 Chunk 子节点。
type TreeViewBuilder interface {
    // Tree 输出基于 Region 层级的知识树。
    // regionID 为空时返回整棵树;非空时返回该 Region 子树。
    // Document 子节点下的 Chunk 子节点由 HyperIndexer 通过 SemanticIndexer/IndexerAdmin 从 VectorStore 动态补齐。
    Tree(ctx context.Context, regionID string) (*core.TreeNode, error)
}
```

**实现方式**:
1. 从 GraphIndexer 取得 `Region → Document` 树(GraphStore 中仅存在这两层)。
   GraphIndexer 不实现 `TreeViewBuilder` 接口,仅提供未导出的 `regionTree` 方法供 HyperIndexer 内部组合。
2. 对每个 Document 节点,通过 `SemanticIndexer.GetChunks(ctx, docID)` 从 VectorStore 读取该文档的 Chunk 列表。
3. 将 Chunk 挂载为对应 Document 的子节点,形成完整的 `Region → Document → Chunk` 视图树。

这样 GraphStore 只保存 Region/Document 层级与实体关系,Chunk 层级由 VectorStore 提供,避免同一份 Chunk 数据在两种存储中重复写入。

### 3.6 GraphSearcher(图查询扩展接口)

图查询扩展接口:执行图检索,返回 `*Hit`(填充 Nodes/Edges)。仅 GraphIndexer 实现。

**与 `Indexer.Search` 统一返回 `*Hit`**——`SearchGraph` 返回的 Hit 中 Chunks 为空,仅填充 Nodes/Edges。客户端可对 `Search` 和 `SearchGraph` 的结果直接做 Fusion 融合。`GraphResult` 类型删除。

```go
// GraphSearcher 图查询扩展接口:执行图检索,返回 *Hit(Nodes/Edges 填充)。
// 仅 GraphIndexer 实现,SemanticIndexer 不维护 GraphStore。
type GraphSearcher interface {
    // SearchGraph 执行图查询,返回 Hit(Nodes/Edges 填充,Chunks 为空)。
    // 与 Indexer.Search 统一返回 *Hit,便于 Fusion 融合。
    SearchGraph(ctx context.Context, query core.Query) (*core.Hit, error)
}
```

### 3.7 组件接口(Chunker / Summarizer)

这两个接口定义在各自包,通过构造函数注入 Indexer,是 Indexer 的可替换组件。当前设计中 **Chunker 已完全取代 Extractor**——分块器在切分文档的同时,基于语法/结构规则产出 Nodes/Edges,不再单独维护 Extractor 组件。

#### 3.7.1 Chunker(分块器)

定义在 `chunker` 包,注入到 HyperIndexer。HyperIndexer 调用 Chunker 分块,把 Chunks/Nodes/Edges 存入 StructuredDoc。

```go
// chunker 包

// ChunkResult 分块结果容器。
// Chunker 在分块过程中同时产出结构化的 Nodes/Edges,因此返回 ChunkResult。
type ChunkResult struct {
    Chunks []core.Chunk // 分片列表
    Nodes  []core.Node  // 结构节点(如 heading、函数、类、数据表等)
    Edges  []core.Edge  // 结构关系(如 CONTAINS、BELONGS_TO、CALLS、INHERITS、IMPLEMENTS 等)
}

type Chunker interface {
    // Chunk 对原始文档进行分块,返回分块结果(含 Chunks/Nodes/Edges)。
    // 由 HyperIndexer 调用,结果存入 doc.SetChunks()/SetNodes()/SetEdges()。
    Chunk(doc document.RawDoc) (ChunkResult, error)
}
```

#### 3.7.2 Summarizer(摘要器,可选)

定义在 `llm` 包,可选注入到 SemanticIndexer。**条件式使用**:只有当 Chunk 的 title/summary 为空时,才调用 Summarizer 补充;没有 Summarizer 或补充失败时,直接跳过对应维度的向量,不强制填充。

**传 string 而非 Chunk,与 core.Chunk 解耦**,可用于任何需要标题+摘要的场景。

```go
// llm 包
type Summarizer interface {
    // Summarize 从文本生成标题和摘要。
    // 与 core.Chunk 解耦,调用方负责把结果赋值给 chunk.Title/Summary。
    Summarize(ctx context.Context, text string) (title, summary string, err error)
    Name() string
}
```

**条件式使用流程**:
```go
// SemanticIndexer.Save 内部
for _, chunk := range doc.Chunks() {
    if s.summarizer != nil && (chunk.Title == "" || chunk.Summary == "") {
        title, summary, _ := s.summarizer.Summarize(ctx, chunk.Content)
        if chunk.Title == "" { chunk.Title = title }
        if chunk.Summary == "" { chunk.Summary = summary }
    }
    // 主向量(Content):ID = chunk.ID
    if vec, err := s.embedder.CalcText(chunk.Content); err == nil {
        vec.ChunkID = chunk.ID
        s.vectorDB.Upsert(ctx, []*core.Vector{vec})
    }
    // 标题维度:ID = chunk.ID:title(字段为空则跳过)
    if chunk.Title != "" {
        if vec, err := s.embedder.CalcText(chunk.Title); err == nil {
            vec.ChunkID = chunk.ID + ":title"
            s.vectorDB.Upsert(ctx, []*core.Vector{vec})
        }
    }
    // 摘要维度:ID = chunk.ID:summary(字段为空或没有 Summarizer 则跳过)
    if chunk.Summary != "" {
        if vec, err := s.embedder.CalcText(chunk.Summary); err == nil {
            vec.ChunkID = chunk.ID + ":summary"
            s.vectorDB.Upsert(ctx, []*core.Vector{vec})
        }
    }
}
```

#### 3.7.3 为什么 Chunker 取代 Extractor?

旧设计把"实体提取"作为独立 `Extractor` 接口,由 GraphIndexer 可选注入。实践证明:

- 确定性结构(heading 层级、代码 AST、数据路径)在分块阶段即可用规则精确提取,不需要 LLM。
- 把提取逻辑下沉到 Chunker,避免了 HyperIndexer 中"Chunker 分块 → Extractor 再遍历提取"的重复解析。
- Chunker 已经掌握完整 AST/结构信息,产出的 Nodes/Edges 与 Chunks 天然对齐(`SourceChunkIDs` 精确绑定)。

因此当前设计将实体/关系提取能力并入 Chunker,`extractor` 包不再作为独立组件存在。GraphIndexer 只负责把 Chunker 产出的 Nodes/Edges 写入 GraphStore,并维护 Region→Document 的 CONTAINS 边;GraphIndexer 不再依赖 `extractor` 包。

### 3.8 HyperIndexer(复合索引器)

HyperIndexer 是双线结合的契机——协调 SemanticIndexer(语义线)和 GraphIndexer(关系线)。

**核心设计**:
- GraphIndexer 职责独立,不做分块+向量化(避免 V1 的污染)
- SemanticIndexer 职责独立,不做图结构化
- HyperIndexer 编排双线:读文件→结构化→分块(同时产出 Nodes/Edges)→分流到 SemanticIndexer + GraphIndexer

```go
// HyperIndexer 复合索引器:统一入口,协调语义线+关系线。
//
// 工作流:
//   1. document.Open(path) → RawDoc(读文件+归一化)
//   2. core.NewStructuredDoc(raw) → StructuredDoc(结构化容器,类型定义在 core 包)
//   3. chunker.New(raw) → Chunker; chunker.Chunk(raw) → ChunkResult(含 Chunks/Nodes/Edges)
//   4. doc.SetChunks(result.Chunks); doc.SetNodes(result.Nodes); doc.SetEdges(result.Edges)
//   5. semantic.Save(doc)(语义线:向量化+写入 VectorStore,路由由 SemanticIndexer.Save 实现)
//   6. graph.Save(doc)(关系线:Region→Document CONTAINS 边+实体/关系写入 GraphStore,路由由 GraphIndexer.Save 实现)
type HyperIndexer struct {
    chunker   chunker.Chunker  // 分块器(按 RawDoc.Type 路由,同时产出 Nodes/Edges)
    semantic  Indexer          // 语义线(必注入,通常由 NewSemanticIndexer 创建)
    graph     Indexer          // 关系线(可选,为 nil 则不启用图功能;通常由 NewGraphIndexer 创建)
    logger    logging.Logger   // 日志记录器(必注入,禁止内部创建)
}

// AddFile 实现 Indexer 接口(对外统一入口)
func (h *HyperIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
    // 1. 读文件+归一化
    raw, err := document.Open(filePath)
    if err != nil { return nil, err }

    // 2. 结构化
    doc, err := core.NewStructuredDoc(raw)
    if err != nil { return nil, err }

    // 3. 分块(同时产出 Nodes/Edges)
    chunkerImpl := h.chunker
    if chunkerImpl == nil {
        chunkerImpl, err = chunker.New(raw)
        if err != nil { return nil, err }
    }
    result, err := chunkerImpl.Chunk(raw)
    if err != nil { return nil, err }

    // 4. 把 Chunks/Nodes/Edges 完整消费进 StructuredDoc
    doc.SetChunks(result.Chunks)
    doc.SetNodes(result.Nodes)
    doc.SetEdges(result.Edges)

    // 5. 语义线:向量化+写入 VectorStore(存储路由由 SemanticIndexer.Save 实现)
    if store, ok := h.semantic.(IndexerStore); ok {
        if err := store.Save(ctx, doc); err != nil { return nil, err }
    }

    // 6. 关系线:图结构化(存储路由由 GraphIndexer.Save 实现,如果 GraphIndexer 存在)
    //    关系线失败不阻塞语义线,仅记录警告
    if h.graph != nil {
        if store, ok := h.graph.(IndexerStore); ok {
            if err := store.Save(ctx, doc); err != nil {
                h.logger.Warn("关系线保存失败: %v", err)
            }
        }
    }

    chunks := make([]*core.Chunk, 0, len(result.Chunks))
    for i := range result.Chunks { chunks = append(chunks, &result.Chunks[i]) }
    return chunks, nil
}

// Search 实现 Indexer 接口(双线融合:语义 Hit + 图 Hit → 融合 Hit)
func (h *HyperIndexer) Search(ctx context.Context, query core.Query) (*core.Hit, error) {
    // 1. 语义线:SemanticIndexer.Search → Hit{Chunks: [...]}
    semHit, err := h.semantic.Search(ctx, query)
    if err != nil { return nil, err }

    // 2. 关系线:GraphIndexer.SearchGraph → Hit{Nodes: [...], Edges: [...]}
    //    (如果 graph 为 nil,跳过图检索;图检索失败不阻塞语义结果,仅记录警告)
    var graphHit *core.Hit
    if h.graph != nil {
        if gs, ok := h.graph.(GraphSearcher); ok {
            graphHit, err = gs.SearchGraph(ctx, query)
            if err != nil {
                h.logger.Warn("图检索失败: %v", err)
            }
        }
    }

    // 3. 融合:语义 Hit + 图 Hit → 综合 Hit(详见 §7.12)
    //    Chunks 来自语义线,Nodes/Edges 来自关系线
    //    Score = 语义 Score + 图 Score 加权(RRF 融合)
    return result.RRF(
        result.NewSource("semantic", 1.0, semHit),
        result.NewSource("graph", 0.7, graphHit),  // graphHit 为 nil 时 RRF 内部跳过
    )
}

// Tree 实现 TreeViewBuilder 接口
func (h *HyperIndexer) Tree(ctx context.Context, regionID string) (*core.TreeNode, error) {
    // 1. 从 GraphIndexer 取得 Region→Document 树
    //    GraphIndexer 不实现 TreeViewBuilder,但提供未导出的 regionTree 方法供 HyperIndexer 内部组合。
    g, ok := h.graph.(*GraphIndexer)
    if !ok { return nil, fmt.Errorf("关系线未提供 Region→Document 树能力") }
    tree, err := g.regionTree(ctx, regionID)
    if err != nil { return nil, err }

    // 2. 为每个 Document 节点从 VectorStore 补齐 Chunk 子节点
    admin, ok := h.semantic.(IndexerAdmin)
    if !ok { return nil, fmt.Errorf("语义线未实现管理接口") }
    h.populateChunks(ctx, tree, admin)
    return tree, nil
}

// NewQuery 实现 Indexer 接口
func (h *HyperIndexer) NewQuery(terms string) core.Query {
    return h.semantic.NewQuery(terms)
}

// Name 实现 Indexer 接口
func (h *HyperIndexer) Name() string { return "hyper" }
```

**HyperIndexer 的扩展能力**(通过 type-assert):
- `hyper.(IndexerAdmin)` → 委托 semantic
- `hyper.(IndexerCloser)` → 关闭 semantic + graph
- `hyper.(TreeViewBuilder)` → HyperIndexer 自身实现
- `hyper.(GraphSearcher)` → 委托 graph

---

## 4. 接口分配表

| 方法          | 接口            | SemanticIndexer | GraphIndexer | HyperIndexer | 说明                                                                 |
| ------------- | --------------- | :-------------: | :----------: | :----------: | -------------------------------------------------------------------- |
| `Name`        | Indexer         |        ✓        |      ✓       |      ✓       | 元信息                                                               |
| `AddFile`     | Indexer         |        ✓        |      ✓       |      ✓       | 文件索引                                                             |
| `Search`      | Indexer         |        ✓        |      ✓       |      ✓       | 语义检索                                                             |
| `NewQuery`    | Indexer         |        ✓        |      ✓       |      ✓       | 查询构造                                                             |
| `Save`        | IndexerStore    |        ✓        |      ✓       |      ✗       | 存储路由(各自实现,被 Hyper 组合)                                     |
| `List`        | IndexerAdmin    |        ✓        |      ✗       |   ✓(委托)    | 浏览:基于 VectorStore 的 Chunk 数据                                  |
| `GetChunks`   | IndexerAdmin    |        ✓        |      ✗       |   ✓(委托)    | 浏览:基于 VectorStore 的 Chunk 数据                                  |
| `Count`       | IndexerAdmin    |        ✓        |      ✗       |   ✓(委托)    | 统计:基于 VectorStore 的 Chunk 数据                                  |
| `Remove`      | IndexerAdmin    |        ✓        |      ✗       |   ✓(委托)    | 维护:删除;由语义线作为 Chunk 管理来源                                |
| `Clear`       | IndexerAdmin    |        ✓        |      ✗       |   ✓(委托)    | 维护:清空;由语义线作为 Chunk 管理来源                                |
| `Close`       | IndexerCloser   |        ✓        |      ✓       |   ✓(委托)    | 资源管理                                                             |
| `Tree`        | TreeViewBuilder |        ✗        |      ✗       |      ✓       | 导航:GraphStore 提供 Region→Document,VectorStore 补齐 Document→Chunk |
| `SearchGraph` | GraphSearcher   |        ✗        |      ✓       |   ✓(委托)    | 图查询                                                               |

**说明**:
- SemanticIndexer / GraphIndexer 都实现 Indexer + IndexerStore,可独立使用或被 HyperIndexer 组合
- GraphIndexer 不实现 IndexerAdmin:GraphStore 不保存 Chunk,Chunk 管理由 SemanticIndexer 通过 VectorStore 负责
- HyperIndexer 不实现 IndexerStore(它是组合器,通过调用 semantic.Save + graph.Save 实现存储路由)
- HyperIndexer 的 Admin/Closer/GraphSearcher 通过委托实现;TreeViewBuilder 由 HyperIndexer 自身实现,组合 GraphIndexer 的 Region→Document 树与 SemanticIndexer 的 Document→Chunk 数据
- GraphIndexer 不实现 TreeViewBuilder,但提供未导出的 `regionTree` 方法供 HyperIndexer 内部组合;对外只暴露 GraphSearcher 能力

**已删除的 V1 方法**:
- `Add(ctx, content string)`:接收字符串输入,违反"绝对路径"约束,不支持溯源
- `Type()`:与 `Name()` 语义重复
- `StoreChunk(chunk)`:被 `Save(doc)` 取代——Save 接收整个 StructuredDoc,由各 Indexer 内部从 `doc.Chunks()` 读取并路由到各自存储

---

## 5. 接口定义位置

所有 6 个 Indexer 接口定义在 `indexer/interfaces.go`,不放在 `core` 包。

`core` 包只保留数据类型:`Chunk` / `Hit` / `Query` / `Node` / `Edge` / `TreeNode` / `FilterCondition` / `Vector` / `StructuredDoc` 等。

组件接口定义在各自包:
- `chunker.Chunker` 定义在 `chunker` 包(返回 `ChunkResult`,含 Chunks/Nodes/Edges)
- `llm.Summarizer` 定义在 `llm` 包

---

## 6. 包结构调整

按最终接口职责与依赖关系,各文件/包职责划分如下:

| 文件/包                             | 职责                       | 说明                                                                                    |
| ----------------------------------- | -------------------------- | --------------------------------------------------------------------------------------- |
| `indexer/interfaces.go`             | 6 个 Indexer 接口          | Indexer / IndexerStore / IndexerAdmin / IndexerCloser / TreeViewBuilder / GraphSearcher |
| `indexer/semantic.go`               | SemanticIndexer 实现       | 分块、向量化、语义检索、Chunk 管理                                                      |
| `indexer/graph.go`                  | GraphIndexer 实现          | 图结构化、Region→Document CONTAINS 边、图检索                                           |
| `indexer/hyper.go`                  | HyperIndexer 实现          | 编排语义线+关系线,实现 TreeViewBuilder                                                  |
| `indexer/utils.go`(或 `vectors.go`) | 向量化辅助                 | 多维度向量 ID 规则、批量 Upsert 等                                                      |
| `core/structured_doc.go`            | StructuredDoc 类型与构造器 | 取代 `structurizer` 包,供 `indexer` 包引用                                              |
| `core/embedder.go`                  | Embedder 接口              | 纯向量计算,不耦合 Chunk                                                                 |
| `chunker/*.go`                      | Chunker 实现               | 分块并产出 Chunks/Nodes/Edges                                                           |
| `llm/*.go`                          | Summarizer 实现            | 可选注入 SemanticIndexer                                                                |

**依赖清理**:
- `indexer` 包不再导入 `structurizer` 包;`StructuredDoc` 统一使用 `core.StructuredDoc`。
- `indexer` 包不再导入 `extractor` 包;实体/关系提取由 Chunker 负责。
- 删除所有 `v2_*.go` 文件;V1 文件(semantic.go / graph.go / hyper.go 等)原地升级。

---

## 7. 关键设计决策

### 7.1 为什么 LLM 不剥离?

LLM 调用是 Indexer 的核心能力,不是附属品。调用方 `idx.AddFile(ctx, filePath)` 一行完成索引,这是 Indexer 的核心价值。V2 把 LLM 剥离后,调用方必须手动编排 Chunker→Index 两步,API 复杂度爆炸。

成熟库(LlamaIndex、LangChain)都提供一站式 Indexer 作为主入口,而非只留分步 Pipeline。需要分步控制的场景(如自定义 Chunker/Summarizer)走独立接口,不污染 Indexer。

### 7.2 为什么只支持文件输入?

V1 的 `Add(ctx, content string)` 接收字符串输入,违反"索引系统必须使用绝对路径"的硬性约束:

- 字符串无法溯源,丢失 `source_file` / `region_id` 等元数据
- 字符串输入不经过 `document.Open`,绕过文件归一化流程(4 类归一化策略失效)
- 与 Region 层级(基于文件目录推导)冲突——字符串没有目录

V2 删除 `Add`,只保留 `AddFile`,Indexer 职责单一化为"索引文件":
- 强制使用绝对路径(符合硬性约束)
- 保留 `source_file` / `region_id` 等元数据
- 经过 `document.Open` 文件归一化流程
- 支持 Region 层级(基于文件目录推导)

字符串内容如何索引?调用方需先把字符串写入临时文件(用 `os.CreateTemp` + 绝对路径),再调用 `AddFile`。这迫使调用方显式管理"内容落盘",避免索引系统承担字符串溯源的复杂度。

### 7.3 为什么 CONTAINS 边只保留 Region→Document?

Region 作为区域节点的核心价值是组织 `Region → Document` 的层级关系,CONTAINS 边承担这一职责。Document 节点由 Chunker 在分块过程中生成,GraphIndexer 负责创建 Region 节点以及 `Region --CONTAINS--> Document` 边。

`Document → Chunk` 的层级不在 GraphStore 中体现,原因如下:

- **避免同一份数据重复写入**:Chunk 的完整内容、元数据已通过 SemanticIndexer 写入 VectorStore;若再在 GraphStore 中以 Node 形式写入,造成存储冗余且维护成本翻倍。
- **职责边界清晰**:GraphStore 只保存 Node(Region/Document/实体)与 Edge(层级/语义关系),不保存 Chunk;Chunk 是语义线的数据,由 VectorStore 管理。
- **视图层动态组装**:TreeViewBuilder 在 HyperIndexer 中实现,先从 GraphIndexer 取得 `Region → Document` 树,再通过 `SemanticIndexer.GetChunks` 从 VectorStore 为每个 Document 补齐 Chunk 子节点,形成完整的 `Region → Document → Chunk` 视图树。

因此,CONTAINS 边仅用于 `Region → Document` 层级,Document→Chunk 层级由 TreeViewBuilder 通过 VectorStore 动态组装。

### 7.4 为什么 TreeViewBuilder 和 SearchGraph 独立接口?

`SearchGraph` 只有 GraphIndexer 真正实现,SemanticIndexer 不维护 GraphStore。`TreeViewBuilder` 只有 HyperIndexer 真正实现,因为它需要同时访问 GraphStore 的 `Region → Document` 层级和 VectorStore 的 `Document → Chunk` 数据。如果放入通用接口:

- SemanticIndexer 被迫实现空方法(返回 nil)——接口污染
- 调用方无法通过接口类型判断是否支持该能力
- 违反接口分离原则(ISP)

独立接口后:
- 调用方按需 type-assert:
  - `if g, ok := idx.(GraphSearcher); ok { ... }`
  - `if t, ok := idx.(TreeViewBuilder); ok { ... }`
- 语义清晰:`idx.(TreeViewBuilder)` 比 `idx.(IndexerAdmin).Tree()` 更明确
- 符合 Go 小接口组合的风格(参考 `io.Reader`/`io.Writer`/`io.Closer`/`io.Seeker`)

### 7.5 为什么构造函数返回 `(Indexer, error)`?

- 返回**接口**而非具体类型:调用方依赖抽象,不依赖具体实现
- 返回 **error**:对 db/embedder/graphDB 做 nil 检查,避免后续 panic
- 调用方代码:`idx, err := indexer.New(...); if err != nil { return err }`

### 7.6 为什么保留多维度向量索引?

SemanticIndexer 为每个 Chunk 生成 1~3 条向量,对应 Content/Title/Summary 三个维度:

- 主向量 ID 为 `chunkID`,对应 **Content**
- 辅助向量 ID 为 `chunkID:title`,对应 **Title**
- 辅助向量 ID 为 `chunkID:summary`,对应 **Summary**

不同查询命中不同维度:

- 短关键词查询命中 **Title** 维度
- 概述性查询命中 **Summary** 维度
- 具体内容查询命中 **Content** 维度
- 任一维度命中都能定位同一 Chunk,提高召回率

**向量化职责拆分**:

- **Embedder 只做纯向量计算**:接收文本,返回向量;不感知 Chunk、Chunker 或维度策略;不增加 `CalcChunk` 等与 Chunk 耦合的方法。
- **SemanticIndexer 负责维度策略**:从 `chunk.Content`、`chunk.Title`、`chunk.Summary` 提取文本,分别调用 `embedder.CalcText(text)`;字段为空或没有 Summarizer 时,跳过对应维度。
- **Chunker 负责填充 Chunk 字段**:包括 `Title`、`Summary`、`Content`。
- **Summarizer 可选补充**:当 Chunk 缺少 Title/Summary 时,由注入到 SemanticIndexer 的 Summarizer 补充;没有 Summarizer 则跳过对应维度(详见 §7.9)。

向量化逻辑放在 `indexer/utils.go`(或 `vectors.go`),不放在独立的 `v2_helpers.go`。

### 7.7 为什么 GraphIndexer 职责独立?

V1 GraphIndexer 2394 行,内含大量分块+向量化逻辑,与图结构化逻辑耦合——这是"语义化索引污染"。

**V2 设计**:GraphIndexer 职责单一化为"图结构化":
- 不做分块(Chunker 由 HyperIndexer 调用,同时产出 Chunks/Nodes/Edges;独立使用时 GraphIndexer.AddFile 内部调用 Chunker)
- 不做向量化(SemanticIndexer 负责)
- 不内嵌 SemanticIndexer(避免污染)
- 不再依赖 `extractor` 包,V1 残留的 LLM 实体提取、旧 Add/Index 方法全部移除
- 只持有 GraphStore,从 doc.Nodes()/Edges() 读取实体/关系并写入;Chunk 不作为 Node 写入 GraphStore
- 不实现 IndexerAdmin:Chunk 的浏览/统计/删除由 SemanticIndexer 通过 VectorStore 负责;GraphIndexer 只维护图数据

**双线结合在 HyperIndexer**:
- 语义线:SemanticIndexer.Save → 从 doc.Chunks() 提取文本,向量化,写入 VectorStore
- 关系线:GraphIndexer.Save → 从 doc.Nodes()/Edges() 读取实体/关系,维护 Region→Document 的 CONTAINS 边,写入 GraphStore
- HyperIndexer 编排:先语义后关系,通过 StructuredDoc 共享 Chunker 产出的 Chunks/Nodes/Edges;关系线失败不阻塞语义线

### 7.8 为什么 Save 不返回 Chunks,且 HyperIndexer 必须完整消费 ChunkResult?

Save 的本质是"各 Indexer 保存各自需要的数据的入口":
- SemanticIndexer.Save 从 doc.Chunks() 提取文本,生成向量,保存到 VectorStore
- GraphIndexer.Save 从 doc.Nodes()/Edges() 读取实体/关系,维护 Region→Document 的 CONTAINS 边,保存到 GraphStore;Chunk 不作为 Node 写入 GraphStore
- Chunks 只是中间产物,不是"保存的数据"

**HyperIndexer 负责完整消费 ChunkResult**:
1. `chunker.Chunk(raw)` → `ChunkResult{Chunks, Nodes, Edges}`
2. `doc.SetChunks(result.Chunks)` / `doc.SetNodes(result.Nodes)` / `doc.SetEdges(result.Edges)` → 把三类数据全部存入 StructuredDoc
3. 各 Indexer 从 `doc.Chunks()` / `doc.Nodes()` / `doc.Edges()` 读取,处理各自的数据

**关系线失败不阻塞语义线**:HyperIndexer.AddFile 中,`graph.Save(doc)` 失败时只记录警告,不返回 error;语义线已成功写入 VectorStore 的结果仍然可用。这样避免图索引的偶发问题(如网络抖动、schema 冲突)影响语义检索可用性。

这样 Save 只返回 error,职责清晰。**存储路由的关键**:各 Indexer 的 Save 实现自动路由到各自存储,HyperIndexer 只需调用 `semantic.Save(doc)` + `graph.Save(doc)` 即可完成双线写入。

### 7.9 为什么 Summarizer 传 string 而非 Chunk?

Summarizer 注入到 SemanticIndexer,用于补充 Chunk 缺失的 Title/Summary,从而保证多维度向量索引(详见 §7.6)能生成完整向量。

Summarizer 的本质是"输入文本,输出 title+summary":
- 传 string 与 `core.Chunk` 解耦,可用于任何需要标题+摘要的场景
- 调用方负责把结果赋值给 `chunk.Title/Summary`
- 符合 Go 小接口原则——Summarizer 只做"文本→标题+摘要",不关心调用方是谁

**条件式使用**:只有当 Chunk 的 title/summary 为空且 Summarizer 存在时,才调用 Summarizer 补充。这保证了:
- Chunker 能提取到的 title/summary(如 Markdown heading)直接使用,不浪费 LLM 调用
- Chunker 提取不到的(如纯文本),用 Summarizer(LLM)补充
- 没有 Summarizer 或补充失败时,SemanticIndexer 直接跳过对应维度的向量,不会强制填充空 title/summary

### 7.10 为什么 Chunker 取代 Extractor?

旧设计把"实体/关系提取"作为独立 `Extractor` 接口注入 GraphIndexer。当前设计中,Chunker 在分块的同时即产出结构化的 Nodes/Edges,Extractor 不再作为独立组件存在。

**Chunker 取代 Extractor 的理由**:
- **避免重复解析**:Chunker 已经完整解析了文档 AST/结构(heading 树、代码 AST、数据路径),再交给 Extractor 重新遍历一次是重复工作。
- **确定性结构无需 LLM**:heading 层级、代码包含关系、数据路径前缀等结构关系用规则即可精确提取,成本低、稳定性高。
- **Chunks 与 Nodes 天然对齐**:Chunker 在产出 Chunk 的同时生成对应 Node,`SourceChunkIDs` 绑定精确;独立 Extractor 需要再次关联 Chunk 与 Node,容易错位。
- **简化 API**:调用方无需理解 Chunker/Extractor/Indexer 三者的协作,HyperIndexer 内部只调用 Chunker,再把结果分流到 SemanticIndexer + GraphIndexer。

**边界说明**:
- 当前 Chunker 产出的是**确定性结构**(Region→Document 的 CONTAINS、代码中的 BELONGS_TO/CALLS/INHERITS/IMPLEMENTS 等),不依赖 LLM。
- 其中 `Region --CONTAINS--> Document` 边由 GraphIndexer 在 Save 时创建并维护;`Document → Chunk` 层级不写入 GraphStore,由 TreeViewBuilder 在视图层动态组装。
- 如果未来需要非确定性语义实体(如"这段代码调用了哪个外部 API""图片里有什么内容"),可以在 Chunker 之上增加可选的 LLM 增强层,但该层属于 Chunker 包的内部扩展,不暴露为 GraphIndexer 的独立依赖。
- GraphIndexer 不再依赖 `extractor` 包,V1 残留的 extractor 注入、LLM 实体提取逻辑全部移除。

### 7.11 GraphIndexer 独立使用:纯图谱模式

GraphIndexer **一般不独立使用**——双线结合的契机在 HyperIndexer,生产场景应通过 HyperIndexer 协调语义线+关系线。

但 GraphIndexer **仍然实现完整的 Indexer 接口**,允许独立使用,代价是降级为**纯图谱模式**:

| 模式            | AddFile 行为                                                    | Search 行为                    | Node 与 Chunk 关联                  | Chunk 是否写入 GraphStore                                   |
| --------------- | --------------------------------------------------------------- | ------------------------------ | ----------------------------------- | ----------------------------------------------------------- |
| 通过 Hyper 使用 | HyperIndexer 调用 `graph.Save(doc)`,doc 已含 Chunks/Nodes/Edges | 委托 SemanticIndexer(语义检索) | Node.SourceChunkIDs 完整            | 否,Chunk 由 VectorStore 管理                                |
| 独立使用        | GraphIndexer.AddFile 内部使用 Chunker 分块,调用 Save 写入图数据 | 图检索(返回 Node 相关的 Hit)   | Node.SourceChunkIDs 指向内部 Chunks | 否,Chunk 仍由 VectorStore 管理,但独立模式不写入 VectorStore |

**独立使用的语义**:
- GraphIndexer.AddFile 内部使用 Chunker 分块,产出 Chunks/Nodes/Edges,然后调用自身的 `Save(ctx, doc)` 写入图数据。
- **不做向量化**——VectorStore 不写入,无语义检索能力。
- Node.SourceChunkIDs 仍然填充(指向内部 Chunks),但这些 Chunks 未入 VectorStore,无法通过语义检索反查。
- GraphStore 只保存 Region/Document/实体 Node 与 Edge;Chunk 不作为 Node 写入 GraphStore。
- Search 只能走图遍历(`SearchGraph` 能力),返回的 Hit 缺少向量分数。

**结论**:GraphIndexer 实现 Indexer 接口是为了"可独立使用",但独立使用是**降级模式**——只作为纯图谱使用,无语义检索能力。生产场景应通过 HyperIndexer 使用,获得双线协同能力。

### 7.12 Hit 与 StructuredDoc 的对称性:存取镜像

**核心洞察**:`StructuredDoc` 是**索引过程的统一容器**(Indexer.Save 接收),`Hit` 是**检索过程的统一容器**(Indexer.Search 返回)。两者必须对称——都持有 Chunks/Nodes/Edges 三类数据。

**V1/V2 错误的分离设计**:
- `Hit` 只持有单个 Chunk 的内容(Content/Title/Summary)
- `GraphResult` 只持有 Nodes/Edges
- 类型不统一,`result.RRF` 只能融合 `[]Hit`,无法融合图检索结果
- `HyperIndexer.Search` 只能委托 SemanticIndexer,丢失图命中能力

**最终设计**:`Hit` 重构为持有 Chunks/Nodes/Edges 三类集合的容器,与 `StructuredDoc` 形成存取镜像。

```go
// ChunkHit 语义命中的分片:嵌入 *Chunk + 命中信息(分数/维度)。
type ChunkHit struct {
    *Chunk                // 嵌入,客户端可直接访问 hit.Chunks[i].Content
    Score float32         // 该分片的相关性分数
    Dim   string          // 命中维度:"content" / "title" / "summary"
}

// NodeHit 图命中的实体:嵌入 *Node + 分数。
type NodeHit struct {
    *Node
    Score float32
}

// EdgeHit 图命中的关系:嵌入 *Edge + 分数。
type EdgeHit struct {
    *Edge
    Score float32
}

// Hit 检索结果容器:持有 Chunks/Nodes/Edges 三类命中数据。
//
// 设计对称性:
//   - StructuredDoc 是索引过程容器(Indexer.Save 接收)
//   - Hit 是检索过程容器(Indexer.Search 返回)
//   - 两者都持有 Chunks/Nodes/Edges 三类数据
//
// 融合:
//   - SemanticIndexer.Search → Hit{Chunks: [...]}
//   - GraphIndexer.SearchGraph → Hit{Nodes: [...], Edges: [...]}
//   - HyperIndexer.Search → Hit{Chunks: [...], Nodes: [...], Edges: [...]}(双线融合)
type Hit struct {
    Query  core.Query    // 触发本次检索的查询对象
    Score  float32       // 综合分数(RRF 融合后的总分)
    Chunks []ChunkHit    // 语义命中的分片(SemanticIndexer 填充)
    Nodes  []NodeHit     // 图命中的实体(GraphIndexer 填充)
    Edges  []EdgeHit     // 图命中的关系(GraphIndexer 填充)
}
```

**为什么用嵌入式 `*Chunk` 类型而非其他方案**:

| 方案                                  | 优雅度 | 问题                                           |
| ------------------------------------- | ------ | ---------------------------------------------- |
| Chunk 加 Score/Dim 字段               | ✗      | 破坏 Chunk 纯净性(索引侧 Chunk 不需要这些字段) |
| `map[chunkID]float32` 关联分数        | ✗      | 客户端要先拿 ID 再查 map,体验差                |
| 泛型 `ScoredEntry[T]`                 | △      | Go 泛型 JSON 不友好,访问需 `.Entry.Content`    |
| **嵌入式 `ChunkHit/NodeHit/EdgeHit`** | ✓      | 最自然——3 个小类型,职责清晰                    |

**嵌入式设计的 7 个优势**:
1. **对称性**:与 `StructuredDoc` 的 `Chunks()/Nodes()/Edges()` 三类集合完全对称
2. **纯净性**:`Chunk/Node/Edge` 保持纯净,索引侧使用不受影响(无冗余字段)
3. **嵌入式设计**:通过 `*Chunk` 嵌入,客户端访问无间接——`hit.Chunks[i].Content`(而非 `hit.Chunks[i].Chunk.Content`)
4. **细粒度分数**:每个 Chunk 有自己的 Score(支持细粒度排序),`Hit.Score` 是综合分(RRF 融合后)
5. **维度信息局部化**:`Dim` 字段只在 `ChunkHit` 上有意义(Node/Edge 无维度概念),不污染其他类型
6. **JSON 友好**:嵌入式结构 JSON 序列化自然扁平化,无嵌套层级
7. **类型安全**:相比 `map[string]any`,字段访问是类型安全的

**Fusion 重构**:
- `result.RRF(sources ...*FusionSource) (*core.Hit, error)`——接收多个 `*Hit`,返回融合后的 `*Hit`
- 内部按 Chunks/Nodes/Edges 三类分别 RRF 融合,按 ID 去重
- `Hit.Score = Σ(各类 Score 加权平均)`

**HyperIndexer.Search 双线融合流程**:
1. `semantic.Search(ctx, q)` → `*Hit{Chunks: [...]}`
2. `graph.SearchGraph(ctx, q)` → `*Hit{Nodes: [...], Edges: [...]}`(graph 为 nil 则跳过)
3. `result.RRF(semantic, graph)` → 融合后的 `*Hit{Chunks, Nodes, Edges, Score}`

**`GraphResult` 类型删除**——`SearchGraph` 与 `Search` 统一返回 `*Hit`,消除了类型分离导致的融合障碍。

---

## 8. 差异说明表

| 维度                              | 旧设计(V1 / V2 残留 / 当前代码临时状态)                                      | 当前最终设计                                                                                                                                                     | 影响范围                                                          |
| --------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| **Embedder 与 Chunk 耦合**        | `Embedder.Calc(chunk *Chunk)` / `Bulk(chunks []*Chunk)`, Embedder 感知 Chunk | Embedder 只接收文本/图片,做纯向量计算;不增加 `CalcChunk` 方法                                                                                                    | `core.Embedder`, `indexer/semantic.go`                            |
| **多维度向量生成**                | 由 Embedder 或辅助函数统一计算                                               | SemanticIndexer 从 `chunk.Content/Title/Summary` 提取文本,分别调用 `embedder.CalcText`;主向量 ID=`chunkID`,title ID=`chunkID:title`,summary ID=`chunkID:summary` | `indexer/semantic.go`                                             |
| **Summarizer 行为**               | 缺少 title/summary 时保持为空,或强制补充                                     | 可选注入;存在且字段为空时补充;没有则跳过对应维度                                                                                                                 | `indexer/semantic.go`, `llm` 包                                   |
| **GraphStore 中 Chunk 存储**      | Chunk 作为 Label="Chunk" 的 Node 写入 GraphStore                             | GraphStore 只保存 Region/Document/实体 Node 与 Edge;Chunk 不写入 GraphStore                                                                                      | `indexer/graph.go`                                                |
| **CONTAINS 边范围**               | `Region→Document→Chunk` 全部用 CONTAINS                                      | 仅 `Region→Document` 用 CONTAINS;`Document→Chunk` 在 TreeViewBuilder 中通过 VectorStore 组装                                                                     | `indexer/graph.go`, `indexer/hyper.go`                            |
| **导航树接口**                    | `TreeBuilder` 由 GraphIndexer 实现,Tree 方法返回三层树                       | 更名为 `TreeViewBuilder`,由 HyperIndexer 实现;GraphIndexer 只提供 Region→Document 树,Chunk 子节点从 VectorStore 补齐                                             | `indexer/interfaces.go`, `indexer/hyper.go`                       |
| **GraphIndexer 依赖**             | 依赖 `extractor` 包,保留 V1 LLM 实体提取逻辑                                 | 不再依赖 `extractor`;V1 残留方法全部移除;独立使用时 AddFile 内部用 Chunker 分块并调用 Save                                                                       | `indexer/graph.go`                                                |
| **HyperIndexer 错误处理**         | `graph.Save` 失败直接返回 error,阻塞语义线                                   | `graph.Save` 失败仅记录警告,不阻塞语义线                                                                                                                         | `indexer/hyper.go`                                                |
| **HyperIndexer 消费 ChunkResult** | 只消费 Chunks,忽略 Nodes/Edges(或分步处理)                                   | 必须完整消费 `Chunks/Nodes/Edges`,分别写入 StructuredDoc                                                                                                         | `indexer/hyper.go`                                                |
| **SemanticIndexer.AddFile**       | 使用 `GetFileChunks` 等旧分块方式                                            | 使用 Chunker 分块,产出 Chunks/Nodes/Edges                                                                                                                        | `indexer/semantic.go`                                             |
| **StructuredDoc 位置**            | 类型定义在 `core`,但构造器在 `structurizer` 包,接口文件引用 `structurizer`   | `StructuredDoc` 类型与构造均在 `core` 包;`indexer` 包不再引用 `structurizer`                                                                                     | `core/structured_doc.go`, `indexer/interfaces.go`, `indexer/*.go` |
| **代码注释**                      | 中英混合                                                                     | 全部使用中文注释                                                                                                                                                 | 所有相关文件                                                      |

---

## 9. 实施计划

按以下顺序推进代码改造,确保每一步都有对应测试覆盖:

### 9.1 core 包调整

1. 在 `core/structured_doc.go` 中补充 `NewStructuredDoc(raw document.RawDoc) (StructuredDoc, error)` 构造函数,使 `indexer` 包不再依赖 `structurizer`。
2. 调整 `core.Embedder` 接口:移除 `Calc(chunk *Chunk)` 和 `Bulk(chunks []*Chunk)` 等与 Chunk 耦合的方法,仅保留 `CalcText(text string)`、`CalcImage(data []byte)` 等纯向量计算方法;调用方(SemanticIndexer)负责从 Chunk 提取文本。

### 9.2 SemanticIndexer 改造

1. `AddFile` 改为使用 Chunker 分块(`chunker.Chunk(raw)`),产出 `ChunkResult{Chunks, Nodes,Edges}`。
2. 在 `Save` 中遍历 `doc.Chunks()`,从 `chunk.Content/Title/Summary` 提取文本,分别调用 `embedder.CalcText` 生成向量;主向量 ID=`chunkID`,title 维度 ID=`chunkID:title`,summary 维度 ID=`chunkID:summary`;字段为空则跳过对应维度。
3. 支持可选注入 `llm.Summarizer`;当 `chunk.Title` 或 `chunk.Summary` 为空且 Summarizer 存在时调用,否则跳过。
4. 移除对 `GetFileChunks` 等旧分块入口的依赖。

### 9.3 GraphIndexer 改造

1. 移除 `extractor` 包导入及 V1 残留的 LLM 实体提取方法(`llmIndex`、text2Cypher 中依赖的内部 LLM 等按实际情况清理)。
2. 移除 `saveChunk` 及相关逻辑:Chunk 不再作为 Node 写入 GraphStore。
3. `Save` 中只处理 `doc.Nodes()` / `doc.Edges()`,维护 `Region→Document` 的 CONTAINS 边;Document 节点由 Chunker 生成,Region 节点由 GraphIndexer 根据文件路径创建。
4. GraphIndexer 不再实现 `IndexerAdmin`;`Remove` / `Clear` / `List` / `GetChunks` / `Count` 等 Chunk 管理方法由 SemanticIndexer 通过 VectorStore 实现。GraphIndexer 可保留图数据清理的内部方法,但不在 `IndexerAdmin` 接口中暴露。
5. 独立使用模式:`AddFile` 内部调用 Chunker 分块,构造 `StructuredDoc`,然后调用 `Save(ctx, doc)`。

### 9.4 HyperIndexer 改造

1. `AddFile` 完整消费 `ChunkResult` 的 `Chunks/Nodes/Edges`,写入 `StructuredDoc`。
2. 关系线 `graph.Save(doc)` 失败时记录警告,不返回 error,不阻塞语义线结果。
3. 实现 `TreeViewBuilder` 接口:
   - 从 GraphIndexer 取得 `Region→Document` 树;
   - 对每个 Document 节点,通过 `SemanticIndexer.(IndexerAdmin).GetChunks` 从 VectorStore 读取 Chunk 列表并挂载为子节点。
4. 更新扩展接口委托:`TreeViewBuilder` 由 HyperIndexer 自身实现;内部通过 GraphIndexer 的 `regionTree` 方法取得 Region→Document 树,再补齐 Chunk 子节点,不再把 TreeViewBuilder 接口整体委托给 graph。

### 9.5 接口与包结构调整

1. `indexer/interfaces.go`:
   - 将 `TreeBuilder` 重命名为 `TreeViewBuilder`,注释说明由 HyperIndexer 实现;
   - `IndexerStore.Save` 参数使用 `core.StructuredDoc`;
   - 移除 `structurizer` 包导入。
2. 所有相关文件代码注释改为中文。

### 9.6 测试与验证

1. 单元测试:
   - SemanticIndexer 多维度向量 ID 规则(`chunkID:title` / `chunkID:summary`);
   - GraphIndexer 不写入 Chunk Node;
   - GraphIndexer 只维护 Region→Document CONTAINS 边;
   - HyperIndexer 关系线失败不阻塞语义线;
   - TreeViewBuilder 正确组装三层视图树。
2. 集成测试:
   - HyperIndexer.AddFile → Search → Tree 端到端流程;
   - GraphIndexer 独立使用模式(AddFile → SearchGraph)。
3. 迁移检查:
   - 确认 `structurizer` 包不再被 `indexer` 包引用;
   - 确认 `extractor` 包不再被 `indexer` 包引用;
   - 确认 `v2_*.go` 文件已全部删除。
