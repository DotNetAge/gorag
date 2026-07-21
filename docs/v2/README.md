# GoRAG V2

GoRAG 的核心理念：为 LLM 提供「分析」与「抽象」的数据基础。

GoRAG 是双路结构：一路语义化（分片），一路关系化（图谱）。

> V2 的总体方向是**做减法**：一次性清理 V1 中的过度设计与错误设计，只保留最实用的内容。本文档既是设计稿，也是开工清单——所有接口、数据模型、迁移步骤、验收标准均给出明确定义。

---

## 1. 设计原则

1. **做减法优先**：每个被保留的接口、字段、模块必须能用一句话说清存在的理由；说不清就删。
2. **减法不等于无脑删**：核心功能（如 `Region`/`Tree()`/LLM 调用/compress/text2Cypher）即使在 V1 中设计有问题或暂时未用，V2 也是「保留并重构」而非「删除」，避免丢失应用价值或后续扩展能力。
3. **双线结构是核心**：Chunk 与 Node 必须双向关联——Chunk 通过 `ParentID` 自连结成分块树，Node 通过 `SourceChunkIDs` 反向引用 Chunk；任意 Chunk 可通过 `ChunkID` 直接定位到对应 Node。语义（Chunk）、实体与关系（Node/Edge）必须有机结合，这是本项目区别于其他 RAG 的根本特征。
4. **接口最小化**：接口只暴露「做什么」，不暴露「怎么做」；一个接口一个职责；管理类方法剥离到扩展接口（如 `IndexerAdmin`）。
5. **索引与查询分离**：写入路径与查询路径不共用接口，避免接口膨胀。
6. **LLM 体外化但提供默认实现**：核心包不持有 LLM 客户端，LLM 调用由 `Extractor`/`Chunker` 接口承载；但 `extractor.LLMExtractor` 作为默认应用实现直接提供，调用方开箱即用。
7. **结构化驱动的分块**：分块器必须基于 4 大结构化文档类型（Markdown/数据/图片/代码）的结构特征进行分块，禁止使用 V1 的分词式分块（fixed_size/sentence/paragraph/recursive/semantic/parent_doc）。
8. **绝对路径**：所有引用文件的字段必须是绝对路径，相对路径视为 critical bug。
9. **构造函数返回 error**：所有必传参数非空检查；禁止静默 panic。
10. **注释中文**：代码注释统一使用中文，禁止英文注释。
11. **概念统一**：相似概念统一为单一类型（如 `ChunkNode` 归一化为 `Node`），避免类型膨胀。归一化仅是类型层面的统一，**Chunk 不作为 Node 写入 Graph**，Chunk 只存在于 VectorStore（语义线），实体 Node 存在于 GraphStore（关系线），二者通过 `Node.SourceChunkIDs` 双向关联。
12. **多维度向量索引**：每个 Chunk 必须具备 `Title`/`Summary`/`Content` 三大属性，三者分别向量化写入 VectorStore。**这里的「维度」是数据维度（data dimension），不是向量空间维度（vector space dimension）**——同一个 Chunk 在 VectorStore 中有 3 条向量记录，分别对应 3 个数据维度，但向量空间维度（embedding model 输出维度）三者相同。主向量 ID 为 `chunkID`（对应 Content），辅助向量 ID 为 `chunkID:title` 和 `chunkID:summary`。语义搜索时同时匹配 3 个维度，任一维度命中即可定位同一 Chunk，从而覆盖用户不同的查询方式（短关键词命中 title、概述性查询命中 summary、具体内容查询命中 content），提高召回率。

---

## 2. 模块清单

V2 模块布局（精简后）：

| 模块           | 职责                                                                                       | V1 状态                              |
| -------------- | ------------------------------------------------------------------------------------------ | ------------------------------------ |
| `document`     | 文件 → `RawDoc` 归一化（图片/文档/纯文本/数据 4 类）                                       | 保留，接口化                         |
| `structurizer` | `RawDoc` → `StructuredDoc`（Coder/Datum/Documented 3 类）                                  | 保留，接口化                         |
| `chunker`      | `RawDoc` → `[]Chunk`，按 4 大结构化文档类型分块（Markdown/Datum/Image/Code）               | 重写，删除分词式分块                 |
| `extractor`    | `RawDoc` → `([]Node, []Edge)`（外部扩展接口，`LLMExtractor` 作为默认应用实现）             | 新增为接口                           |
| `embedder`     | `Chunk` 的 `Title`/`Summary`/`Content` 三维度 → `[]Vector`；文本/图片 → `Vector`           | 强化，支持多维度向量化               |
| `indexer`      | `StructuredDoc` → 写入 `VectorStore`（多维度向量）+ `GraphStore`，建立 Chunk↔Node 双向关联 | 简化，剥离 LLM，强化双线，多维度索引 |
| `query`        | 查询对象定义与预处理；`Text2Cypher` 作为预实现类保留                                       | 简化，保留 text2Cypher               |
| `result`       | 结果后处理（reranker/fusion/dedup/compress）                                               | 保留全部                             |
| `store/vector` | 向量存储实现（默认 govector）                                                              | 不变                                 |
| `store/graph`  | 图存储实现（默认 gograph）                                                                 | 不变                                 |
| `store/cache`  | 通用 KV 缓存（默认 bbolt，供 extractor 缓存）                                              | 不变                                 |
| `cmd`          | 命令行工具                                                                                 | 保留                                 |

**已删除**（详见 §5）：

- `indexer/fulltext*.go`
- `store/doc/`（bleve 全文索引）
- `core.FullTextStore`、`core.FullTextSearchResult`
- `core.StructuredDocument`、`core.StructureNode`（被 `StructuredDoc` 接口取代）
- `core.Loader`（与 `document.Open` 重复）
- `query/fulltext.go`、`query/stemming`（全文索引已删）
- `chunker/strategy.go`、`fixed_size.go`、`sentence.go`、`paragraph.go`、`recursive.go`、`semantic.go`、`parent_doc.go`（分词式分块全部废弃）

**保留并重构**（详见 §5）：

- `core.Region`：保留，明确语义为「对应文件目录中 README.md 的知识库分区节点」，作为 Graph 内的分区抽象
- `core.ChunkNode`：归一化为 `Node`（仅类型层面统一，Chunk 不作为 Node 写入 Graph），不再单独定义类型
- `core.Chunk.ParentID`：保留并强化语义为「分块树父节点 ID」，建立可追溯的分块树
- `Indexer.Tree()`：保留，作为 `GraphIndexer` 的重要方法，输出基于 Region 的知识树
- `Indexer.NewQuery()` / `List()` / `GetChunks()` / `StoreChunk()` / `Count()` / `Clear()`：从核心 `Indexer` 接口剥离到 `IndexerAdmin` 扩展接口，保留功能但不污染核心接口
- `LLM` 调用逻辑：从 `GraphIndexer` 内部迁移到 `extractor.LLMExtractor`，作为 `extractor` 包默认提供的应用实现
- `result/compress.go`：保留，当前未用不代表后续不用，作为 result 包的预实现能力
- `query.Text2Cypher`：保留，作为 `query` 包的预实现类，承载自然语言→Cypher 的转换能力

---

## 3. 工作流程

### 3.1 索引流程

```
文件 → document.Open → RawDoc
                        ↓
                  structurizer.New
                        ↓
                  StructuredDoc
                  (标题/摘要/Chunks/Nodes/Edges 的空容器)
                        ↓
           ┌────────────┼─────────────┐
           ↓            ↓             ↓
       chunker        extractor     (可选)
       .Chunk()      .Extract()
       补 Chunks     补 Nodes/Edges
       (含 ParentID   (Nodes 通过
        建分块树)     SourceChunkIDs
           ↓            反向引用 Chunk)
           └────────────┴─────────────┐
                        ↓             │
                  StructuredDoc       │ (补全后)
                        ↓             │
                  embedder.Calc       │
                  (Title/Summary/     │
                   Content 三维度)    │
                        ↓             │
                  indexer.Index       │
                        ↓
              ┌─────────┴─────────┐
              ↓                   ↓
        VectorStore           GraphStore
        (每个 Chunk 3 个向量:  (实体 Node + Edge)
         chunkID         → Content 向量
         chunkID:title   → Title 向量
         chunkID:summary → Summary 向量)
              ↑                   ↑
              └─── 双向关联 ───────┘
              (通过 ChunkID 调 GetByChunkIDs 可找到
               引用该 Chunk 的实体 Node；
               Node.SourceChunkIDs 可反查 Chunk)
```

**双线结构（本项目核心）**：

- **语义线**：Chunk 通过 `ParentID` 自连结成分块树，支持任意分块向上追溯；Chunk 向量化后存入 `VectorStore`，支持语义检索。**Chunk 只存在于 VectorStore，不作为 Node 写入 GraphStore**
- **关系线**：实体 Node 存入 `GraphStore`，通过 Edge 形成关系网络；实体 Node 的 `SourceChunkIDs` 反向引用来源 Chunk
- **双向关联**：通过 `ChunkID` 调用 `GraphStore.GetByChunkIDs` 可找到引用该 Chunk 的实体 Node；通过 `Node.SourceChunkIDs` 可反查所有相关 Chunk（V1 已实现，V2 必须保持）
- **应用价值**：语义检索命中 Chunk 后，可通过 `GetByChunkIDs` 找到相关实体 Node，进而扩展到关系网络；反之亦然

**多维度向量索引（提高召回率的关键设计）**：

- **三大属性**：每个 Chunk 必须具备 `Title`（标题）/`Summary`（摘要）/`Content`（内容）三大属性，由 `Chunker`/`Extractor` 在分块/提取阶段填充
- **三维度向量化**：三大属性分别向量化，写入 VectorStore 形成 3 条向量记录：
  - `chunkID` → Content 向量（主向量）
  - `chunkID:title` → Title 向量（辅助向量）
  - `chunkID:summary` → Summary 向量（辅助向量）
- **维度澄清**：这里的「维度」是**数据维度**（data dimension），不是向量空间维度（vector space dimension）。3 条向量的 embedding 维度（向量空间维度）相同，但对应的数据维度不同
- **多维度检索**：语义搜索时同时对 3 个维度进行匹配，任一维度命中都能通过 vector id 定位到同一 Chunk
- **召回率提升**：覆盖用户不同的查询方式——短关键词（如函数名）更易命中 Title 维度，概述性查询更易命中 Summary 维度，具体内容查询更易命中 Content 维度；解决长短关键字不匹配、查询方式不可预判的问题

**关键约束**：

- `RawDoc` 是数据入口，承载文件元信息；不再支持内嵌附件（V1 `GetImages()` 删除）。
- `StructuredDoc` 是结构化中间产物，本身不调用 LLM；LLM 调用全部在 `Chunker` / `Extractor` 中。
- `Indexer` 只做向量化 + 存储路由，不再做分块、不再做实体提取、不再持有 LLM 客户端；但必须建立 Chunk↔Node 双向关联。
- `extractor.LLMExtractor` 作为默认应用实现直接可用，调用方无需自行实现 LLM 调用逻辑。
- `Region` 对应文件目录中的 `README.md`，是知识库分区的核心抽象；`GraphIndexer.Tree()` 基于 Region 输出知识树。
- 分块器必须基于 4 大结构化文档类型的结构特征（Markdown heading/数据树/图片整体/代码 AST），禁止分词式分块。

### 3.2 查询流程

```
Query → query.New → 预处理（tokenization/stopword）
                       ↓
                 indexer.Search
                       ↓
        ┌──────────────┴──────────────┐
        ↓                             ↓
   VectorStore.Search           GraphStore.Query
        ↓                             ↓
        └──────────────┬──────────────┘
                       ↓
                 result.Fusion
                       ↓
                 result.Rerank
                       ↓
                 result.Dedup
                       ↓
                   []Hit
```

---

## 4. V2 的核心变化

