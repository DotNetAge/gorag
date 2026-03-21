# Document Chunker (文档分块器)

在检索增强生成 (RAG) 系统中，分块（Chunking）是将长文档转换为适合向量化和 LLM 处理的小片段的关键步骤。本包 (`pkg/indexing/chunker`) 提供了多种分块器实现，以满足不同场景下的性能、成本和检索精度需求。

## 为什么需要分块器？

1. **突破模型限制**：所有的大语言模型 (LLM) 和向量模型 (Embedding Models) 都有上下文窗口限制（如 512, 1024 或 8191 tokens）。长文档必须切分后才能被处理。
2. **提升检索精度**：如果将一整篇包含多个主题的文章作为单个向量存储，其语义特征会被严重稀释（"Lost in the middle"）。将文档切分为主题聚焦的小块，能大幅提高向量相似度检索的准确率。
3. **控制 API 成本**：按需检索和组装最小上下文，可以减少向 LLM 发送的无关 token 数量，从而降低 API 调用成本并提升响应速度。

## 基本原理：Size 与 Overlap

无论使用哪种底层切分算法，所有分块器都依赖两个核心参数：
- **`ChunkSize` (分块大小)**：每个片段的最大容量（按字符或 Token 计算）。
- **`ChunkOverlap` (重叠大小)**：相邻两个分块之间重复的容量。它的作用是**防止关键上下文在硬切分时断裂**（例如一句话被生硬地切成两半）。

> **经验法则**：如果你不确定如何设置，可以从 `ChunkSize = 1000`, `ChunkOverlap = 150` 开始。

---

## 我们提供的分块器

### 1. CharacterChunker (基于字符的分块器)

`CharacterChunker` 是最基础、速度最快的分块器。它直接根据字符串的 Rune（字符）数量进行切分。

- **工作原理**：通过硬性的字符数限制进行截断。生产级实现通常会结合分隔符（`\n\n`, `\n`, ` `）进行递归切分，以尽量保证段落或句子的完整性。
- **优点**：计算开销极小，速度极快，不依赖任何外部模型或词表。
- **适用场景**：
  - 本地快速原型开发。
  - 处理不需要极高语义精度的普通日志、纯文本数据。
- **使用方式**：
  ```go
  // 创建一个按 1000 字符切分，重叠 150 字符的 Chunker
  chunker := chunker.NewCharacterChunker(1000, 150)
  chunks, err := chunker.Chunk(ctx, doc)
  ```

### 2. TokenChunker (基于 Token 的分块器)

`TokenChunker` 是一种更精确的兜底分块器。它引入了 `tiktoken-go`，将文本编码为与 OpenAI 等模型底层一致的 Token 数组后进行切分。

- **工作原理**：将字符串转换为 `[]int` (Tokens)，按确切的 Token 数量切分后再 Decode 回字符串。
- **为什么需要它**：1000个中文字符通常会产生 1500~2000 个 Token，极易导致开源 Embedding 模型（通常上限 512 tokens）报错或截断丢失信息。基于 Token 切分能做到**绝对安全**。
- **优点**：严格保证生成的 Chunk 绝对不会超出目标模型的 Token 限制；能精准控制 API 计费成本。
- **适用场景**：
  - 对 API 成本和 Token 上限有严格要求的生产环境。
  - 处理中英混合、包含大量代码或特殊符号的复杂文档。
- **使用方式**：
  ```go
  // 创建一个严格按 500 Token 切分，重叠 50 Token 的 Chunker，使用 OpenAI cl100k_base 词表
  chunker, err := chunker.NewTokenChunker(500, 50, "cl100k_base")
  chunks, err := chunker.Chunk(ctx, doc)
  ```

### 3. SemanticChunker (高级语义分块器)

`SemanticChunker` 实现了 `core.SemanticChunker` 接口，它不是一种基础的文本切割算法，而是采用了**装饰器模式（Decorator）**，包装了基础的 Chunker，并为其赋予了 **Advanced RAG (高级 RAG)** 的能力。

它解决了传统 RAG 中“块切得太小导致丢失上下文，块切得太大导致检索不准”的核心矛盾。

#### 功能 A：HierarchicalChunk (层级/父子分块)
- **原理**：将文档先切分为大块（Parent，如 1000 字符），再将每个大块细分为小块（Child，如 250 字符）。系统将 Parent 和 Child 都进行向量化存储。
- **优势**：检索时更容易命中语义高度聚焦的 Child 块（提高精度）；但在组装提示词发送给 LLM 时，系统会替换为其对应的 Parent 块（提供完整的上下文背景）。
- **适用场景**：事实问答、深度研报分析、法律合同问答。

