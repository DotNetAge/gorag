# 实体 ID 系统设计

本文档记录 goRAG 系统中各类实体的 ID 生成规则、设计意图及关联关系，确保跨数据库、跨分块策略场景下的内容关联不丢失。

---

## 一、ID 类型总览

| ID 类型 | 生成来源 | 存储位置 | 用途 |
|---------|----------|----------|------|
| `DocID` | `RawDocument.ID` | `Chunk.DocID` / `Vector.Metadata["doc_id"]` | 文档级唯一标识，跨数据库关联 |
| `NodeID` | `StructureNode.ID()` | `Chunk.Metadata["node_id"]` / `Vector.Metadata["node_id"]` | 结构节点追溯，用于代码 AST 分块 |
| `ChunkID` | `GenerateChunkID()` | `Vector.ChunkID` | 向量与 Chunk 的映射 |
| `ParentID` | `Chunk.ParentID` | `Chunk.ParentID` / `Vector.Metadata["parent_id"]` | 父子块关联（ParentDoc 策略） |
| `VectorID` | `uuid.NewString()` | `Vector.ID` | 向量数据库中的记录 ID |

---

## 二、ID 生成规则

### 2.1 DocID（文档 ID）

**来源**：`RawDocument.ID`

```go
// document/raw.go:30
func (r *RawDocument) ID() string {
    return utils.GenerateID([]byte(r.Text))
}
```

**设计意图**：基于文档原始文本内容生成 hash，作为文档的全局唯一标识。

---

### 2.2 NodeID（结构节点 ID）

**来源**：`StructureNode.ID()`

```go
// core/entity.go:18-23
func (n *StructureNode) ID() string {
    if n.Text == "" {
        return ""
    }
    return utils.GenerateID([]byte(n.Text))
}
```

**设计意图**：基于 StructureNode 的文本内容生成 hash，用于追溯到原始文档结构（仅 CodeChunker 策略生效）。

---

### 2.3 ChunkID（分块 ID）

**来源**：`GenerateChunkID()`

```go
// chunker/utils.go:62-68
func GenerateChunkID(docID string, index int, content string) string {
    hash := sha256.Sum256([]byte(content))
    hashStr := hex.EncodeToString(hash[:])[:8]
    return fmt.Sprintf("chunk_%s_%d_%s", docID, index, hashStr)
}
```

**格式**：`chunk_{docID}_{index}_{hash8}`

**设计意图**：
- `docID`：关联到文档
- `index`：分块序号
- `hash8`：基于内容的安全 hash，用于内容校验

---

### 2.4 ParentID（父块 ID）

**来源**：`Chunk.ParentID`

由 `ParentDocChunker.establishParentChildRelationships()` 自动设置。

**设计意图**：建立父子块关联，当检索到子块时可替换为父块返回完整上下文。

---

### 2.5 ChunkID 与 NodeID 的关系

**核心结论**：ChunkID 与 NodeID 是两个完全独立的 ID 系统，没有直接的派生或包含关系。

#### 对比表

| 属性 | ChunkID | NodeID |
|------|---------|--------|
| 生成方式 | `GenerateChunkID(docID, index, content)` | `StructureNode.ID()` |
| 格式 | `chunk_{docID}_{index}_{hash8}` | `sha256(text)[:16]` |
| 生成依据 | docID + 序号 + 内容hash | 仅节点文本内容 |
| 数量关系 | 一个 Doc 可有多个 Chunk | 一个 Doc 可有多个 Node |

#### 关联方式

它们通过 **`Metadata["node_id"]`** 建立**应用层关联**，而非 ID 本身的派生关系：

```
CodeChunker 分块时：

StructureNode (node_id = "abc123")
    ├── NodeType: "function"
    ├── Title: "CalculateTotal"
    └── Text: "func CalculateTotal..."

         ↓ isKeyCodeNode() == true
         
Chunk
    ├── ID = "chunk_doc1_0_def456"
    └── Metadata["node_id"] = "abc123"  ←── 手动关联
```

#### 各分块策略的 node_id 存在性

| 策略 | 基于 Node 分块？ | node_id 存在？ |
|------|-----------------|---------------|
| `RecursiveChunker` | ❌ 基于分隔符文本分割 | ❌ 无 |
| `FixedSizeChunker` | ❌ 基于字符位置 | ❌ 无 |
| `SentenceChunker` | ❌ 基于句子边界 | ❌ 无 |
| `ParagraphChunker` | ❌ 基于段落 | ❌ 无 |
| `CodeChunker` | ✅ 基于代码 AST 节点 | ✅ 有 |
| `ParentDocChunker` | 取决于父/子策略 | 子块可能有 |

#### 设计哲学

- **NodeID**：文档结构层面的标识，用于追溯"这段代码属于哪个函数/类"
- **ChunkID**：向量检索层面的标识，用于"这个向量对应哪个分块"

**类比**：就像文件系统中 `inode` 和 `path` 的关系——inode 标识存储结构，path 标识访问路径，它们通过目录项关联，但不是包含关系。

---

## 三、ID 关联链路

### 3.1 完整数据流