1. **删除 FullText 索引**：语义索引的 tag 字段天然支持 BM25 风格的过滤，无需 bleve 重复实现。
2. **删除 `store/doc/`**：bleve 依赖一并移除，降低二进制体积与维护成本。
3. **`RawDoc` 从结构变接口**：保证文档原子性，统一 `ID/Type/FileName/Content/Size/ModTime/Meta` 6 个方法。
4. **`StructuredDoc` 从结构变接口**：删除 `StructureNode` 树形抽象，改为 `Chunks()/Nodes()/Edges()` 扁平集合。
5. **LLM 逻辑从 `Indexer` 移出**：实体提取移到 `Extractor`，分块移到 `Chunker`；`Indexer` 不再依赖 `gochat`。但 LLM 作为常规操作，由 `extractor.LLMExtractor` 作为默认应用实现提供。
6. **`Indexer` 接口瘦身**：核心接口只保留 `Name/Index/Search/SearchGraph/Remove/Close` 5 个方法；`List/GetChunks/StoreChunk/Count/Clear` 剥离到 `IndexerAdmin` 扩展接口；`NewQuery` 剥离到 `query.New()`。
7. **`Region` 保留并明确语义**：Region 是知识库分区抽象，对应文件目录中的 `README.md`，作为 Graph 内的分区节点存在；`Indexer.Tree()` 保留，输出基于 Region 的知识树。
8. **`ChunkNode` 归一化为 `Node`**：不再单独定义类型，仅类型层面统一为 `Node`；**Chunk 不作为 Node 写入 Graph**，只在 VectorStore 中存在。归一化的目的是消除 V1 中 ChunkNode 与 Node 的类型重叠，而非让 Chunk 占据 Graph 节点位置。
9. **`Chunk.ParentID` 强化语义**：从 V1 的「父 Chunk/父文档 ID」明确为「分块树父节点 ID」，用于建立可追溯的分块树；与 `Node.SourceChunkIDs` 共同构成 Chunk↔Node 双向关联。
10. **chunker 包重写**：删除 V1 分词式分块器（fixed_size/sentence/paragraph/recursive/semantic/parent_doc/strategy），按 4 大结构化文档类型重新实现（MarkdownChunker/DatumChunker/ImageChunker/CodeChunker）。
11. **`GraphStore` 简化**：删除 `GetMultiHopPaths`/`GetAllEdgeTypes` 等使用率低的方法；保留 `GetNeighbors`/`GetByChunkIDs`/`Query`/`GetByLabels`。
12. **`Query` 保持接口形式并增强**：保留 `Raw/Keywords/Filters/AddFilter`；新增 `Type()` 返回查询类型（用于查询路由）；新增 `Embedding()`/`SetEmbedding()` 由 Query 自身承载查询向量（替代 V1 `Indexer.NewQuery`）；`Text2Cypher` 作为 `query` 包的预实现类保留。**Query 必须保持接口形式，不能改为结构体**——查询前优化、查询类型识别是行为而非数据。
13. **`result/compress.go` 保留**：当前未用不代表后续不用，作为 result 包的预实现能力保留。
14. **多维度向量索引**：每个 Chunk 必须具备 `Title`/`Summary`/`Content` 三大属性，三者分别向量化写入 VectorStore，主向量 ID 为 `chunkID`（Content），辅助向量 ID 为 `chunkID:title` 和 `chunkID:summary`。**「维度」指数据维度，非向量空间维度**——3 条向量的 embedding 维度相同，但对应的数据维度不同。语义搜索时同时匹配 3 个维度，任一命中即可定位同一 Chunk，覆盖用户不同的查询方式以提高召回率。

---

## 5. V1 过度设计裁剪清单

### 5.1 数据结构层

| V1 设计                          | 问题                                                                                                                    | V2 决策                                                                                                                  |
| -------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `core.StructureNode` 树形结构    | 字段过多（NodeType/Title/Level/StartPos/EndPos/Children/Comment/Relations），实际仅 heading/paragraph/code_block 被用到 | **删除**，由 `Chunk` 直接承载                                                                                            |
| `core.StructuredDocument` 结构体 | 结构体非接口，扩展性差；持有 `Root *StructureNode` 强依赖树形                                                           | **删除**，由 `StructuredDoc` 接口替代                                                                                    |
| `core.ChunkMeta`                 | `HeadingLevel/HeadingPath` 来自 StructureNode，删树后无意义                                                             | **简化**：仅保留 `Index/StartPos/EndPos`                                                                                 |
| `core.Chunk.MIMEType`            | 与 `Metadata["mime_type"]` 重复                                                                                         | **删除**字段，仅保留 metadata                                                                                            |
| `core.Chunk.ParentID`            | V1 语义模糊（既指父 Chunk 又指父文档）                                                                                  | **保留并强化语义**：明确为「分块树父节点 ID」，用于建立可追溯的分块树；与 `Node.SourceChunkIDs` 构成 Chunk↔Node 双向关联 |
| `core.Chunk.Title`               | 与 `Metadata["title"]` 重复                                                                                             | **删除**字段，仅保留 metadata                                                                                            |
| `core.Hit.Entities/Relations`    | 让 Hit 承担图查询职责，违反单一职责                                                                                     | **删除**字段；图查询结果走独立 `GraphResult` 类型                                                                        |
| `core.Document.GetImages()`      | V2 不再索引附件                                                                                                         | **删除**                                                                                                                 |
| `core.Document.GetExt()`         | 与 `GetMimeType()` 重复                                                                                                 | **删除**，统一为 `Type()`                                                                                                |
| `core.ChunkNode`                 | 与 `core.Node` 概念重叠，但 V1 同时存在两种类型导致 Graph 内节点类型不统一                                              | **归一化为 `Node`**：仅类型层面统一，消除类型重叠；**Chunk 不作为 Node 写入 Graph**，只在 VectorStore 中存在             |
| `core.Region` 结构体             | V1 中 Region 语义模糊（既像目录路径又像分区标识），且与 `RegionIndexer` 耦合                                            | **保留并明确语义**：Region 是知识库分区节点，对应目录中的 `README.md`，作为 Graph 内分区抽象                             |
| `core.Loader` 接口               | 与 `document.Open` 重复                                                                                                 | **删除**                                                                                                                 |

### 5.2 接口层

| V1 接口方法                                       | 问题                                | V2 决策                                                                                                                                                 |
| ------------------------------------------------- | ----------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Indexer.Add(ctx, content string)`                | 让 Indexer 同时承担「分块」职责     | **删除**，分块由 `Chunker` 完成；`Indexer` 只接受 `StructuredDoc`                                                                                       |
| `Indexer.AddFile(ctx, path)`                      | 同上，且与 `document.Open` 职责重叠 | **删除**                                                                                                                                                |
| `Indexer.NewQuery(terms)`                         | Query 应由 `query` 包创建           | **剥离**到 `query.New(text) core.Query`（返回接口类型）                                                                                                 |
| `Indexer.List(offset, limit)`                     | 分页浏览不属于核心索引职责          | **剥离**到 `IndexerAdmin.List` 扩展接口                                                                                                                 |
| `Indexer.GetChunks(docID)`                        | 同上                                | **剥离**到 `IndexerAdmin.GetChunks`                                                                                                                     |
| `Indexer.StoreChunk(chunk)`                       | 绕过分块直接写入是特殊场景          | **剥离**到 `IndexerAdmin.StoreChunk`                                                                                                                    |
| `Indexer.Count()` / `Clear()`                     | 管理操作非核心索引职责              | **剥离**到 `IndexerAdmin.Count` / `IndexerAdmin.Clear`                                                                                                  |
| `Indexer.Tree()`                                  | V1 中实现复杂且与 Region 耦合松散   | **保留**：作为 `GraphIndexer.Tree()` 方法，基于 Region 节点输出知识树                                                                                   |
| `GraphIndexer.CypherQuery` / `text2Cypher`        | V1 中作为 GraphIndexer 方法实现     | **保留并迁移**：`text2Cypher` 移到 `query.Text2Cypher` 作为预实现类；`GraphIndexer.CypherQuery` 删除（由调用方通过 `GraphStore.Query` 直接执行 Cypher） |
| `VectorStore.ListFiltered`                        | 过滤逻辑复杂且与 List 重复          | **简化**：合并为 `List(offset, limit, filters)`                                                                                                         |
| `GraphStore.GetMultiHopPaths` / `GetAllEdgeTypes` | 实际使用率低                        | **删除**；保留 `GetNeighbors(depth, limit)`                                                                                                             |
| `Chunker.Chunk(*StructuredDocument)`              | 依赖了不该存在的 StructureNode      | **改签**：`Chunk(RawDoc) ([]Chunk, error)`                                                                                                              |

### 5.3 实现层

| V1 实现                                                          | 问题                                     | V2 决策                                                                                                         |
| ---------------------------------------------------------------- | ---------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| `indexer/fulltext.go` + bleve 依赖                               | 与语义索引 tag 重复                      | **删除整个文件**                                                                                                |
| `store/doc/bleve/`                                               | 同上                                     | **删除整个目录**                                                                                                |
| `indexer.GraphIndexer` 持有 `chat.Client`                        | 违反 LLM 体外化原则                      | **迁移**：LLM 调用逻辑全部迁移到 `extractor.LLMExtractor`，作为 `extractor` 包默认应用实现                      |
| `indexer.EntityDef` + `WithSchemas` + `WithSchemasFromFS` + YAML | 三套实体配置机制                         | **迁移**：由 `extractor.LLMExtractor` 内部管理；V1 Prompt 模板（`prompts.go` 等）一并迁移                       |
| `indexer.IndexData` Ordinal ID 系统                              | 复杂的 ID 映射                           | **删除**，由 `Extractor` 直接返回 `[]Node` / `[]Edge`                                                           |
| `GraphIndexer.SetEntityDefsByRegion`                             | region 级隔离实体定义，过度灵活          | **简化**：Region 保留后，按 Region 选择 Schema 的逻辑由 `LLMExtractor` 内部实现，不再作为 GraphIndexer 公开方法 |
| `indexer.mergeIndexData` / `dedupRelations`                      | 内部数据合并逻辑                         | **删除**，由 `Extractor` 负责去重                                                                               |
| `core.Chunker` 接口（在 core 包）                                | 与 `chunker` 包接口重复                  | **删除** core 中的接口，仅保留 `chunker.Chunker`                                                                |
| `result/compress.go`                                             | 当前未用，但属于 result 包的预实现能力   | **保留**，不删除；后续可能用于结果压缩场景                                                                      |
| `query/fulltext.go` / `query/tree.go` / `query/stemming`         | 全文索引已删 / 未使用                    | **删除**                                                                                                        |
| `HybridIndexer`                                                  | fulltext 删除后，hybrid 失去意义         | **删除**，由调用方按需组合多个 `Indexer`                                                                        |
| `core.ChunkMeta.HeadingLevel/HeadingPath`                        | 依赖 StructureNode                       | **删除**                                                                                                        |
| `indexer.getChatClient` 懒加载 + TokenUsage 跟踪                 | LLM 状态泄漏到 indexer                   | **迁移**：Token 跟踪由 `extractor.LLMExtractor` 实现内部完成                                                    |
| `indexer/prompt.go` / `prompts.go` / `prompts_code.go`           | LLM Prompt 定义                          | **迁移**：移到 `extractor/llm.go` 内部，作为 `LLMExtractor` 的 Prompt 常量                                      |
| `indexer/text2Cypher`                                            | V1 在 indexer 包中                       | **迁移**：移到 `query.Text2Cypher` 作为预实现类保留                                                             |
| `chunker/strategy.go`                                            | 7 种分词式策略常量，与结构化分块理念冲突 | **删除**：分块策略由 `RawDocType` 自然决定，不再需要常量枚举                                                    |
| `chunker/fixed_size.go`                                          | 分词式分块，无结构语义                   | **删除**                                                                                                        |
| `chunker/sentence.go`                                            | 分词式分块，无结构语义                   | **删除**                                                                                                        |
| `chunker/paragraph.go`                                           | 分词式分块，无结构语义                   | **删除**                                                                                                        |
| `chunker/recursive.go`                                           | 分词式分块，无结构语义                   | **删除**                                                                                                        |
| `chunker/semantic.go`                                            | LLM 语义分块，与结构化分块理念冲突       | **删除**                                                                                                        |
| `chunker/parent_doc.go`                                          | 父子分块概念与 `Chunk.ParentID` 重复     | **删除**：父子关系由 `Chunk.ParentID` 直接承载                                                                  |
| `chunker/code.go`                                                | V1 基于 StructureNode，依赖被删类型      | **重写**：基于 AST 直接分块，不再依赖 StructureNode                                                             |
| `chunker/image_chunks.go`                                        | V1 实现保留                              | **保留**，适配新接口签名                                                                                        |

---

## 6. 核心数据模型

> V2 数据模型的核心是「双线结构」：Chunk 与 Node 必须双向关联。Chunk 通过 `ParentID` 自连结成分块树（语义线），只在 VectorStore 中存在；实体 Node 存入 GraphStore（关系线），其 `SourceChunkIDs` 反向引用 Chunk。通过 `ChunkID` 调用 `GraphStore.GetByChunkIDs` 可找到引用该 Chunk 的实体 Node；通过 `Node.SourceChunkIDs` 可反查所有相关 Chunk。

```go
// Chunk 分片：可索引的最小语义单元，承载语义线
// 通过 ParentID 自连结成可追溯的分块树；只在 VectorStore 中存在，不作为 Node 写入 GraphStore
// 与实体 Node 的关联通过 Node.SourceChunkIDs 反向引用实现
//
// 多维度向量索引：Title/Summary/Content 三大属性必须分别向量化
// - Content 向量 ID = chunkID（主向量）
// - Title 向量 ID = chunkID:title
// - Summary 向量 ID = chunkID:summary
// 任一维度命中都能定位同一 Chunk，覆盖用户不同查询方式以提高召回率
type Chunk struct {
    ID       string         `json:"id"`                  // Chunk 唯一标识（docID + 序号 + 内容哈希）
    ParentID string         `json:"parent_id,omitempty"` // 父 Chunk ID（空表示文档级 Chunk）；建立分块树，支持向上追溯
    DocID    string         `json:"doc_id"`              // 所属文档 ID（RawDoc.ID()）
    Title    string         `json:"title"`               // 分片标题（由 Chunker 从 Markdown heading/代码符号名等提取；向量化为 chunkID:title）
    Summary  string         `json:"summary"`             // 分片摘要（由 Chunker/Extractor 生成；向量化为 chunkID:summary）
    Content  string         `json:"content"`             // 分片内容（清洗后纯文本；向量化为主向量 chunkID）
    Index    int            `json:"index"`               // 分片在文档中的序号
    StartPos int            `json:"start_pos"`           // 在原文中的起始字节位置
    EndPos   int            `json:"end_pos"`             // 在原文中的结束字节位置
    Metadata map[string]any `json:"metadata,omitempty"`  // 扩展元数据（tags/source_file/region_id 等；不再含 title/summary，已提升为字段）
}