#### 功能 B：ContextualChunk (上下文感知分块)
- **原理**：在生成每个 Chunk 时，将整个文档的全局摘要（Summary）硬注入到该 Chunk 的内容或元数据中。
- **优势**：使得原本孤立的文本片段（"这个决定是正确的"）具有了全局视角（"文档背景：2023年Q3财报会议... Chunk内容：这个决定是正确的"），大幅减少 LLM 断章取义产生的幻觉。

- **使用方式**：
  ```go
  // 1. 创建基础分块器
  baseChunker := chunker.NewTokenChunker(1000, 100, "")
  
  // 2. 创建语义分块器 (父块 1000, 子块 250)
  semanticChunker := chunker.NewSemanticChunker(baseChunker, 1000, 250, 50)
  
  // 生成父子层级结构
  parents, children, err := semanticChunker.HierarchicalChunk(ctx, doc)
  ```

## 演进路线参考

- **Lv1: 跑通流程** -> 使用 `CharacterChunker`。
- **Lv2: 生产稳定** -> 换用 `TokenChunker`，彻底消除长度越界报错。
- **Lv3: 提升质量** -> 引入 `SemanticChunker` 开启父子分块检索。

---

## 最佳实践：如何选择与配置？

作为初学者，面对这三种分块器可能会感到困惑。以下是具体的选择指南与在 GoRAG 中的实际配置示例：

### 1. 怎么选？(决策树)

- **场景 1：我只是在本地测试，或者我的数据全是英文、结构很简单。**
  👉 **选择 `CharacterChunker`**。
  - *原因*：最简单，不需要下载词表，启动最快。

- **场景 2：我要把系统部署到生产环境，数据包含大量中文、代码，我怕调用 OpenAI/Milvus 时因为 "Token exceeded" 报错。**
  👉 **选择 `TokenChunker`**。
  - *原因*：它能精确计算 Token 消耗，是生产环境的绝对安全线。

- **场景 3：我的回答总是“断章取义”，或者我想实现最高精度的 RAG（如多跳推理、长文总结）。**
  👉 **选择 `SemanticChunker`**。
  - *原因*：它支持父子层级检索（找得准 + 看得全）和上下文注入，是解决复杂问答的终极武器。

### 2. 在 GoRAG 中如何初始化？

在 GoRAG 的 `indexer` 配置中，你可以通过 `WithChunker` 选项来注入你选择的分块器。为了方便初学者，所有分块器都提供了**开箱即用的默认版本**。

#### 示例 A：使用基础字符分块器 (极简起步)
```go
import "github.com/DotNetAge/gorag/pkg/indexing/chunker"

// 开箱即用：默认 1000 字符大小，150 字符重叠
c := chunker.DefaultCharacterChunker()

idx, err := indexer.NewBuilder().
    // ... 其他配置
    WithChunker(c).
    Build()
```

#### 示例 B：使用精确的 Token 分块器 (生产推荐)
```go
import "github.com/DotNetAge/gorag/pkg/indexing/chunker"

// 开箱即用：默认 500 Token，50 重叠，使用 OpenAI cl100k_base 词表
c, err := chunker.DefaultTokenChunker()
if err != nil {
    panic(err)
}

idx, err := indexer.NewBuilder().
    // ... 其他配置
    WithChunker(c).
    Build()
```

#### 示例 C：开启高级父子层级分块 (Advanced RAG)
```go
import "github.com/DotNetAge/gorag/pkg/indexing/chunker"

// 开箱即用：自动内置 Token 分块器
// 默认切分比例：父块 1000 Token, 子块 250 Token, 重叠 50 Token
semChunker, err := chunker.DefaultSemanticChunker()
if err != nil {
    panic(err)
}

// 可在自定义的解析流程中使用高级方法
// parents, children, err := semChunker.HierarchicalChunk(ctx, doc)
```

> **总结**：作为初学者，完全不需要纠结 `ChunkSize` 或 `Overlap` 填什么数字。直接调用 `DefaultXXXChunker()`，GoRAG 已经为你预设了经过业界验证的最佳参数。等你熟悉后，再使用 `NewXXXChunker(size, overlap)` 进行微调。