```
RawDocument
    │
    ├── ID() ──────────────────────────────→ DocID
    │                                          │
    │                                          ▼
    │                               Chunk.DocID
    │                                          │
    ▼                                          ▼
StructureNode                          Vector.Metadata["doc_id"]
    │                                          │
    ├── ID() ──────────────────────────→ NodeID
    │                                    │
    │                                    ▼
    │                           Vector.Metadata["node_id"]
    │
    ▼
Chunk
    ├── ID ──────────────────────→ Vector.ChunkID
    ├── DocID ───────────────────→ Vector.Metadata["doc_id"]
    ├── ParentID ────────────────→ Vector.Metadata["parent_id"]
    └── Metadata["node_id"] ─────→ Vector.Metadata["node_id"]
```

### 3.2 ParentDoc 父子块关联

```
ParentChunk
    ├── ID = "chunk_doc1_0_abc123"
    ├── ParentID = "" (空，顶级)
    └── Metadata["is_parent"] = true
         │
         │ establishParentChildRelationships()
         │
         ▼
ChildChunk
    ├── ID = "chunk_doc1_3_def456"
    ├── ParentID = "chunk_doc1_0_abc123"
    └── Metadata["is_parent"] = false
```

---

## 四、VectorStore Payload 字段规范

所有存储到向量数据库的 Payload 必须包含以下关键字段：

```go
// embedder/default.go:64-76
meta := make(map[string]any)
maps.Copy(meta, chunk.Metadata)

// 显式存储关键关联字段
meta["doc_id"] = chunk.DocID
meta["parent_id"] = chunk.ParentID
meta["content"] = chunk.Content
meta["mime_type"] = chunk.MIMEType

// CodeChunker 专用
if nodeID, ok := meta["node_id"].(string); ok {
    meta["node_id"] = nodeID
}
```

### Payload 字段清单

| 字段名 | 类型 | 来源 | 必须 | 说明 |
|--------|------|------|------|------|
| `chunk_id` | string | `Vector.ChunkID` | 是 | 向量与 Chunk 的映射键 |
| `doc_id` | string | `Chunk.DocID` | 是 | 文档级关联，跨数据库追溯 |
| `parent_id` | string | `Chunk.ParentID` | 否 | 父子块关联（ParentDoc 策略） |
| `is_parent` | bool | `Chunk.Metadata["is_parent"]` | 否 | 区分父块与子块 |
| `node_id` | string | `StructureNode.ID()` | 否 | 代码结构节点追溯 |
| `content` | string | `Chunk.Content` | 是 | 直接返回，无需二次查询 |
| `mime_type` | string | `Chunk.MIMEType` | 是 | 内容类型标识 |

---

## 五、跨数据库关联保证

### 5.1 设计原则

1. **显式存储**：所有关键关联字段必须显式写入 `Vector.Metadata`，而非隐式依赖数据库特性
2. **字段冗余**：`doc_id`、`parent_id` 同时存在于 `Chunk` 结构和 `Vector.Metadata` 中
3. **可追溯性**：通过 `doc_id` 可在任何向量数据库中追溯到原始文档

### 5.2 跨库查询示例

```go
// 场景：从向量数据库结果构建文档上下文
for _, hit := range hits {
    docID := hit.DocID  // 从 Vector.Metadata["doc_id"] 获取

    // 通过 DocID 关联原始文档存储
    originalDoc := docStore.FindByID(docID)

    // 如果是子块，通过 ParentID 获取父块
    if hit.IsParent == false {
        parentChunk := chunkStore.FindByParentID(hit.ParentID)
        // 使用父块内容作为返回上下文
    }
}
```

---

## 六、实现文件清单

| 文件 | 职责 |
|------|------|
| `gorag/core/entity.go` | `StructureNode.ID()` 定义 |
| `gorag/document/raw.go` | `RawDocument.ID()` 定义 |
| `gorag/chunker/utils.go` | `GenerateChunkID()` 定义 |
| `gorag/chunker/code.go` | `node_id` 写入 `Chunk.Metadata` |
| `gorag/chunker/parent_doc.go` | `parent_id` / `is_parent` 写入 |
| `gorag/embedder/default.go` | 所有 ID 字段写入 `Vector.Metadata` |
| `gorag/indexer/semantic.go` | 从 `Vector.Metadata` 提取 ID 构建 `Hit`，使用 `ChunkID` 去重 |
| `gorag/indexer/fulltext.go` | 从 `SearchResult` 提取 `DocID` 和 `Content` 构建 `Hit` |
| `gorag/indexer/graph_adapter.go` | `extractDocID()` 从 `ChunkID` 解析 `DocID` |
| `gorag/indexer/hybrid.go` | RRF 融合时按 `Hit.ID`（`ChunkID`）分组 |
| `gorag/store/vector/govector/store.go` | Payload 透传到 govector，按 `ChunkID` 删除 |
| `gorag/store/doc/bleve/store.go` | 全文索引，存储 `doc_id`、`content` 字段 |

---

## 七、版本历史

| 版本 | 日期 | 变更说明 |
|------|------|----------|
| 1.0 | 2026-04-13 | 初始文档，记录 DocID/NodeID/ChunkID/ParentID 关联设计 |
| 1.1 | 2026-04-13 | 补充 fulltextIndexer、graphIndexerAdapter、hybridIndexer 的 ID 关联说明 |