// Node 图节点：统一节点类型，通过 Labels 区分实体类型（Person/Organization 等）和 Region
// V1 中的 ChunkNode 归一化为此类型（仅类型层面统一，Chunk 不作为 Node 写入 Graph）
// V1 中的 Region 也合并为此类型，Labels=["Region"] 标识
// 双线结构关键：实体节点的 SourceChunkIDs 反向引用 Chunk，构成 Chunk↔Node 双向关联
type Node struct {
    ID             string         `json:"id"`                       // 节点唯一标识
    Labels         []string       `json:"labels"`                   // 节点类型标签，如 ["Person"] / ["Region"]（不再有 ["Chunk"]）
    Name           string         `json:"name"`                     // 节点名称（实体名/Region 目录名）
    Properties     map[string]any `json:"properties,omitempty"`     // 扩展属性（confidence/description/path/summary 等）
    SourceChunkIDs []string       `json:"source_chunk_ids,omitempty"` // 关联的 Chunk IDs（实体节点反向引用其来源 Chunk）
    SourceDocIDs   []string       `json:"source_doc_ids,omitempty"` // 关联的 Doc IDs
}

// 节点 Label 常量
const (
    LabelRegion = "Region" // 知识库分区节点，对应目录中的 README.md
    // 实体 Label（Person/Organization/Location 等）由 Extractor 实现内部定义
    // 注意：不再有 LabelChunk，Chunk 不作为 Node 写入 Graph
)

// 双线结构说明：
// 1. Chunk 只存在于 VectorStore（语义线），不作为 Node 写入 GraphStore
// 2. 实体 Node 存在于 GraphStore（关系线），其 SourceChunkIDs 反向引用来源 Chunk
// 3. 通过 ChunkID 调用 GraphStore.GetByChunkIDs 可找到引用该 Chunk 的实体 Node（V1 已实现，V2 必须保持）
// 4. 通过 Node.SourceChunkIDs 可反查所有相关 Chunk
// 5. 这种双向关联让语义检索与关系检索可以无缝衔接

// Region 知识库分区：对应文件目录中的 README.md
// 每个含 README.md 的目录构成一个 Region，README.md 内容作为该分区的摘要
// Region 同时作为 Graph 中的节点存在（Label=["Region"]），与其他实体节点形成 BELONGS_TO 关系
type Region struct {
    ID       string         `json:"id"`                       // Region 唯一标识（目录绝对路径的 SHA256）
    Path     string         `json:"path"`                     // 目录绝对路径（必须是绝对路径）
    Name     string         `json:"name"`                     // 目录名
    Summary  string         `json:"summary"`                  // README.md 的内容摘要
    ParentID string         `json:"parent_id,omitempty"`      // 父 Region ID（形成 Region 树）
    Meta     map[string]any `json:"meta,omitempty"`           // 扩展元数据（readme_path/doc_count 等）
}

// Edge 图边：实体间关系
type Edge struct {
    ID             string         `json:"id"`                       // 边唯一标识（source + type + target + docID 的哈希）
    Type           string         `json:"type"`                     // 关系类型（如 WORKS_FOR / BELONGS_TO / CONTAINS）
    Source         string         `json:"source"`                   // 起点节点 ID
    Target         string         `json:"target"`                   // 终点节点 ID
    Predicate      string         `json:"predicate,omitempty"`      // 关系描述（如 "就职于" / "属于"）
    Properties     map[string]any `json:"properties,omitempty"`     // 扩展属性
    SourceChunkIDs []string       `json:"source_chunk_ids,omitempty"`
    SourceDocIDs   []string       `json:"source_doc_ids,omitempty"`
}

// 边 Type 常量
const (
    EdgeBelongsTo = "BELONGS_TO" // 实体 → Region，表示实体所属的知识库分区
    // 其他关系 Type（WORKS_FOR/LOCATED_IN 等）由 Extractor 实现内部定义
    // 注意：不再有 EdgeContains（Region → Chunk）和 EdgeChildChunk（Chunk → Chunk），
    //       因为 Chunk 不作为 Node 写入 Graph，分块树只在 VectorStore.Metadata 中通过 parent_id 体现
)

// Vector 向量：Chunk 的向量化表示
//
// 多维度向量索引：每个 Chunk 对应 3 条向量记录，分别对应 3 个数据维度
// - ID = chunkID         → Content 维度向量（主向量）
// - ID = chunkID:title   → Title 维度向量
// - ID = chunkID:summary → Summary 维度向量
// 「维度」是数据维度（data dimension），不是向量空间维度（vector space dimension）
// 3 条向量的 embedding 维度（向量空间维度）相同，但对应的数据维度不同
// 语义搜索时同时匹配 3 个维度，任一命中即可通过 ChunkID 定位同一 Chunk
type Vector struct {
    ID       string         `json:"id"`                  // 向量唯一标识（chunkID / chunkID:title / chunkID:summary 三种形式）
    ChunkID  string         `json:"chunk_id"`            // 关联的 Chunk ID（3 条向量指向同一 ChunkID）
    Dim      string         `json:"dim"`                 // 数据维度标识："content" / "title" / "summary"
    Values   []float32      `json:"values"`              // 向量值（embedding model 输出，3 条向量的向量空间维度相同）
    Metadata map[string]any `json:"metadata,omitempty"`  // 持有 Chunk 的快照（doc_id/source_file/region_id/parent_id 等）
}

// Hit 查询结果
type Hit struct {
    ID       string         `json:"id"`                  // Chunk ID
    DocID    string         `json:"doc_id"`              // 文档 ID
    Score    float32        `json:"score"`               // 相关性分数（多维度检索时取最高分或加权融合）
    Dim      string         `json:"dim,omitempty"`       // 命中的数据维度："content" / "title" / "summary"（标识命中来源，便于调试与重排）
    Title    string         `json:"title,omitempty"`     // 命中 Chunk 的标题
    Summary  string         `json:"summary,omitempty"`   // 命中 Chunk 的摘要
    Content  string         `json:"content"`             // 命中 Chunk 的内容
    Metadata map[string]any `json:"metadata,omitempty"`  // 完整元数据（含 parent_id，支持向上追溯分块树）
}

// GraphResult 图查询结果（独立于 Hit，承载实体关系）
type GraphResult struct {
    Nodes []*Node `json:"nodes"`
    Edges []*Edge `json:"edges"`
}

// Query 查询接口：承载查询前优化、查询类型识别等重要功能
// 必须保持接口形式，不能改为结构体——查询优化（同义词扩展、关键词加权、查询分类等）
// 是行为而非数据，用结构体会丢失这些能力
type Query interface {
    // Raw 返回原始查询字符串
    Raw() string

    // Keywords 返回提取的关键词（用于过滤、BM25 等）
    Keywords() []string

    // Filters 返回元数据过滤条件
    Filters() map[string]any

    // AddFilter 添加过滤条件，返回 Query 自身支持链式调用
    AddFilter(key string, value any) Query

    // Type 返回查询类型（如 "semantic" / "keyword" / "hybrid" / "graph"）
    // 用于查询路由与优化，由查询前优化阶段识别
    Type() string

    // Embedding 返回查询向量（由 indexer 内部计算并缓存，避免重复计算）
    // V2 新增：删除 V1 Indexer.NewQuery 后，Embedding 由 Query 自身承载
    Embedding() []float32
    SetEmbedding(vec []float32) Query
}

// TreeNode 知识树节点：Indexer.Tree() 的返回类型
// 基于 Region 层级组装，叶子节点为 Chunk
type TreeNode struct {
    ID       string      `json:"id"`                 // Region ID 或 Chunk ID
    Type     string      `json:"type"`               // "region" 或 "chunk"
    Name     string      `json:"name"`               // Region 目录名或 Chunk 标题
    Summary  string      `json:"summary,omitempty"`  // Region 摘要（README.md 内容）
    Path     string      `json:"path,omitempty"`     // Region 目录绝对路径
    Children []*TreeNode `json:"children,omitempty"` // 子节点（仅 Region 有）
}
```

> 数据模型相比 V1 的关键变化：
> - **双线结构强化**：`Chunk.ParentID` 从 V1 的模糊语义明确为「分块树父节点 ID」，分块树只在 VectorStore 中通过 `parent_id` 体现；与 `Node.SourceChunkIDs` 构成 Chunk↔Node 双向关联
> - **多维度向量索引**：`Chunk` 新增 `Title`/`Summary` 字段（从 Metadata 提升为独立字段），与 `Content` 一起分别向量化；`Vector.ID` 有 3 种形式（`chunkID`/`chunkID:title`/`chunkID:summary`），`Vector.Dim` 标识数据维度；`Hit` 新增 `Dim`/`Title`/`Summary` 字段标识命中来源
> - `Chunk` 字段从 V1 的 8 个调整为 10 个（删除 `ChunkMeta/MIMEType/Title`，新增 `Title`/`Summary` 字段并保留 `ParentID`）
> - `Hit` 字段从 V1 的 8 个调整为 8 个（删除 `Entities/Relations`，新增 `Dim`/`Title`/`Summary`）
> - 新增 `GraphResult` 类型承载图查询结果
> - **`Query` 保持接口形式**：V1 是接口，V2 继续保持接口，**不能改为结构体**——Query 承载查询前优化（同义词扩展、关键词加权）、查询类型识别（semantic/keyword/hybrid/graph 路由）等行为，结构体无法承载。V2 新增 `Type()` 方法返回查询类型，`Embedding()`/`SetEmbedding()` 由 Query 自身承载（替代 V1 `Indexer.NewQuery`）
> - `ChunkNode` 归一化为 `Node`（仅类型层面统一，**Chunk 不作为 Node 写入 Graph**），消除 V1 类型重叠
> - `Region` 保留并明确语义为「对应目录 README.md 的知识库分区」，同时作为 Graph 内分区节点存在
> - 新增 `TreeNode` 类型作为 `Indexer.Tree()` 的返回，基于 Region 层级组装
> - **删除** `EdgeContains`（Region → Chunk）和 `EdgeChildChunk`（Chunk → Chunk）边类型：因 Chunk 不在 Graph 中，分块树只在 VectorStore.Metadata 中通过 `parent_id` 体现

---

## 7. 核心接口契约

### 7.1 `document.RawDoc`（接口）

```go
package document

// RawDocType 文档归一化后的 4 种类型
type RawDocType string

const (
    RawDocImage  RawDocType = "image"    // 图片（jpg/png/gif 等，缩限最小边长后转 Base64）
    RawDocDoc    RawDocType = "document" // 文档（epub/html/pdf/docx/md，统一为 Markdown）
    RawDocText   RawDocType = "text"     // 纯文本（txt/md/代码等，内容不变）
    RawDocData   RawDocType = "data"     // 数据（csv/json/xml/excel/eml/log/yml，统一为 JSON）
)

// RawDoc 归一化后的文档接口
type RawDoc interface {
    ID() string             // 文档唯一标识（FileName 的 SHA256）
    Type() RawDocType       // 归一化类型
    FileName() string       // 文件名（必须是绝对路径）
    Content() string        // 归一化后的内容
    Size() int64            // 原始文件大小（字节）
    ModTime() time.Time     // 原始文件修改时间
    Meta() map[string]any   // 元数据（width/height/title 等）
}

