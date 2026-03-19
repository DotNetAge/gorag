# DocStore

 一、 多管线策略的实现


目前，builder.go 暴露了统一的 Indexer 接口，并提供了两个工厂函数来隔离不同的管线。并且如您所要求，强制绑定了多模态与 GraphStore 的依赖关系：


1. 单纯向量管线 (NewVectorIndexer)：
    - 这是标准的 Text-to-Vector 管道。
    - 流向：Discover -> MultiParse -> Chunk -> BatchEmbed(纯文本) -> MultiStore(VectorStore, DocStore)
    - 这里的 GraphStore 强制为 nil，走轻量级的纯向量存储。
2. 多模态图谱管线 (NewMultimodalGraphIndexer)：
    - 这是高阶图文混合计算管道。
    - 流向：Discover -> MultiParse -> SemanticChunk -> MultimodalEmbed(双塔) -> Entities抽取 -> MultiStore(Vector, Doc, Graph)
    - 强制拦截：函数内部增加了严格的依赖校验：
```go
    if graphStore == nil {
        return nil, fmt.Errorf("multimodal pipeline requires GraphStore support to be successfully enabled")
    }
```

---


二、 DocumentStore 的定位与生命周期角色


在基础 RAG 的认知中，很多人只知道 VectorStore（向量库）。但到了真实的生产环境或高级 RAG 中，DocumentStore 是不可或缺的基石（Source of Truth）。


1. 在【索引阶段】（Indexing Phase）它充当什么角色？
* 承载超大载荷（Payload）：向量数据库（如 Pinecone, Milvus）对 Metadata 的体积通常有严格限制，如果把成千上万字的原始 Text 甚至高分辨率图片的 Base64 全塞进向量数据库的 Metadata
    中，不仅极其昂贵，而且会导致极其严重的性能劣化。
* 职责分离：在我们的 MultiStoreStep 中，VectorStore 仅仅只存：[Vector (512维), ChunkID, 极简过滤Metadata]。而 DocumentStore 负责存：[ChunkID -> 几千字的原始文本 / 原始图片引用 / 标题 /
    上下文结构]。
* 维持层级结构（Hierarchical Tree）：如果一篇文章被分为了 "Document -> Parent Chunk -> Child Chunk"，这种复杂的父子引用树是存在 DocumentStore 中的。


2. 在【查询阶段】（Retrieval Phase），它服务于哪一类 RAG？


绝不仅仅是基础 RAG，DocumentStore 是开启高阶 RAG 模式的物理前提：


* 服务于 Native RAG（基础版）：
    * 在最朴素的 RAG 中，向向量库查询拿到 ChunkID 后，如果向量库没有存原文，就需要根据 ChunkID 去 DocStore 把原文捞出来喂给 LLM。
* 🔥 服务于 Advanced RAG（高阶版 - 核心发力点）：
    * 小到大检索 (Small-to-Big / Parent-Child Retrieval)：向量库通过检索一个极小的 Child Chunk（比如就一句话“苹果发布了新Mac”）命中了，但这层上下文太少大模型无法回答。我们用这个命中的 ID 去
    DocStore 调出它的 Parent Chunk（包含了前后3大段的完整章节）喂给 LLM。只有 DocStore 能实现这种跨层级的读取。
* 服务于 GraphRAG（图谱版）：
    * GraphStore 查出来的是 Entities（实体：例如节点“苹果公司”和“Tim Cook”）。节点身上本身没带多少上下文。我们需要通过 Node -> Source Chunk ID，去 DocStore
    中提取支撑这个节点关系的原始文本段落作为 Grounding（知识溯源）。
* 服务于 Agentic RAG（智能体版）：
    * Agent 可能会在思考（ReAct）后主动调用 Tool：ReadDocument(doc_id="xyz")。这个时候，Agent 不是在做语义向量搜索，而是在进行精确查询（Exact Match Lookup），这个动作就是直接打在 DocStore
    上的（就像查关系型数据库一样）。


