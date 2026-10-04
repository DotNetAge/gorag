# Native RAG

这是一个最朴素的RAG，就是做了一个语义索引与查询层；不涉及文档解析、分块、图谱等完整流水线，适用于技能注册表、工具目录、配置项语义匹配这类「键值条目 + 语义检索」场景。

## 核心设计

- **族 = Collection**：`<族>` 直接对应底层一个 Collection（bbolt bucket），一库（单个 db 文件）多族，共享一个存储连接。族级物理隔离是正确性必需——底层 HNSW 检索是 post-filtering 策略（先取 topK*10 近邻再过滤），多族混存会互相挤占过打名额导致漏召回。
- **一条目多记录**：主值向量化为主体记录（ID = `族:sha256(值)`）；meta 的每个键值对额外生成一条子键记录（ID = `族:键:sha256(键值)`），并冗余主值与完整元数据——任一维度命中即可召回该条目，且命中即返回完整条目，无需二次查询。
- **schema 记录**：每族的子键名集合存为一条特殊记录（ID = `__schema__`），检索时被排除，供定向子键检索使用。
- **幂等覆盖**：同族同值重复 Add 即更新索引；不同条目若某子键值完全相同，视为同一个东西，后写覆盖。
- **一次搜索即聚合**：主记录与子键记录同池检索，结果按条目去重取最高分（过采 4 倍后聚合），不再需要分键多路搜索。

## 用法

增加索引

```bash
seg add <族> <值> <Meta|可选> # 族名即 collection 名，默认库文件 native.db
seg add skill "websearch" { "name":"websearch", "title": "网页搜索", "description": "搜索网络上内容"}
seg add skill:title "网页搜索"
```

伪代码用法:

```go
seg, err := native.NewSegIndexer("/绝对路径/native.db", embedder)
if err != nil { ... }

// meta 的每个键值对都会生成一条子键向量（title/description 可分别命中）
err = seg.Add(ctx, "skill", "websearch", map[string]string{
  "name":        "websearch",
  "title":       "网页搜索",
  "description": "搜索网络上内容",
})
```

搜索

```bash
seg search <键> 目标 <值> -t 10 # -t 代表topK，默认为1
seg search skill "搜索"           # 主值 + 全部子键联合检索
seg search skill:title "网页"     # 仅在 title 子键上定向检索
```

代码用法

```go
results, err := seg.Search(ctx, "skill", "搜索", 1) // 末行为 topK
print(results[0].Value)  // "websearch"
print(results[0].Meta)   // 完整元数据（无论命中主值还是子键）
print(results[0].Field)  // 命中来源字段，"" 表示主值本身命中
```

返回结构为专用的 `SegHit`（Key / Field / Value / Meta / Score），不复用 `core.Hit`——本包命中即「一条 KV 条目 + 分数」，无需背上 Chunks/Nodes/Edges 重容器的包袱。

## 行为边界

- 检索无相似度阈值：族内数据极少时，零相似度记录也会作为「最近邻」返回；需要阈值语义由调用方按 `Score` 自行截断。
- 检索键 `族:子键` 中冒号是语法保留字符，族名不可包含冒号。
- 一个索引器实例绑定一个 embedder（全族维度统一）；族在首次 Add 时惰性创建。