// Open 基于后缀名的工厂方法，返回对应 RawDoc 实现
// path 必须是绝对路径，否则返回 error
func Open(path string) (RawDoc, error)

// New 从文本内容和类型构造 RawDoc（无文件场景）
func New(content string, docType RawDocType) RawDoc
```

**与 V1 差异**：

- V1 `core.Document` 与 `document.RawDocument` 命名冲突 → V2 统一为 `document.RawDoc` 接口
- V1 `GetExt()/GetMimeType()/GetImages()` 全部删除
- V1 `GetSource()` 改名为 `FileName()`，明确语义且强制绝对路径

### 7.2 `structurizer.StructuredDoc`（接口）

```go
package structurizer

// StructuredDoc 结构化文档：RawDoc 的 Wrapper，承载结构化产物的容器
type StructuredDoc interface {
    Raw() RawDoc

    Title() string
    Summary() string
    Chunks() []core.Chunk
    Nodes() []core.Node
    Edges() []core.Edge

    SetTitle(string)
    SetSummary(string)
    SetChunks([]core.Chunk)
    SetNodes([]core.Node)
    SetEdges([]core.Edge)
}

// New 工厂方法：根据 RawDoc.Type 返回对应的 StructuredDoc 实现
// 实现分 3 类：Coder（代码）/ Datum（数据）/ Documented（文档）
// 纯文本归入 Documented 的最简单实现
func New(raw RawDoc) (StructuredDoc, error)
```

**与 V1 差异**：

- V1 `core.StructuredDocument` 是结构体 + `Root *StructureNode` 树形 → V2 改为扁平接口
- V1 `StructureNode` 树形抽象完全删除
- V1 4 种结构化器（Code/Config/Web/Markdown/Plain）→ V2 收敛为 3 种（Coder/Datum/Documented）

### 7.3 `chunker.Chunker`（接口）

```go
package chunker

// Chunker 分块器：从 RawDoc 提取 Chunks，按 4 大结构化文档类型实现
// 禁止使用 V1 的分词式分块（fixed_size/sentence/paragraph/recursive/semantic/parent_doc）
// 分块策略由 RawDoc.Type() 自然决定，不再需要 Strategy() 方法枚举
type Chunker interface {
    Chunk(doc document.RawDoc) ([]core.Chunk, error)
}

// 默认实现按 RawDoc.Type() 自动选择：
// - RawDocDoc（文档）   → MarkdownChunker：按 Markdown "#" 切分章节段落
// - RawDocData（数据）  → DatumChunker：按数据结构树状分块
// - RawDocImage（图片） → ImageChunker：整个图片作为一个 Chunk
// - RawDocText（代码）  → CodeChunker：通过语法解释器 AST 分块
//
// 所有分块器必须为返回的 Chunk 设置 ParentID（建立分块树）：
// - 文档级 Chunk：ParentID 为空
// - 子分块 Chunk：ParentID 指向其父 Chunk 的 ID

// MarkdownChunker Markdown 分块器
// 按 Markdown "#" 标题层级切分章节段落，每个标题下的内容构成一个 Chunk
// 父子关系按标题层级自然建立（H1 → H2 → H3）
type MarkdownChunker struct{}

// DatumChunker 数据分块器
// 按 JSON/YAML/XML 等数据结构树状分块，每个对象/数组元素构成一个 Chunk
// 父子关系按数据嵌套层级自然建立
type DatumChunker struct{}

// ImageChunker 图片分块器
// 整个图片作为一个 Chunk（图片基本不需要分块）
type ImageChunker struct{}

// CodeChunker 代码分块器
// 通过语法解释器 AST 进行分块，按函数/类/方法等语义单元切分
// 父子关系按代码嵌套层级建立（类 → 方法）
type CodeChunker struct{}

// New 默认工厂方法：根据 RawDoc.Type() 返回对应分块器实现
func New(doc document.RawDoc) (Chunker, error)
```

**与 V1 差异**：

- V1 `Chunk(doc *StructuredDocument)` 依赖了不该存在的 StructureNode → V2 改为接收 `RawDoc`
- V1 `core.Chunker` 与 `chunker.Chunker` 重复 → V2 仅保留 `chunker.Chunker`
- V1 `Strategy()` 方法返回 7 种策略常量 → V2 **删除**：分块策略由 `RawDoc.Type()` 自然决定，不再需要枚举
- V1 7 种分词式分块器（FixedSize/Sentence/Paragraph/Recursive/Semantic/ParentDoc/Code）→ V2 **重写**为 4 大结构化分块器（Markdown/Datum/Image/Code）
- V1 分块器不建立分块树 → V2 所有分块器必须为返回的 Chunk 设置 `ParentID` 建立分块树

### 7.4 `extractor.Extractor`（接口）

```go
package extractor

// Extractor 实体提取器：从 RawDoc 提取 Nodes 和 Edges
// 这是一个外部扩展接口，承载所有 LLM 调用逻辑
// extractor 包默认提供两种实现：RegexExtractor（规则）和 LLMExtractor（LLM 应用实现）
type Extractor interface {
    Extract(doc document.RawDoc) ([]core.Node, []core.Edge, error)
}
```

**默认应用实现**：

```go
// LLMExtractor 基于 LLM 的实体提取器
// 作为 extractor 包的默认应用实现，承载 V1 GraphIndexer 中的所有 LLM 调用逻辑
// LLM 是常规操作，本实现直接可用，无需外部重新实现
type LLMExtractor struct {
    chat      chat.Client           // LLM 客户端
    model     string                // 模型名称
    cache     extractor.CacheStore  // 实体提取缓存（避免重复调用 LLM）
    logger    logging.Logger
    schemas   map[string]EntitySchema // 按文件类型分组的实体 Schema
    prompts   map[string]string       // 按文件类型分组的 Prompt 模板
}

// NewLLMExtractor 构造函数（必传参数非空检查，返回 error）
// chat/model/logger 为必传，cache 可选（nil 表示不缓存）
func NewLLMExtractor(
    chat chat.Client,
    model string,
    logger logging.Logger,
    opts ...Option,
) (*LLMExtractor, error)

// RegexExtractor 基于正则的实体提取器
// 内置人名/电话/邮箱/URL/IP 等规则，无需 LLM 调用
type RegexExtractor struct {
    rules   []RegexRule
    logger  logging.Logger
}

func NewRegexExtractor(logger logging.Logger, opts ...Option) (*RegexExtractor, error)
```

**与 V1 差异**：

- V1 实体提取逻辑深埋在 `GraphIndexer.writeToStores` 中 → V2 独立为 `Extractor` 接口
- V1 `indexer.EntityDef` + `WithSchemas` + `WithSchemasFromFS` + YAML 三套配置 → V2 全部迁移到 `LLMExtractor` 内部管理
- V1 `indexer/prompt.go` / `prompts.go` / `prompts_code.go` → V2 全部迁移到 `extractor/llm.go` 内部作为 Prompt 常量
- V1 `indexer.getChatClient` 懒加载 + TokenUsage 跟踪 → V2 由 `LLMExtractor` 实现内部完成
- V1 `extractor.CacheStore` 缓存逻辑保留，作为 `LLMExtractor` 的内部细节
- **关键**：`LLMExtractor` 是 `extractor` 包默认提供的应用实现，调用方直接 `NewLLMExtractor` 即可用，不需要自行实现 LLM 调用逻辑

### 7.5 `indexer.Indexer`（接口，大幅瘦身）

```go
package indexer

// Indexer 索引器核心接口：接收已补全的 StructuredDoc，写入 VectorStore + GraphStore
// 不做分块、不做实体提取、不持有 LLM 客户端
// 仅包含索引和查询的核心方法，管理类方法见 IndexerAdmin
type Indexer interface {
    // Name 返回索引器名称（如 "semantic" / "graph"）
    Name() string

    // Index 将结构化文档写入索引
    // 调用前必须保证 StructuredDoc 的 Chunks/Nodes/Edges 已补全
    Index(ctx context.Context, doc structurizer.StructuredDoc) error

    // Search 执行查询，返回命中的 Chunks
    Search(ctx context.Context, q core.Query) ([]core.Hit, error)

    // SearchGraph 执行图查询，返回 Nodes 和 Edges
    // 仅 GraphIndexer 实现，SemanticIndexer 返回 nil
    SearchGraph(ctx context.Context, q core.Query) (*core.GraphResult, error)

    // Remove 按 ChunkID 移除索引项（连带删除关联的 Nodes/Edges）
    Remove(ctx context.Context, chunkID string) error

    // Close 释放底层资源
    Close(ctx context.Context) error
}

// IndexerAdmin 索引器管理接口：承载从 V1 Indexer 剥离的管理类方法
// 这些方法是从应用需求中收集到的必要功能，但又不属于核心索引职责
// 调用方按需 type-assert：if a, ok := idx.(IndexerAdmin); ok { ... }
type IndexerAdmin interface {
    // List 分页浏览已索引的 Chunk
    List(ctx context.Context, offset, limit int, filters []core.FilterCondition) ([]core.Chunk, int, error)

    // GetChunks 按 DocID 获取该文档的所有 Chunk
    GetChunks(ctx context.Context, docID string) ([]core.Chunk, error)

    // StoreChunk 绕过分块直接写入 Chunk（特殊场景，如手动补全）
    StoreChunk(ctx context.Context, chunk core.Chunk) error

    // Count 返回已索引的 Chunk 总数
    Count(ctx context.Context) (int, error)

    // Clear 清空索引
    Clear(ctx context.Context) error
}
```

**与 V1 差异**：

- V1 `Indexer` 14 个方法 → V2 核心 `Indexer` 5 个方法 + `IndexerAdmin` 5 个方法（按需实现）
- V1 `Add(ctx, content)` / `AddFile(ctx, path)` 删除 → 由调用方完成 RawDoc 创建
- V1 `NewQuery` 剥离到 `query.New(text) core.Query`（返回接口类型，Query 必须保持接口形式）
- V1 `List/GetChunks/StoreChunk/Count/Clear` 剥离到 `IndexerAdmin`，不再污染核心 `Indexer` 接口
- 新增 `SearchGraph`，让图查询走独立路径，不再污染 `Hit`

### 7.6 `embedder.Embedder`（接口，支持多维度向量化）

```go
package embedder

// Embedder 向量化器：将文本/图片转换为向量
// 核心方法 CalcChunk 为 Chunk 的 Title/Summary/Content 三个数据维度分别生成向量
type Embedder interface {
    // Calc 将单一文本转换为向量（用于 Region 摘要、查询向量化等场景）
    Calc(ctx context.Context, text string) (core.Vector, error)

    // CalcImage 将图片转换为向量
    CalcImage(ctx context.Context, imagePath string) (core.Vector, error)

    // CalcChunk 为 Chunk 的三个数据维度分别生成向量（多维度向量索引的核心方法）
    // 返回 3 条向量：
    //   - Vector{ID: chunkID,         Dim: "content",  Values: embedding(Content)}
    //   - Vector{ID: chunkID+":title",   Dim: "title",    Values: embedding(Title)}
    //   - Vector{ID: chunkID+":summary", Dim: "summary",  Values: embedding(Summary)}
    // 三条向量的向量空间维度（embedding model 输出维度）相同，但数据维度不同
    // 若 Title 或 Summary 为空，跳过对应维度（只生成非空维度的向量）
    CalcChunk(ctx context.Context, chunk core.Chunk) ([]core.Vector, error)

    // Dim 返回 embedding model 的向量空间维度（如 768/1024/1536 等）
    Dim() int
}
```

**多维度向量化关键设计**：

- `CalcChunk` 是 V2 新增方法，替代 V1 中只对 `Content` 向量化的做法
- 返回 `[]Vector`（1~3 条），每条向量对应一个数据维度
- 主向量 ID = `chunkID`（Content 维度），辅助向量 ID = `chunkID:title` 和 `chunkID:summary`
- 3 条向量的 `ChunkID` 字段都指向同一个 Chunk，便于反查
- 3 条向量的 `Dim` 字段标识数据维度（"content"/"title"/"summary"）
- **「维度」是数据维度（data dimension），不是向量空间维度（vector space dimension）**——3 条向量的 embedding 维度相同，但对应的数据维度不同
- 维度为空时跳过：若 `Chunk.Title` 为空则不生成 title 维度向量，`Summary` 同理；`Content` 必须非空

### 7.7 `indexer.SemanticIndexer` / `GraphIndexer`（实现）

```go
// SemanticIndexer 仅写入 VectorStore，不写 GraphStore
// SearchGraph 返回 nil
// 不实现 IndexerAdmin（如需管理能力，调用方直接使用 VectorStore）
type SemanticIndexer struct {
    db       core.VectorStore
    embedder core.Embedder
    logger   logging.Logger
}

