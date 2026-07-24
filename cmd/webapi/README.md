# GoRAG Web API 参考手册

## 概述

GoRAG 提供 RESTful HTTP API，覆盖所有 CLI 命令功能。通过 `grag serve` 启动。

## 快速开始

```bash
# 初始化并启动（init-dir 不传路径时默认当前工作目录）
grag serve --port 8080 --init-dir

# 指定 .rag 库目录启动
grag serve --port 8080 --rag-dir /path/to/.rag

# 先初始化再启动
grag serve --port 9000 --init-dir /path/to/project
```

## 通用约定

### 基础 URL

所有 API 端点以 `/api/` 为前缀。

### 响应格式

成功响应：

```json
{
  "success": true,
  "data": { ... }
}
```

错误响应：

```json
{
  "success": false,
  "error": "错误描述信息"
}
```

### HTTP 方法

- `GET` — 查询类操作（无副作用）
- `POST` — 写入类操作（有副作用）
- 不支持的方法返回 `405 Method Not Allowed`

### CORS

所有端点已启用 CORS，允许任意来源的跨域请求。

---

## 端点详情

### 1. 健康检查

```
GET /health
```

检查服务是否运行。

**响应示例：**

```json
{
  "status": "ok"
}
```

---

### 2. 初始化 RAG 库

```
POST /api/init
```

在当前目录或指定目录创建 `.rag` 库。

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `rag_dir` | string | 否 | .rag 库目录路径（默认：`{cwd}/.rag`） |
| `index_type` | string | 否 | 索引器类型：`semantic`、`graph`、`hyper`（默认：`hyper`） |
| `model_id` | string | 否 | HuggingFace 模型 ID（默认：`Xenova/chinese-clip-vit-base-patch16`） |
| `model_file` | string | 否 | 模型文件名（默认：`onnx/model.onnx`） |
| `model_path` | string | 否 | 本地模型文件路径（与 `model_id` 二选一） |

**请求示例：**

```json
{
  "index_type": "hyper",
  "model_id": "Xenova/chinese-clip-vit-base-patch16",
  "model_file": "onnx/model.onnx"
}
```

**响应示例：**

```json
{
  "success": true,
  "data": {
    "rag_dir": "/path/to/.rag",
    "index_type": "hyper",
    "model_path": "/home/user/.embeddings/Xenova/chinese-clip-vit-base-patch16/onnx/model.onnx",
    "indexer_name": "hyper"
  }
}
```

---

### 3. 索引文件

```
POST /api/index
```

对指定文件或目录执行批量索引（快路径，无 LLM 处理）。

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | 否 | 目标文件或目录路径（默认：`.`） |

**请求示例：**

```json
{
  "path": "./docs/"
}
```

**响应示例：**

```json
{
  "success": true,
  "data": {
    "message": "索引完成",
    "target": "/absolute/path/to/docs"
  }
}
```

---

### 4. 搜索查询

```
POST /api/query
```

执行语义/混合搜索查询，支持多关键字组（以 `|` 分隔）。

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `text` | string | 是 | 查询文本（多关键字以 `|` 分隔） |
| `top_k` | int | 否 | 返回结果数量上限（默认：10） |
| `filter_path` | string | 否 | 按 source 路径前缀过滤 |
| `format` | string | 否 | 输出格式：`json`、`prompt`、`terminal`（默认：`json`） |
| `show_score` | bool | 否 | 是否显示相似度分数（仅 `prompt`/`terminal` 格式有效） |
| `show_doc_id` | bool | 否 | 是否显示文档 ID（仅 `terminal` 格式有效） |
| `content_max` | int | 否 | 内容最大显示长度（仅 `prompt`/`terminal` 格式有效） |

**请求示例：**

```json
{
  "text": "机器学习",
  "top_k": 5,
  "filter_path": "./docs/"
}
```

**响应示例（默认 json 格式）：**

```json
{
  "success": true,
  "data": {
    "query": {
      "type": "semantic",
      "text": "机器学习"
    },
    "score": 0.85,
    "chunks": [
      {
        "chunk": {
          "id": "abc123",
          "title": "机器学习概述",
          "summary": "机器学习是人工智能的一个分支...",
          "content": "机器学习是人工智能的一个分支，主要研究如何...",
          "source": "/path/to/docs/ml.md",
          "tags": ["AI", "ML"],
          "start_line": 1,
          "end_line": 50
        },
        "score": 0.85
      }
    ]
  }
}
```

---

### 5. 查看库信息

```
GET /api/info
```

获取 RAG 库的配置、存储大小和索引统计信息。

**响应示例：**

```json
{
  "success": true,
  "data": {
    "config": {
      "version": 1,
      "storage": { "vectors_dir": "vectors", "graphs_dir": "graphs", "logs_dir": "logs", "meta_db": "meta.db" },
      "embedding": { "model_file": "...", "dimension": 512 },
      "llm": { "base_url": "", "model": "", "language": "Chinese" },
      "indexer": { "type": "hyper" },
      "query": { "semantic_weight": 0.8, "graph_weight": 0.2 }
    },
    "config_yaml": "version: 1\n...",
    "abs_path": "/path/to/.rag",
    "sizes": { "total": 1048576, "vectors": 524288, "graphs": 262144, "logs": 1024 },
    "vector_count": 150,
    "graph_nodes": 45,
    "graph_edges": 120
  }
}
```

---