总结：
VectorStore 和 GraphStore 都是用来“找 ID 的寻址系统”，而 DocumentStore 才是真正用来“提货的内容仓库”。多模态下，图片和长文本的数据体积激增，DocumentStore 将发挥决定性作用。

---


一个真正能在生产环境落地的高级 RAG（Advanced RAG / GraphRAG）系统，其底层确实是由这“三驾马车”共同支撑的。


这不仅仅是架构上的堆叠，而是由它们各自不可替代的数学和物理特性决定的。

为什么高级 RAG 必须“三库鼎立”？


我们可以用一个简单的类比来理解高级 RAG 的索引数据架构：图书馆系统。


1. 向量数据库 (Vector Database) —— “语义模糊检索仪”
* 物理特性：存储高维浮点数组（Float32 Array）并计算距离（如余弦相似度、欧氏距离）。它极度擅长“近似最近邻搜索（ANN）”。
* 角色：快速定位候选人。比如你说“找一些有关苹果公司新产品的描述”，它通过计算 512 维的张量夹角，瞬间帮你从千万级语料中框选出 Top 10 的 Chunk ID。
* 局限：它不适合存长文本和图片，不能做精确的事务更新，也不能告诉你 A 和 B 的关系。


2. 图数据库 (Graph Database) —— “知识逻辑关系网”
    * 物理特性：存储 Node（实体，如“Tim Cook”）和 Edge（关系，如“CEO_OF”），极度擅长通过图遍历算法（如最短路径、社区发现）挖掘深层逻辑。
    * 角色：解决多跳推理（Multi-Hop Reasoning）和全局总结。如果你问“苹果现任 CEO 之前在哪个公司供职？”，向量搜索可能因为缺乏直接相关的文本块而失效。但图谱可以通过 `Tim Cook -> [WORKED_AT] -> IBM` 的关系链，精准推理出答案。
    * 局限：图数据库里存的都是高度浓缩的短文本（Entity/Relationship），没有原始长文本的细节上下文（Grounding Context），极易让大模型产生幻觉。
3. K/V数据库或文档/SQL数据库 (DocStore) —— “高保真档案馆”
    * 物理特性：以 O(1) 的极高效率，通过唯一键（ID）精确读写海量 JSON/二进制数据（如 MongoDB, Redis, PostgreSQL 的 JSONB，甚至本地的 Badger/BoltDB）。
    * 角色：RAG 系统的 Source of Truth（单一事实来源）。
        * 小到大检索（Parent-Child retrieval）：Vector 找出了 Sentence_ID_001，系统转身去 K/V 库里秒查 DocStore.GetChunk("Sentence_ID_001") 的
        Parent_ID，然后提取包含上下问的三千字完整段落交给大模型。
        * 多模态溯源：存图片原图的 Base64 或是 OSS 对象存储链接。
        * Agentic RAG 查询：大模型 Agent 在规划步骤时，直接下发 ReadDocument("Doc_099") 命令，绕过 Vector 直接命中 K/V 提取长文。
---


在 gorag 中的架构投射


正如我刚刚查看的 gorag/pkg/core/store/docstore.go 源码所展示的，架构设计完全契合了这一点：

```go
1 type DocStore interface {
2     GetDocument(ctx context.Context, docID string) (*core.Document, error)
3     GetChunk(ctx context.Context, chunkID string) (*core.Chunk, error)
4     GetChunksByDocID(ctx context.Context, docID string) ([]*core.Chunk, error)
5 }
```

我们的 `MultiStoreStep` 就是这一理念的执行者：

1. 向量存入 Milvus/Qdrant。
2. 实体关系存入 Neo4j/Memgraph。
3. Document 和 Chunk 的全量图文上下文，通过 SetDocument 和 SetChunks 存入 MongoDB 或 Badger (K/V)。


在高级的 Retriever 查询阶段，这三个库将像齿轮一样咬合运转。这种分层架构（Decoupled Storage）虽然增加了系统部署的复杂性，但它是构建可控、防幻觉、具备强逻辑推理能力的新一代 Agentic RAG
的唯一正解。