func NewSemanticIndexer(db core.VectorStore, embedder core.Embedder, opts ...Option) (*SemanticIndexer, error)


// GraphIndexer 同时写入 VectorStore + GraphStore
// 构造函数不再接收 ModelConfig，LLM 调用由调用方通过 Extractor 完成
// 保留 Tree() 方法，基于 Region 节点输出知识树
// 实现 IndexerAdmin 接口
type GraphIndexer struct {
    vectorDB  core.VectorStore
    graphDB   core.GraphStore
    embedder  core.Embedder
    logger    logging.Logger
    // 注意：不再有 chatClient / model / entityDefs / regionEntityDefs
}

func NewGraphIndexer(
    vectorDB core.VectorStore,
    graphDB core.GraphStore,
    embedder core.Embedder,
    opts ...Option,
) (*GraphIndexer, error)

// Tree 输出基于 Region 层级的知识树
// 叶子节点为 Chunk，非叶子节点为 Region（对应目录中的 README.md）
// regionID 为空时返回整棵树；非空时返回该 Region 子树
func (g *GraphIndexer) Tree(ctx context.Context, regionID string) ([]*core.TreeNode, error)
```

**关键变化**：

- `GraphIndexer` 构造参数从 5 个（model/embedder/vectorDB/graphDB/opts）减到 4 个，移除 `ModelConfig`
- LLM 调用从 `GraphIndexer.Index` 内部移除，调用方需在调 `Index` 前用 `Extractor.Extract` 补全 `StructuredDoc`
- `Tree()` 方法保留，从 V1 的 `populateTree` 复杂实现简化为基于 Region 节点的层级组装
- `GraphIndexer` 实现 `IndexerAdmin` 接口，提供 `List/GetChunks/StoreChunk/Count/Clear`

### 7.8 `core.VectorStore` / `core.GraphStore`（接口，简化）

```go
// VectorStore 向量存储
type VectorStore interface {
    Upsert(ctx context.Context, vectors []*Vector) error
    Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*Vector, []float32, error)
    Delete(ctx context.Context, id string) error
    GetByDocID(ctx context.Context, docID string) ([]*Vector, error)
    List(ctx context.Context, offset, limit int, filters []FilterCondition) ([]*Vector, int, error) // 合并 V1 List + ListFiltered
    Count(ctx context.Context) (int, error)
    Clear(ctx context.Context) error
    Close(ctx context.Context) error
}

// GraphStore 图存储
type GraphStore interface {
    UpsertNodes(ctx context.Context, nodes []*Node) error
    UpsertEdges(ctx context.Context, edges []*Edge) error
    GetNode(ctx context.Context, id string) (*Node, error)
    GetNeighbors(ctx context.Context, nodeID string, depth, limit int) ([]*Node, []*Edge, error)
    GetByChunkIDs(ctx context.Context, chunkIDs []string) ([]*Node, []*Edge, error) // 合并 V1 GetNodesByChunkIDs + GetEdgesByChunkIDs；通过 ChunkID 反查引用该 Chunk 的实体 Node
    GetByLabels(ctx context.Context, labels []string, limit int) ([]*Node, error)   // 按 Label 查询节点（用于查询 Region 节点；不再有 Chunk 节点）
    DeleteNode(ctx context.Context, id string) error
    DeleteEdge(ctx context.Context, id string) error
    Query(ctx context.Context, query string, params map[string]any) ([]map[string]any, error)
    Clear(ctx context.Context) error
    Close(ctx context.Context) error
}
```

**与 V1 差异**：

- V1 `VectorStore` 9 方法 → V2 8 方法（合并 List/ListFiltered）
- V1 `GraphStore` 12 方法 → V2 10 方法（删除 `GetMultiHopPaths`/`GetAllEdgeTypes`/`GetNodesByChunkIDs`/`GetEdgesByChunkIDs`，用 `GetNeighbors` + `GetByChunkIDs` + `GetByLabels` 覆盖）
- 新增 `GetByLabels` 方法，用于按 Label 查询节点（如查询所有 Region 节点），支撑 `Tree()` 实现；**不再有 Chunk 节点**，因 Chunk 不作为 Node 写入 Graph

---

## 8. 模块详细设计

### 8.1 `document` - 文档归一化

**职责**：从文件读取内容，归一化为 4 种 `RawDoc` 类型之一。

**V1 → V2 改造点**：

1. 删除 `core.Document` 接口（V1 中与 `document.RawDocument` 重名）
2. 删除 `RawDocument.Images` 字段（V2 不索引附件）
3. 删除 `RawDocument.Meta` 上的 `images` 写入
4. `RawDocument` 结构体 → `RawDoc` 接口
5. `document.Open` 返回 `RawDoc` 而非 `core.Document`
6. 所有文件路径参数必须为绝对路径，否则返回 error

**4 类归一化策略**：

| 类型       | 扩展名                                        | 归一化方式                       |
| ---------- | --------------------------------------------- | -------------------------------- |
| `image`    | jpg/jpeg/png/gif/webp/bmp                     | 缩限最小边长 → Base64 编码字符串 |
| `document` | epub/html/pdf/docx/md/markdown                | 统一为 Markdown 结构             |
| `text`     | txt/md/所有代码文件（go/py/js/ts/rs/java 等） | 内容不变                         |
| `data`     | csv/json/xml/yml/yaml/excel/eml/log           | 统一为 JSON                      |

### 8.2 `structurizer` - 结构化器

**职责**：将 `RawDoc` 包装为 `StructuredDoc` 容器，**不做 LLM 调用**，只根据文件类型做基础结构化。

**3 类实现**：

| 实现         | 适用类型                        | 输出                                             |
| ------------ | ------------------------------- | ------------------------------------------------ |
| `Coder`      | 代码文件（.go/.py/.js 等）      | 基础 Chunks（按 AST 切分），Nodes/Edges 留空     |
| `Datum`      | 数据文件（.json/.yaml/.xml 等） | 基础 Chunks（按数据结构切分），Nodes/Edges 留空  |
| `Documented` | 文档文件（.md/.html/.pdf 等）   | 基础 Chunks（按 heading 切分），Nodes/Edges 留空 |

**关键决策**：

- V1 `MarkdownStructurizer` 用 tree-sitter 解析 → V2 保留，但删除 `StructureNode` 树，直接输出 `[]Chunk`
- V1 `PlainTextStructurizer` 启发式规则复杂 → V2 简化为「按空行分段，首行若像标题则为 title」
- V1 `ConfigStructurizer` 改名 `DatumStructurizer`
- V1 `WebStructurizer` 并入 `DocumentedStructurizer`
- V1 `CodeStructurizer` 改名 `CoderStructurizer`

### 8.3 `chunker` - 分块器

**职责**：从 `RawDoc` 重新提取 Chunks，覆盖 `StructuredDoc` 中的默认 Chunks。**必须为每个 Chunk 设置 `ParentID` 建立分块树，且必须填充 `Title`/`Summary`/`Content` 三大字段**（多维度向量索引的数据来源）。

**核心设计**：分块器针对 4 大结构化文档类型的天然结构特征进行分块，**禁止使用 V1 的分词式分块**（fixed_size/sentence/paragraph/recursive/semantic/parent_doc）。这些 V1 实现完全停留在理论层面、采用分词式，根本没有实用价值。

**4 大结构化分块器**：

| 分块器            | 适用类型                  | 分块策略                                                             | 分块树建立                                 | Title 来源               | Summary 来源                          |
| ----------------- | ------------------------- | -------------------------------------------------------------------- | ------------------------------------------ | ------------------------ | ------------------------------------- |
| `MarkdownChunker` | 文档（.md/.html/.pdf 等） | 按 Markdown "#" 标题层级切分章节段落，每个标题下的内容构成一个 Chunk | 父子关系按标题层级自然建立（H1 → H2 → H3） | Markdown heading 文本    | 段落首句或 LLM 生成（可选）           |
| `DatumChunker`    | 数据（.json/.yaml/.xml）  | 按数据结构树状分块，每个对象/数组元素构成一个 Chunk                  | 父子关系按数据嵌套层级自然建立             | 对象 key 或数组元素索引  | 对象/元素的简短描述（如有）           |
| `ImageChunker`    | 图片（.jpg/.png/.gif 等） | 整个图片作为一个 Chunk（图片基本不需要分块）                         | 单个 Chunk，无父子关系                     | 文件名或图片标题（如有） | 图片描述（如 EXIF 或 LLM 生成，可选） |
| `CodeChunker`     | 代码（.go/.py/.js 等）    | 通过语法解释器 AST 进行分块，按函数/类/方法等语义单元切分            | 父子关系按代码嵌套层级建立（类 → 方法）    | 函数/类/方法名           | 注释块首行或 LLM 生成（可选）         |

**关键决策**：

- V1 7 种分词式分块器全部废弃：fixed_size/sentence/paragraph/recursive/semantic/parent_doc/strategy
- V1 `CodeChunker` 基于 `StructureNode` → V2 重写为基于 AST 直接分块，不再依赖被删的 `StructureNode`
- V1 `image_chunks.go` 保留，适配新接口签名
- 分块策略由 `RawDoc.Type()` 自然决定，不再需要 `Strategy()` 方法和 `ChunkStrategy` 枚举常量
- 所有分块器返回的 Chunk 必须设置 `ParentID`（文档级 Chunk 为空，子分块指向父 Chunk ID），这是「双线结构」中语义线的根基
- 所有分块器返回的 Chunk 必须填充 `Title`/`Summary`/`Content` 三大字段，这是「多维度向量索引」的数据来源；`Content` 必须非空，`Title`/`Summary` 可空（空时跳过对应维度向量化）
- `chunker.New(doc)` 工厂方法按 `RawDoc.Type()` 自动选择对应分块器实现

### 8.4 `extractor` - 实体提取器

**职责**：从 `RawDoc` 提取 Nodes 和 Edges，承载所有 LLM 调用。

**默认应用实现**（extractor 包内提供，调用方开箱即用）：

- `RegexExtractor`：基于正则的简单实体提取（人名/电话/邮箱/URL/IP 等内置类型），无需 LLM
- `LLMExtractor`：基于 LLM 的实体提取器，**作为 extractor 包的默认应用实现**
  - 承载 V1 `GraphIndexer` 中的所有 LLM 调用逻辑（实体 Schema、Prompt、Token 跟踪）
  - 接收 `chat.Client` 和 `model` 作为必传参数
  - 内置按文件类型分组的实体 Schema（Person/Organization/Location 等）
  - 内置按文件类型分组的 Prompt 模板（迁移自 V1 `indexer/prompt*.go`）
  - 通过 `CacheStore` 提供实体提取缓存，避免重复调用 LLM
  - V1 `EntityDef` / `WithSchemas` / `WithSchemasFromFS` / YAML 配置全部内聚到本实现

**关键决策**：

- V1 实体提取逻辑深埋在 `GraphIndexer` 中 → V2 独立为 `Extractor` 接口，`LLMExtractor` 作为默认应用实现
- V1 `indexer.IndexData` Ordinal ID 系统 → V2 删除，`Extractor` 直接返回 `[]Node` / `[]Edge`
- V1 `indexer/prompt*.go` 全部迁移到 `extractor/llm.go` 内部作为 Prompt 常量
- V1 `indexer.EntityDef` + `WithSchemas` + YAML 三套配置 → V2 全部迁移到 `LLMExtractor` 内部
- V1 `extractor.ContentHash`（FNV-1a 16字符）→ V2 改用 `utils.GenerateID` 统一
- V1 `extractor.BuildFromExtraction` 保留，作为 `LLMExtractor` 的内部辅助
- **设计原则**：LLM 是常规操作，extractor 包必须提供可直接使用的 LLM 实现，不应只留接口给外部实现

### 8.5 `region` - 知识库分区（V2 保留并明确语义）

**职责**：将知识库按目录划分为多个 Region，每个含 `README.md` 的目录构成一个 Region；`README.md` 内容作为该分区的摘要。

**Region 的双重身份**：

1. **数据结构**：`core.Region` 结构体，描述分区元信息（ID/Path/Name/Summary/ParentID）
2. **Graph 节点**：以 `Node`（`Labels=["Region"]`）形态存入 Graph，与其他实体节点形成 `BELONGS_TO` 关系

**Region 树构造规则**：

- 扫描知识库根目录，每个含 `README.md` 的子目录构成一个 Region
- Region 的 `Path` 必须是绝对路径
- Region 的 `Summary` 取自 `README.md` 的内容（或前 N 字符）
- Region 的 `ParentID` 指向最近的祖先 Region（无则空）
- 不含 `README.md` 的目录不构成 Region，但其文件仍归属于最近的祖先 Region

**Chunk 与 Region 的关联**：

- 每个 Chunk 的 `Metadata["region_id"]` 指向所属 Region 的 ID
- `GraphIndexer.Tree()` 基于此关联组装知识树

**V1 → V2 改造点**：

1. V1 `core.Region` 语义模糊 → V2 明确为「对应目录 README.md 的知识库分区」
2. V1 `RegionIndexer` 单独存在 → V2 删除独立 RegionIndexer，Region 构造逻辑移到 `indexer` 包的辅助函数（如 `BuildRegions(rootPath) ([]core.Region, error)`）
3. V1 `Region` 字段含 `IsReadme` 等 UI 标识 → V2 删除，UI 层自行判断
4. V1 `SetEntityDefsByRegion` → V2 改为 `LLMExtractor` 内部根据 `RawDoc.FileName()` 推断所属 Region，按 Region 选择 Schema

### 8.6 `indexer` - 索引器

**职责**：接收已补全的 `StructuredDoc`，执行向量化 + 存储路由，**建立 Chunk↔Node 双向关联（双线结构的核心落地环节）**。

**`GraphIndexer.Index` 流程（V2 重构后）**：

```
1. 校验 doc.Chunks() / doc.Nodes() / doc.Edges() 已就绪
2. 对每个 Chunk：
   a. embedder.CalcChunk(chunk) → []Vector（1~3 条，对应 Content/Title/Summary 三维度）
      - Vector{ID: chunkID,         Dim: "content",  Values: embedding(Content)}  // 主向量
      - Vector{ID: chunkID+":title",   Dim: "title",    Values: embedding(Title)}    // 辅助向量（Title 非空时）
      - Vector{ID: chunkID+":summary", Dim: "summary",  Values: embedding(Summary)}  // 辅助向量（Summary 非空时）
   b. 每条 Vector.Metadata = chunk 快照（doc_id/source_file/region_id/parent_id 等）
   c. vectorDB.Upsert(vectors)  // 写入 1~3 条向量记录
   注意：Chunk 不作为 Node 写入 GraphStore，只在 VectorStore 中存在