### 6. 诊断配置

```
GET /api/doctor
```

诊断 RAG 库的配置完整性。

**响应示例：**

```json
{
  "success": true,
  "data": [
    { "name": "config.yml", "ok": true, "hint": "" },
    { "name": "embedding.model_file", "ok": false, "hint": "运行：grag config embedder <path>" },
    { "name": "向量库目录", "ok": true, "hint": "" },
    { "name": "图库目录", "ok": true, "hint": "" },
    { "name": "meta.db", "ok": true, "hint": "" }
  ]
}
```

---

### 7. 查看日志

```
GET /api/logs
```

返回 RAG 库的日志文件内容（纯文本）。

**响应头：** `Content-Type: text/plain; charset=utf-8`

---

### 8. 增量更新

```
POST /api/update
```

对指定路径执行增量更新（重新索引 + LLM 增强）。

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `path` | string | 否 | 目标文件或目录路径（默认：`.`） |

**请求示例：**

```json
{
  "path": "./src/"
}
```

**响应示例：**

```json
{
  "success": true,
  "data": {
    "message": "增量更新完成",
    "target": "/absolute/path/to/src"
  }
}
```

---

### 9. 查看文件索引树

```
GET /api/tree
```

基于已索引 Chunk 的 Source 属性重建文件目录树。

**响应示例：**

```json
{
  "success": true,
  "data": {
    "name": ".",
    "is_dir": true,
    "children": [
      {
        "name": "docs",
        "is_dir": true,
        "summary": "项目文档目录",
        "children": [
          {
            "name": "guide.md",
            "path": "/path/to/docs/guide.md",
            "size": 10240,
            "chunks": [
              {
                "type": "文档",
                "title": "快速入门指南",
                "summary": "本文档介绍如何快速上手...",
                "start_line": 1,
                "end_line": 30
              }
            ]
          }
        ]
      }
    ]
  }
}
```

---

### 10. 分页列出 Chunk

```
GET /api/chunks?page=1&size=20&filter=./docs/
```

分页查询已索引的 Chunk。

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `page` | int | 1 | 页码（从 1 开始） |
| `size` | int | 20 | 每页数量 |
| `filter` | string | "" | 按 source 路径前缀过滤 |

**响应示例：**

```json
{
  "success": true,
  "data": {
    "page": 1,
    "size": 20,
    "total": 150,
    "items": [
      {
        "id": "chunk_001",
        "title": "机器学习概述",
        "summary": "机器学习是人工智能的一个分支...",
        "content": "...",
        "source": "/path/to/docs/ml.md",
        "tags": ["AI", "ML"],
        "start_line": 1,
        "end_line": 50,
        "metadata": { ... }
      }
    ]
  }
}
```

---

### 11. 图节点查询

```
POST /api/nodes
```

以指定目录的 Region 节点为起点，查询多跳相邻的图节点与关系。

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `dir` | string | 否 | 目录路径（默认：当前工作目录） |
| `hops` | int | 否 | 跳数，范围 1-3（默认：1） |

**请求示例：**

```json
{
  "dir": "./docs/",
  "hops": 2
}
```

**响应示例：**

```json
{
  "success": true,
  "data": {
    "region_id": "region_abc",
    "region_name": "docs",
    "nodes": [
      {
        "id": "node_001",
        "name": "机器学习",
        "labels": ["Concept"],
        "properties": { ... },
        "source_chunk_ids": ["chunk_001"]
      }
    ],
    "edges": [
      {
        "id": "edge_001",
        "type": "RELATES_TO",
        "source": "node_001",
        "target": "node_002",
        "properties": { ... }
      }
    ]
  }
}
```

---

### 12. Cypher 查询

```
POST /api/cypher
```

直接对底层图存储执行 Cypher 查询。

**请求体：**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `query` | string | 是 | Cypher 查询语句 |

**请求示例：**

```json
{
  "query": "MATCH (n:Concept) RETURN n.name, n.id LIMIT 10"
}
```

**响应示例：**

```json
{
  "success": true,
  "data": [
    { "n.name": "机器学习", "n.id": "node_001" },
    { "n.name": "深度学习", "n.id": "node_002" }
  ]
}
```

---

### 13. 查看索引状态

```
GET /api/status?filter=./src&status=pending&summary=false
```

查看文件的索引与 LLM 处理进度。

**查询参数：**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `filter` | string | "" | 按绝对路径前缀过滤 |
| `status` | string | "" | 按索引状态过滤：`pending`、`indexing`、`indexed`、`failed` |
| `summary` | bool | false | 设置为 `true` 仅返回汇总统计 |

**响应示例（summary=false）：**

```json
{
  "success": true,
  "data": [
    {
      "absolute_path": "/path/to/file.md",
      "index_status": "indexed",
      "llm_status": "done",
      "total_chunks": 5,
      "summarized_count": 5,
      "refilled_count": 3,
      "error_message": ""
    }
  ]
}
```

**响应示例（summary=true）：**

```json
{
  "success": true,
  "data": {
    "pending": 2,
    "indexing": 1,
    "indexed": 45,
    "failed": 0,
    "partial_deleted": 0
  }
}
```

---

## 错误码

| HTTP 状态码 | 说明 |
|-------------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误（如缺少必填字段） |
| 405 | HTTP 方法不支持 |
| 500 | 服务端内部错误 |

所有错误均在响应体中返回中文描述。
