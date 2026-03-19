# 多模态索引管线 (Multimodal Indexing Pipeline)

## 1. 架构概述

基于 `github.com/DotNetAge/gochat/pkg/pipeline` 提供的强类型可编程管线框架，本模块旨在构建一条灵活、高内聚的**单支多模态索引管线**。
该管线的核心目标是实现各类结构化/非结构化文件的自动化摄入，并打通**文本与图片的多模态搜索**能力。

完整的端到端流向为：
**读取文件 (File/Stream) -> 多格式智能解析与分块 (Parser & Chunker) -> 多模态向量生成 (Embedding) -> 多路存储 (VectorStore / DocumentStore / GraphStore)**

## 2. 核心挑战与解决策略：同一向量空间投影

实现多模态（文件与图片）混合搜索的难点在于**如何将文本语义与视觉特征投影到同一个向量空间 (Latent Space) 中**。为了让查询文本能够匹配到相关的图片，或让查询图片能够匹配到相关的文本，我们需要：

1. **选用多模态对齐模型 (Multimodal Alignment Model)**
   基于 `@gochat/pkg/embedding` 库，我们将扩展支持如 **CLIP (Contrastive Language-Image Pretraining)** 或 **Chinese-CLIP (如 `OFA-Sys/chinese-clip-vit-base-patch16`)** 等多模态模型。由于这些模型在预训练时使用了对比学习，文本和图像的输出向量已经对齐在同一维度空间（例如 512 维或 768 维）。

2. **分离解析与统一映射**
   - **Parser 层**：文档解析器（如 PDF、Docx 解析器）在读取文件时，需将原文档拆解为“文本块”和“图片实体”，并保留它们在原文中的邻近上下文关系（上下文锚点）。
   - **Embedding 层**：
     - 如果当前 Chunk 是**文本**，调用模型的 Text Encoder（文本分支）生成稠密向量。
     - 如果当前 Chunk 是**图片**（Base64 或文件引用），调用模型的 Vision Encoder（视觉分支）生成稠密向量。
   - 由于两者输出在同一个向量空间中，它们可以被无缝插入同一个 VectorStore 的同一个 Collection/Index 中。

3. **关联存储策略**
   为了实现图文互搜和图谱问答，数据将被多路路由：
   - **VectorStore**：统一存储所有生成的多模态向量，实现全局余弦相似度（Cosine Similarity）检索。
   - **DocumentStore**：存储原始数据（包含图文内容、元数据、所属文档信息），提供高保真的原始上下文（用于下游大模型生成）。
   - **GraphStore**：对文本 Chunk 提取出的实体（Entities）以及实体间的关系（Relationships）进行构建，支持基于图的推理与发现。

## 3. 管线步骤 (Pipeline Steps) 设计

我们基于现存的 `@pkg/steps/indexing` 进行重新编排与增强：

1. **DiscoverStep (发现与加载)**
   - 获取目标文件元数据（路径、大小、时间等）。
2. **MultimodalParseStep (多路智能解析)**
   - 使用现有的 `stepinx.Multi(parsers...)` 步骤，通过传入系统支持的各种格式解析器集合（如 CSV, DOCX, HTML, Image, Markdown, PDF, PPT, Text, XML, YAML 等），**按文件扩展名自动路由**至对应的 Parser，充分发挥现有解析能力。
   - 各个 Parser 从文件中分离文本段落及内嵌图片，向后游传递带有 `type="text"` 或 `type="image"` 标记的 `core.Document` 流。
3. **SemanticChunkStep (语义与多模态分块)**
   - 对长文本执行滑窗或语义分块（Semantic Chunking）。
   - 对图片执行元数据封装（可选地添加由 VLM 生成的 Image Caption）。
4. **MultimodalEmbedStep (多模态向量生成)**
   - 依赖本地化、可自动下载的 `gochat/pkg/embedding` 库。
   - 根据 Chunk 的模态标识分别调用文本/视觉编码器，产出统一维度的 Vector。
5. **EntityExtractStep (图谱实体抽取)**
   - 提取文本中的实体与关系，打上元数据，准备用于图谱（GraphStore）写入。
6. **MultiStoreStep (多路路由与持久化)**
   - 将 Chunk 原文存入 DocumentStore。
   - 将生成的向量写入 VectorStore。
   - 将抽取的图谱节点/边写入 GraphStore。

## 4. 强类型状态上下文 (Context State)

管线通过强类型泛型（Go 1.18+）传递上下文 `*IndexingState`：

```go
type IndexingState struct {
    // 基础输入
    FilePath string
    Metadata core.Metadata
    
    // 多模态文档流
    Documents <-chan *core.Document
    Chunks    <-chan *core.Chunk // Chunk.Metadata 中包含 "modality": "text" | "image"
    
    // 产出物
    Vectors   []*core.Vector
    Entities  []*core.Entity
    
    // 执行统计
    TotalChunks   int
    TotalImages   int
    TotalEntities int
}
```

## 5. 管线组装示例 (Pipeline Builder)

在 `pkg/indexer/builder.go` 或类似组装类中，构建完整的单支多模态索引服务：

```go
func BuildMultimodalIndexPipeline(
    parsers []core.Parser,                      // 接收所有支持的解析器集合
    chunker core.Chunker, 
    embedder embedding.MultimodalProvider,      // 支持文本与图片的统一对齐提供者
    entityExtractor core.EntityExtractor,
    vectorStore core.VectorStore,
    docStore core.DocumentStore,
    graphStore core.GraphStore,
) *pipeline.Pipeline[*IndexingState] {
    
    p := pipeline.New[*IndexingState]()
    
    // 按序挂载可编程 Step
    p.AddSteps(
        stepinx.Discover(),
        stepinx.Multi(parsers...),                  // ★ 自动根据文件后缀智能路由到正确的解析器
        stepinx.Chunk(chunker),                     // 语义分块
        stepinx.MultimodalEmbed(embedder),          // ★ 核心：投影至同一向量空间
        stepinx.Entities(entityExtractor, logger),  // 抽取图谱网络
        stepinx.Store(vectorStore, docStore, graphStore), // 多端持久化写入
    )
    
    return p
}
```

## 6. 后续演进

1. **Embedding 层扩展**：对齐目前的 `bge` 和 `sentence-bert` 方案，在 `gochat/pkg/embedding` 中补充对等尺寸的多模态视觉模型（如 CLIP）支持，并提供自动下载能力。
2. **多模态图谱关联**：除了存入 VectorStore 外，探讨是否将识别出的“图片 Chunk”作为 Node 注册进 GraphStore，形成跨越文档界限的图文多维关联网络。