3. graphDB.UpsertNodes(doc.Nodes())  // 仅写入实体 Node（由 Extractor 提取）
4. graphDB.UpsertEdges(doc.Edges())  // 仅写入实体 Edge（由 Extractor 提取）
5. 若 doc 关联 Region，构造 BELONGS_TO（实体 → Region）边一并写入
   注意：不再构造 CONTAINS（Region → Chunk）边，因 Chunk 不在 Graph 中
```

**双线结构在 Index 阶段的落地**：

- **语义线落地**：每个 Chunk 通过 `embedder.CalcChunk` 向量化为 1~3 条向量（多维度向量索引）写入 `VectorStore`，每条 Vector.Metadata 持有 `parent_id` 字段，分块树通过 `parent_id` 在 VectorStore 中向上追溯
- **关系线落地**：实体 Node（由 `Extractor` 提取）写入 `GraphStore`，通过 Edge 形成关系网络
- **双向关联落地**：实体 Node 的 `SourceChunkIDs` 反向引用来源 Chunk（由 `Extractor` 在提取阶段设置）；通过 `ChunkID` 调用 `graphDB.GetByChunkIDs(chunkIDs)` 可找到引用该 Chunk 的实体 Node（V1 已实现，V2 必须保持）
- **关键约束**：Chunk 不作为 Node 写入 GraphStore，分块树只在 VectorStore.Metadata 中通过 `parent_id` 体现；不再有 `EdgeChildChunk` 和 `EdgeContains` 边

**多维度向量索引在 Index 阶段的落地**：

- **三维度向量化**：每个 Chunk 通过 `embedder.CalcChunk` 生成 1~3 条向量（Content 必须非空；Title/Summary 为空时跳过对应维度）
- **向量 ID 规范**：主向量 ID = `chunkID`，辅助向量 ID = `chunkID:title` / `chunkID:summary`；3 条向量的 `ChunkID` 字段都指向同一 Chunk
- **向量 Dim 字段**：每条向量通过 `Vector.Dim` 标识数据维度（"content"/"title"/"summary"），便于查询时区分命中来源
- **查询阶段配合**：`Search` 时同时对 3 个维度进行匹配，任一命中都能通过 `Vector.ChunkID` 定位同一 Chunk，由 `result.Fusion` 融合多维度命中结果

**`GraphIndexer.Tree` 流程（V2 保留并简化）**：

```
1. graphDB.GetByLabels(["Region"], limit) → 获取所有 Region 节点
2. 按 ParentID 组装 Region 树
3. 对每个 Region 节点，通过 vectorDB.List 按 region_id 过滤查询其下属 Chunk
   （Chunk 不在 Graph 中，改用 VectorStore 查询）
4. 返回 []*TreeNode
```

**关键决策**：

- V1 `writeToStores` 内部的 ordinal→NodeID 映射、entity_ids 解析、parent_ordinal 推导 → V2 全部删除，由 `Chunker` 和 `Extractor` 在写入 `StructuredDoc` 前完成
- V1 `indexDimensionVectors`（title/summary 从属向量）→ V2 保留，作为 `SemanticIndexer` 的内部优化
- V1 `Refill` 方法 → V2 保留，用于存量数据迁移
- V1 `Tree()` / `populateTree()` 复杂实现 → V2 简化为基于 Region 节点 + VectorStore 查询的层级组装
- V1 `ChunkNode` 单独类型 → V2 归一化为 `Node`（仅类型层面统一，**Chunk 不作为 Node 写入 Graph**）
- V1 `GraphIndexer` 持有 LLM 客户端 → V2 移除，LLM 调用由 `extractor.LLMExtractor` 完成
- V1 通过 ChunkID 直接 `GetNode` → V2 改为通过 `GetByChunkIDs` 反查引用该 Chunk 的实体 Node（因 Chunk 不在 Graph 中）
- `GraphIndexer` 实现 `IndexerAdmin` 接口，提供 `List/GetChunks/StoreChunk/Count/Clear`
- `SemanticIndexer` 不实现 `IndexerAdmin`，调用方如需管理能力直接使用 `VectorStore`

### 8.7 `query` - 查询对象

**职责**：定义查询对象，提供简单的预处理（tokenization/stopword removal）；**保留 `Text2Cypher` 作为预实现类**，承载自然语言→Cypher 的转换能力。

**保留并迁移**：

- `query.Text2Cypher`：从 V1 `indexer.text2Cypher` 迁移到 `query` 包，作为预实现类保留
  - 承载自然语言→Cypher 的转换能力，是 query 包的重要预实现
  - 当前可能未在主流程中使用，但保留作为后续扩展的基础设施
  - 内部依赖 LLM 客户端完成转换，构造函数接收 `chat.Client` 和 `model` 作为必传参数
  - 调用方通过 `Text2Cypher.Translate(ctx, naturalLanguage) (cypher string, error)` 使用

```go
package query

// Text2Cypher 自然语言转 Cypher 预实现类
// 从 V1 indexer.text2Cypher 迁移，作为 query 包的预实现能力保留
// 当前可能未在主流程使用，但作为后续扩展的基础设施不删除
type Text2Cypher struct {
    chat    chat.Client
    model   string
    logger  logging.Logger
}

// NewText2Cypher 构造函数（必传参数非空检查，返回 error）
func NewText2Cypher(chat chat.Client, model string, logger logging.Logger) (*Text2Cypher, error)

// Translate 将自然语言转换为 Cypher 查询语句
func (t *Text2Cypher) Translate(ctx context.Context, naturalLanguage string) (string, error)
```

**V1 → V2 改造点**：

1. 删除 `query/fulltext.go`（全文索引已删）
2. 删除 `query/tree.go`（Tree 已由 `GraphIndexer.Tree()` 提供）
3. 删除 `query/graph.go`（V1 含 RawCypher/TextQuery 等复杂逻辑）→ 由 `core.Query` + `indexer.SearchGraph` 替代
4. 删除 `query/semantic.go` → 简化为 `query.New(text) core.Query`（返回接口类型）
5. 保留 `query/base.go`（tokenization/stopword）
6. **保留并迁移** `indexer.text2Cypher` → `query.Text2Cypher`（作为预实现类，承载自然语言→Cypher 转换能力）

### 8.8 `result` - 结果后处理

**职责**：对 `[]Hit` 进行融合、重排、去重；保留全部实现，包括 `compress.go`。

**保留全部**：

- `result/reranker.go`：结果重排
- `result/fusion.go`：结果融合
- `result/dedup.go`：结果去重
- `result/compress.go`：结果压缩（**保留**，当前未用不代表后续不用，作为 result 包的预实现能力）

**关键决策**：

- V1 `compress.go` 在 README 中标注"基本没用到" → V2 **保留不删除**
- 设计原则：现在不用不代表后面不用，预实现能力应保留以备后续扩展
- `compress.go` 的接口签名适配新 `Hit` 字段（删除 `Entities/Relations`）

### 8.9 `store/*` - 存储实现

**保留**：`store/vector/govector` / `store/graph/gograph` / `store/cache/bbolt`

**删除**：`store/doc/bleve`（整个目录）

---

## 9. 分阶段实施计划

### 里程碑 M1：核心接口定义（基础）

**目标**：在 `core` 和 `document` 包中完成 V2 接口定义，不破坏 V1 编译。

**任务**：

1. `core/chunk.go`：定义新 `Chunk`（删除 `ChunkMeta/MIMEType/Title`，**保留 `ParentID`** 并强化语义为「分块树父节点 ID」）
2. `core/entity.go`：`Node`/`Edge` 不变；新增 `LabelRegion` 常量；新增 `EdgeBelongsTo` 常量（**不再有 `LabelChunk`/`EdgeContains`/`EdgeChildChunk`**，因 Chunk 不作为 Node 写入 Graph）
3. `core/region.go`：定义新 `Region` 结构体（按 §6 定义）+ `TreeNode` 类型
4. `core/vector.go`：`Vector` 不变
5. `core/hit.go`：删除 `Entities/Relations` 字段，新增 `GraphResult` 类型
6. `core/query.go`：**保留 `Query` 接口形式**（不改为结构体）；在 V1 基础上新增 `Type()` 返回查询类型、`Embedding()`/`SetEmbedding()` 承载查询向量（替代 V1 `Indexer.NewQuery`）；保留 `Raw/Keywords/Filters/AddFilter`
7. `core/indexer.go`：定义核心 `Indexer` 接口 + `IndexerAdmin` 扩展接口（按 §7.5）
8. `core/vectorstore.go` / `core/graphstore.go`：按 §7.7 简化（含新增 `GetByLabels`）
9. `document/raw.go`：定义 `RawDoc` 接口 + `RawDocType` 枚举

**验收**：

- `go build ./core/... ./document/...` 通过
- V1 接口暂保留为 `Deprecated` 别名，保证后续里程碑编译

### 里程碑 M2：删除清单执行（清理）

**目标**：删除 V2 不再需要的代码，移除 bleve 依赖。

**任务**：

1. 删除 `indexer/fulltext.go` / `indexer/fulltext_test.go`
2. 删除 `store/doc/`（整个目录）
3. 删除 `core/fulltext.go`
4. 删除 `core/structurizer.go`（V1 接口）
5. 删除 `core/chunker.go`（V1 接口）
6. 删除 `core/loader.go`
7. 删除 `core/chunknode.go`（V1 类型，已归一化为 `Node`）；**保留** `core/region.go`（M1 已重新定义）
8. 删除 `core/reconstruct.go`（依赖 StructureNode）
9. 删除 `query/fulltext.go` / `query/tree.go` / `query/graph.go` / `query/semantic.go`
10. **保留** `result/compress.go`（当前未用不代表后续不用，作为 result 包预实现能力保留；M7 适配新 `Hit` 字段）
11. 删除 `hybrid.go` / `hybrid_test.go` / `hybrid_bench_test.go`
12. 重构 `indexer/region.go`（删除 V1 `RegionIndexer`，保留 `BuildRegions` 辅助函数）
13. 删除 `indexer/ontologies.go` / `indexer/ontologies_code.go`（迁移到 `extractor`）
14. 删除 `indexer/prompt.go` / `prompts.go` / `prompts_code.go`（迁移到 `extractor/llm.go`，由 M5 完成）
15. 删除 `structurizer/web.go` / `structurizer/config.go`（并入新实现）
16. 重构 `extractor/cache.go`：`ContentHash` 改用 `utils.GenerateID`
17. `go.mod` 移除 bleve 依赖

**验收**：

- `go build ./...` 通过
- `go vet ./...` 通过
- 二进制体积显著缩小

### 里程碑 M3：`document` 包改造

**目标**：`document` 包全面实现 `RawDoc` 接口。

**任务**：

1. `document/raw.go`：定义 `RawDoc` 接口，提供 `imageDoc/docDoc/textDoc/dataDoc` 4 个实现
2. `document.Open`：改为工厂方法，根据后缀名返回对应实现
3. `document.New`：从文本内容构造 `textDoc`
4. 各文件类型（csv/docx/eml/epub/html/image/msg/pdf/pptx/txt/xlsx）改造为返回 `RawDoc`
5. 删除所有 `GetImages()` 调用，附件不再索引
6. 所有 `Open(path)` 必须校验绝对路径

**验收**：

- `go test ./document/...` 通过
- 4 种类型的文件都能正确归一化

### 里程碑 M4：`structurizer` 包改造

**目标**：实现 3 类 `StructuredDoc` 容器。

**任务**：

1. `structurizer/entry.go`：定义 `StructuredDoc` 接口 + `New` 工厂方法
2. `structurizer/coder.go`：`CoderStructurizer`（基于 AST，输出 Chunks）
3. `structurizer/datum.go`：`DatumStructurizer`（基于数据结构，输出 Chunks）
4. `structurizer/documented.go`：`DocumentedStructurizer`（基于 heading，输出 Chunks）
5. 删除 `structurizer/markdown.go` 中的 `StructureNode` 树构建逻辑，直接输出 `[]Chunk`
6. 删除 `structurizer/plain.go` 中的复杂启发式规则
7. 删除 `structurizer/web.go` / `structurizer/config.go` / `structurizer/node_helpers.go`

**验收**：

- `go test ./structurizer/...` 通过
- 3 类文件都能正确产出 `StructuredDoc`

### 里程碑 M5：`chunker` + `extractor` 改造

**目标**：`Chunker` 接口落地为 4 大结构化分块器；`Extractor` 接口落地，`LLMExtractor` 作为默认应用实现。

**任务**：

1. **删除 V1 分词式分块器**（共 7 个文件）：
   - `chunker/strategy.go`（7 种策略常量，与结构化分块理念冲突）
   - `chunker/fixed_size.go`（分词式分块，无结构语义）
   - `chunker/sentence.go`（分词式分块，无结构语义）
   - `chunker/paragraph.go`（分词式分块，无结构语义）
   - `chunker/recursive.go`（分词式分块，无结构语义）
   - `chunker/semantic.go`（LLM 语义分块，与结构化分块理念冲突）
   - `chunker/parent_doc.go`（父子分块概念与 `Chunk.ParentID` 重复）
2. **重写 4 大结构化分块器**：
   - `chunker/markdown.go`：`MarkdownChunker`，按 Markdown "#" 标题层级切分章节段落
   - `chunker/datum.go`：`DatumChunker`，按数据结构树状分块
   - `chunker/image.go`：`ImageChunker`（基于 V1 `image_chunks.go` 重构），整个图片作为一个 Chunk
   - `chunker/code.go`：`CodeChunker` 重写，基于 AST 直接分块，不再依赖 `StructureNode`
3. `chunker/chunker.go`：定义 `Chunker` 接口（仅 `Chunk(doc RawDoc) ([]Chunk, error)` 方法，**删除 `Strategy()`**）
4. `chunker/factory.go`：`New(doc RawDoc) (Chunker, error)` 工厂方法，按 `RawDoc.Type()` 自动选择
5. 所有分块器签名统一为 `Chunk(doc document.RawDoc) ([]core.Chunk, error)`
6. **所有分块器必须为返回的 Chunk 设置 `ParentID` 建立分块树**（文档级 Chunk 为空，子分块指向父 Chunk ID）
7. `extractor/extractor.go`：定义 `Extractor` 接口
8. `extractor/regex.go`：`RegexExtractor` 实现（合并 V1 内置规则）
9. `extractor/llm.go`：`LLMExtractor` 实现（**默认应用实现**）
   - 接收 `chat.Client` 和 `model` 作为必传参数
   - 内置按文件类型分组的实体 Schema（迁移自 V1 `indexer/ontologies*.go`）
   - 内置按文件类型分组的 Prompt 模板（迁移自 V1 `indexer/prompt*.go`）
   - 通过 `CacheStore` 提供缓存
   - 按 `RawDoc.FileName()` 推断所属 Region，选择对应 Schema（替代 V1 `SetEntityDefsByRegion`）
10. `extractor/cache.go`：保留缓存逻辑，`ContentHash` 改用 `utils.GenerateID`
11. 删除 `indexer/prompt*.go` 和 `indexer/ontologies*.go`（已在 M2 标记，M5 完成迁移后删除）

**验收**：

- `go test ./chunker/... ./extractor/...` 通过
- `chunker/` 目录下不再有 `strategy.go` / `fixed_size.go` / `sentence.go` / `paragraph.go` / `recursive.go` / `semantic.go` / `parent_doc.go` 文件
- 4 大结构化分块器能正确按文档类型分块，返回的 Chunk 含 `ParentID` 建立分块树
- 4 大结构化分块器返回的 Chunk 含 `Title`/`Summary`/`Content` 三大字段（`Content` 必须非空，`Title`/`Summary` 可空），为多维度向量索引提供数据来源
- `LLMExtractor` 能独立完成实体提取并返回 `[]Node` / `[]Edge`
- 调用方仅需 `NewLLMExtractor(chat, model, logger)` 即可使用，无需自行实现 LLM 调用逻辑

### 里程碑 M6：`indexer` 包改造

**目标**：`SemanticIndexer` 和 `GraphIndexer` 按 V2 接口落地，保留 `Tree()` 方法，**落地双线结构（Chunk↔Node 双向关联）**。

**任务**：

1. `indexer/semantic.go`：实现核心 `Indexer` 接口，`SearchGraph` 返回 nil
2. `indexer/graph.go`：实现核心 `Indexer` 接口，删除 LLM/ModelConfig/chatClient/EntityDef 等
3. `indexer/graph.go` 的 `Index` 方法：仅做向量化 + 存储路由，调用前要求 `StructuredDoc` 已补全
4. **双线结构落地**：`Index` 方法必须执行以下步骤建立 Chunk↔Node 双向关联：
   - Chunk 只写入 `VectorStore`（向量化 + `Metadata` 含 `parent_id`/`region_id` 等），**不作为 Node 写入 GraphStore**
   - 实体 Node（由 `Extractor` 提取）写入 `GraphStore`，其 `SourceChunkIDs` 反向引用来源 Chunk
   - 通过 `ChunkID` 调用 `graphDB.GetByChunkIDs(chunkIDs)` 可找到引用该 Chunk 的实体 Node（V1 已实现，V2 必须保持）
   - 不再构造 `EdgeChildChunk` 和 `EdgeContains` 边（因 Chunk 不在 Graph 中）
5. **多维度向量索引落地**：`Index` 方法必须为每个 Chunk 调用 `embedder.CalcChunk` 生成 1~3 条向量：
   - 主向量 ID = `chunkID`（Content 维度，必须生成）
   - 辅助向量 ID = `chunkID:title` / `chunkID:summary`（Title/Summary 维度，非空时生成）
   - 每条向量 `Dim` 字段标识数据维度，`ChunkID` 字段指向同一 Chunk
   - `Search` 时同时匹配 3 个维度，由 `result.Fusion` 融合多维度命中结果
6. `indexer/graph.go` 的 `Tree` 方法：基于 Region 节点 + `VectorStore.List(region_id)` 组装知识树（Chunk 不在 Graph 中，改用 VectorStore 查询）
7. `indexer/graph.go` 实现 `IndexerAdmin` 接口（`List/GetChunks/StoreChunk/Count/Clear`）
8. `indexer/region.go`：保留 `BuildRegions(rootPath)` 辅助函数，构造 `[]core.Region`
9. `indexer/utils.go`：保留 `indexDimensionVectors` 等辅助函数（V1 已有 title/summary 从属向量优化，V2 与多维度向量索引合并）
10. `indexer/types.go`：删除 `IndexData` / `mergeIndexData` / `dedupRelations`
11. `indexer/chunk_bench_test.go`：适配新接口
12. `indexer/tree_test.go`：新增 `Tree()` 测试，覆盖 Region 层级和 Chunk 叶子
13. `indexer/dual_line_test.go`：新增双线结构测试，覆盖：
    - 通过 `ChunkID` 调用 `GetByChunkIDs` 能找到引用该 Chunk 的实体 Node
    - 通过 `Node.SourceChunkIDs` 能反查所有相关 Chunk
    - 通过 `ParentID` 能在 `VectorStore.Metadata` 中追溯父 Chunk（分块树在 VectorStore 中可查询）
    - 验证 Chunk 不作为 Node 写入 GraphStore（`GetNode(chunkID)` 返回 not found）
14. `indexer/multidim_test.go`：新增多维度向量索引测试，覆盖：
    - 每个 Chunk 在 VectorStore 中有 1~3 条向量记录（Content 必须有；Title/Summary 非空时有）
    - 主向量 ID = `chunkID`，辅助向量 ID = `chunkID:title` / `chunkID:summary`
    - 每条向量 `Dim` 字段标识数据维度（"content"/"title"/"summary"）
    - 3 条向量的 `ChunkID` 字段都指向同一 Chunk
    - `Search` 时同时匹配 3 个维度，任一命中都能通过 `ChunkID` 定位同一 Chunk

**验收**：

- `go build ./indexer/...` 通过
- `go test ./indexer/...` 通过
- 端到端测试：`document.Open → structurizer.New → extractor.LLMExtractor.Extract → indexer.Index → indexer.Search → indexer.Tree` 全流程跑通
- `Tree()` 能正确返回基于 Region 的知识树
- **双线结构验收**：通过 `ChunkID` 调用 `GetByChunkIDs` 能找到引用该 Chunk 的实体 Node；通过 `Node.SourceChunkIDs` 能反查 Chunk；通过 `ParentID` 能在 VectorStore 中追溯分块树；Chunk 不作为 Node 写入 GraphStore
- **多维度向量索引验收**：每个 Chunk 在 VectorStore 中有 1~3 条向量；向量 ID 符合规范（`chunkID`/`chunkID:title`/`chunkID:summary`）；3 个维度同时匹配可提高召回率

### 里程碑 M7：`query` + `result` + `cmd` 收尾

**目标**：完成剩余模块改造，全项目编译测试通过；保留 `Text2Cypher` 和 `compress.go` 作为预实现能力。

**任务**：

1. `query/base.go`：保留 tokenization/stopword
2. `query/query.go`：提供 `New(text) core.Query` 构造函数（返回接口类型，Query 必须保持接口形式）
3. `query/text2cypher.go`：**保留并迁移** V1 `indexer.text2Cypher`，作为 `query.Text2Cypher` 预实现类
   - 构造函数 `NewText2Cypher(chat, model, logger) (*Text2Cypher, error)` 返回 error
   - 提供 `Translate(ctx, naturalLanguage) (string, error)` 方法
   - 当前未在主流程使用，但作为后续扩展的基础设施保留
4. `result/reranker.go` / `fusion.go` / `dedup.go`：接口适配（`Hit` 字段变化）
5. `result/compress.go`：**保留**，接口签名适配新 `Hit` 字段（删除 `Entities/Relations`）
6. `cmd/*.go`：适配新接口
7. `svc.go` / `main.go` / `hybrid.go`（已删除）：清理引用
8. 更新 `go.mod` 移除 bleve 等无用依赖
9. 更新 `Makefile` 测试目标

**验收**：

- `go build ./...` 通过
- `go test ./...` 通过（10 秒超时）
- `go vet ./...` 通过
- 命令行 `midx` 工具能完成索引和查询
- `query.Text2Cypher` 类存在且可构造
- `result/compress.go` 文件存在且编译通过

---

## 10. 测试与验收标准

### 10.1 单元测试

- 每个新接口必须有「契约测试」：验证实现满足接口约定
- 所有测试必须包含 10 秒超时：`t.Parallel()` + `context.WithTimeout(ctx, 10*time.Second)`
- 测试数据使用 `testdata/` 目录，**绝对路径**通过 `filepath.Abs` 在测试入口统一计算

### 10.2 集成测试

- 端到端流程：`Open → Structurize → Chunk → Extract → Index → Search → Tree`
- 测试覆盖 4 种文档类型（image/document/text/data）
- 测试覆盖 3 种 Structurizer（Coder/Datum/Documented）
- 测试覆盖 Region 构造与 `Tree()` 输出（含多层级 Region 嵌套）

### 10.3 验收清单

- [ ] `go build ./...` 通过
- [ ] `go vet ./...` 通过
- [ ] `go test ./...` 通过（10 秒超时）
- [ ] 二进制体积较 V1 缩小 ≥ 30%（bleve 移除）
- [ ] `indexer` 包不再 import `gochat`（LLM 已迁移到 `extractor.LLMExtractor`）
- [ ] `extractor.LLMExtractor` 作为默认应用实现可直接使用
- [ ] `core` 包不再有 `StructureNode` 引用
- [ ] `core.ChunkNode` 已归一化为 `Node`（仅类型层面统一，Chunk 不作为 Node 写入 Graph）
- [ ] `core.Region` 保留并对应目录中的 `README.md`
- [ ] `GraphIndexer.Tree()` 正确返回基于 Region 的知识树
- [ ] `IndexerAdmin` 接口提供 `List/GetChunks/StoreChunk/Count/Clear`
- [ ] 所有文件路径参数为绝对路径
- [ ] 所有构造函数返回 error
- [ ] 所有代码注释为中文
- [ ] **双线结构**：`Chunk.ParentID` 字段存在且语义为「分块树父节点 ID」
- [ ] **双线结构**：通过 `ChunkID` 调用 `graphDB.GetByChunkIDs` 能找到引用该 Chunk 的实体 Node
- [ ] **双线结构**：通过 `Node.SourceChunkIDs` 能反查所有相关 Chunk
- [ ] **双线结构**：`Vector.Metadata` 持有 `parent_id` 字段，分块树在 VectorStore 中可追溯
- [ ] **双线结构**：Chunk 不作为 Node 写入 GraphStore（`GetNode(chunkID)` 返回 not found）
- [ ] **双线结构**：不再有 `LabelChunk` / `EdgeChildChunk` / `EdgeContains` 常量
- [ ] **多维度向量索引**：`Chunk` 含 `Title`/`Summary`/`Content` 三大字段（独立字段，非 Metadata）
- [ ] **多维度向量索引**：`embedder.CalcChunk` 能为 Chunk 生成 1~3 条向量（Content 必须非空；Title/Summary 为空时跳过）
- [ ] **多维度向量索引**：向量 ID 符合规范——主向量 `chunkID`，辅助向量 `chunkID:title` / `chunkID:summary`
- [ ] **多维度向量索引**：每条向量 `Dim` 字段标识数据维度（"content"/"title"/"summary"）
- [ ] **多维度向量索引**：3 条向量的 `ChunkID` 字段都指向同一 Chunk
- [ ] **多维度向量索引**：`Search` 时同时匹配 3 个维度，任一命中都能通过 `ChunkID` 定位同一 Chunk
- [ ] **多维度向量索引**：「维度」是数据维度，不是向量空间维度（3 条向量 embedding 维度相同）
- [ ] **chunker 重写**：`chunker/` 目录下不再有 `strategy.go` / `fixed_size.go` / `sentence.go` / `paragraph.go` / `recursive.go` / `semantic.go` / `parent_doc.go`
- [ ] **chunker 重写**：4 大结构化分块器存在（`MarkdownChunker` / `DatumChunker` / `ImageChunker` / `CodeChunker`）
- [ ] **chunker 重写**：`Chunker` 接口不再有 `Strategy()` 方法
- [ ] **chunker 重写**：所有分块器返回的 Chunk 含 `ParentID` 建立分块树
- [ ] **Text2Cypher 保留**：`query.Text2Cypher` 类存在且可构造
- [ ] **compress.go 保留**：`result/compress.go` 文件存在且编译通过

---

## 11. 风险与边界

### 11.1 不在 V2 范围内

- 多模态嵌入（V1 chinese-clip）保留但不在 V2 重构范围内
- `minirag` 子项目保留但不在 V2 重构范围内
- 外部 VectorDB/GraphDB 驱动（Milvus/Neo4j 等）不在 V2 范围内，仅保持接口兼容

### 11.2 已知风险

| 风险                                                               | 影响 | 缓解措施                                                                     |
| ------------------------------------------------------------------ | ---- | ---------------------------------------------------------------------------- |
| `StructuredDoc` 接口化导致 V1 `*StructuredDocument` 调用方全部破坏 | 高   | M1 保留 Deprecated 别名，M4 完成后删除                                       |
| `Chunker` 接口签名变更（`*StructuredDocument` → `RawDoc`）         | 中   | M5 集中改造，所有分块器同步迁移                                              |
| `GraphIndexer` 不再持有 LLM，调用方需自行调 `Extractor`            | 中   | `extractor.LLMExtractor` 作为默认应用实现，调用方直接 `NewLLMExtractor` 即可 |
| 删除 `HybridIndexer` 影响外部调用方                                | 中   | M7 提供 `MultiIndexer` helper 替代，按权重组合多个 Indexer                   |
| `ChunkNode` 归一化为 `Node` 影响 Graph 内节点查询逻辑              | 中   | M6 同步改造查询逻辑：Chunk 不写入 Graph，通过 `GetByChunkIDs` 反查实体 Node  |
| `IndexerAdmin` 剥离影响 V1 调用方                                  | 低   | 通过 type-assert 兼容：`if a, ok := idx.(IndexerAdmin); ok { ... }`          |
| Region 构造逻辑变更（V1 独立 Indexer → V2 辅助函数）               | 中   | M6 在 `indexer/region.go` 提供 `BuildRegions(rootPath)` 兼容入口             |

### 11.3 边界约束

- V2 不再支持文件内嵌附件索引（V1 `GetImages()` 删除）
- V2 不再支持 `Add(content string)` 直接索引文本（必须走 `RawDoc` 流程）
- V2 不再支持 `Indexer.NewQuery()`（Query 由 `query.New` 创建）
- V2 不再支持全文索引（BM25 风格检索通过 `VectorStore.List(filters)` 实现）
- V2 保留 `Region` 与 `Tree()`，但语义明确为「对应目录 README.md 的知识库分区」

---

## 附录 A：V1 → V2 接口对照速查

| V1                                         | V2                                                            | 备注                                                         |
| ------------------------------------------ | ------------------------------------------------------------- | ------------------------------------------------------------ |
| `core.Document`                            | `document.RawDoc`                                             | 接口化，删除 `GetImages/GetExt`                              |
| `document.RawDocument`                     | `document.RawDoc` 实现之一                                    | 命名统一                                                     |
| `core.StructuredDocument`                  | `structurizer.StructuredDoc`                                  | 接口化，删除树形                                             |
| `core.StructureNode`                       | （删除）                                                      | 由 `Chunk` 承载                                              |
| `core.ChunkMeta`                           | `Chunk.Index/StartPos/EndPos`                                 | 字段扁平化                                                   |
| `core.Chunk.ParentID`                      | `core.Chunk.ParentID`（保留并强化语义）                       | 语义从「父 Chunk/父文档 ID」明确为「分块树父节点 ID」        |
| `core.Chunker`                             | `chunker.Chunker`                                             | 接收 `RawDoc` 而非 `*StructuredDocument`；删除 `Strategy()`  |
| `core.Structurizer`                        | `structurizer.Structurizer`                                   | 内部接口                                                     |
| `core.Indexer`（14 方法）                  | `indexer.Indexer`（5 方法）+ `indexer.IndexerAdmin`（5 方法） | 核心瘦身，管理类方法剥离到扩展接口                           |
| `Indexer.NewQuery`                         | `query.New(text)`                                             | 剥离到 query 包                                              |
| `core.FullTextStore`                       | （删除）                                                      | 全文索引废弃                                                 |
| `core.Loader`                              | （删除）                                                      | 与 `document.Open` 重复                                      |
| `core.Region`                              | `core.Region`（保留并明确语义）                               | 对应目录 README.md 的知识库分区；同时作为 Graph 节点存在     |
| `core.ChunkNode`                           | `core.Node`（归一化）                                         | 归一化为 Node 类型，Chunk 不作为 Node 写入 Graph             |
| `Indexer.Tree()`                           | `GraphIndexer.Tree()`                                         | 保留，基于 Region 节点输出知识树                             |
| `indexer.RegionIndexer`                    | `indexer.BuildRegions(rootPath)`                              | 简化为辅助函数                                               |
| `indexer.GraphIndexer`（含 LLM）           | `indexer.GraphIndexer` + `extractor.LLMExtractor`             | LLM 剥离到 extractor 包默认应用实现                          |
| `indexer.FulltextIndexer`                  | （删除）                                                      | 全文索引废弃                                                 |
| `indexer.EntityDef` / `WithSchemas`        | `extractor.LLMExtractor` 内部                                 | 配置内聚到默认应用实现                                       |
| `indexer/prompt*.go`                       | `extractor/llm.go` 内部 Prompt 常量                           | Prompt 模板迁移                                              |
| `indexer/ontologies*.go`                   | `extractor/llm.go` 内部 EntitySchema                          | Schema 迁移                                                  |
| `indexer.IndexData`                        | （删除）                                                      | Ordinal 系统删除                                             |
| `indexer.text2Cypher`                      | `query.Text2Cypher`（保留并迁移）                             | 作为 query 包预实现类，承载自然语言→Cypher 转换能力          |
| `HybridIndexer`                            | （删除）                                                      | 调用方按需组合                                               |
| `core.Hit.Entities/Relations`              | `core.GraphResult`                                            | 职责分离                                                     |
| `result/compress.go`                       | `result/compress.go`（保留）                                  | 当前未用不代表后续不用，作为预实现能力保留                   |
| `query/fulltext.go` / `tree.go`            | （删除）                                                      | 简化                                                         |
| `store/doc/bleve`                          | （删除）                                                      | bleve 依赖移除                                               |
| `chunker/strategy.go`                      | （删除）                                                      | 7 种策略常量，与结构化分块理念冲突                           |
| `chunker/fixed_size.go`                    | （删除）                                                      | 分词式分块，无结构语义                                       |
| `chunker/sentence.go`                      | （删除）                                                      | 分词式分块，无结构语义                                       |
| `chunker/paragraph.go`                     | （删除）                                                      | 分词式分块，无结构语义                                       |
| `chunker/recursive.go`                     | （删除）                                                      | 分词式分块，无结构语义                                       |
| `chunker/semantic.go`                      | （删除）                                                      | LLM 语义分块，与结构化分块理念冲突                           |
| `chunker/parent_doc.go`                    | （删除）                                                      | 父子分块概念与 `Chunk.ParentID` 重复                         |
| `chunker/code.go`（V1 基于 StructureNode） | `chunker/code.go`（重写，基于 AST）                           | 不再依赖被删的 `StructureNode`                               |
| `chunker/image_chunks.go`                  | `chunker/image.go`（保留并适配新接口）                        | 整个图片作为一个 Chunk                                       |
| （新增）                                   | `chunker/markdown.go`：`MarkdownChunker`                      | 按 Markdown "#" 切分章节段落                                 |
| （新增）                                   | `chunker/datum.go`：`DatumChunker`                            | 按数据结构树状分块                                           |
| `core.Chunk`（V1 仅 Content）              | `core.Chunk`（V2 新增 `Title`/`Summary` 字段）                | 多维度向量索引的数据来源：Title/Summary/Content 三维度向量化 |
| `core.Vector`（V1 单向量）                 | `core.Vector`（V2 新增 `Dim` 字段，ID 三种形式）              | 多维度向量索引：`chunkID`/`chunkID:title`/`chunkID:summary`  |
| `embedder.Calc`（V1 单维度向量化）         | `embedder.CalcChunk`（V2 三维度向量化）                       | 多维度向量索引的核心方法                                     |
| `core.Hit`（V1 仅 Content）                | `core.Hit`（V2 新增 `Dim`/`Title`/`Summary` 字段）            | 标识多维度命中来源，便于调试与重排                           |

---

## 附录 B：开发顺序建议

按里程碑顺序开发，但局部可并行：

```
M1 接口定义
   ↓
M2 删除清理 ──┐
   ↓          │
M3 document  │
   ↓          │ 可并行
M4 structurizer
   ↓          │
M5 chunker + extractor
   ↓          │
M6 indexer ───┘
   ↓
M7 query + result + cmd 收尾
```

**关键依赖**：

- M2 必须在 M1 之后（先定义新接口才能删旧的）
- M3 → M4 → M5 → M6 严格串行（每个里程碑依赖前一个的产出）
- M7 必须最后（依赖前面所有里程碑完成）

**完成定义**：

- 每个里程碑完成 = 该里程碑的所有任务完成 + 所有验收通过 + 提交一次 git commit
- 全部里程碑完成 = V2 改造完成，可发布 v2.0.0-alpha.1
