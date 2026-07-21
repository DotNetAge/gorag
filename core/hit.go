package core

// ChunkHit 语义命中的分片:嵌入 *Chunk + 命中信息(分数)。
//
// 设计要点:
//   - 嵌入 *Chunk,客户端可直接访问 hit.Chunks[i].Content(而非 hit.Chunks[i].Chunk.Content)
//   - Score 为该分片的相关性分数(支持细粒度排序)
//   - 不保留命中维度字段:从属向量(title/summary)命中后已通过 resolveDimensions
//     回查主向量替换为完整 Chunk,调用方无需也无需感知命中来源维度
type ChunkHit struct {
	*Chunk
	Score float32 `json:"score"` // 该分片的相关性分数
}

// NodeHit 图命中的实体:嵌入 *Node + 分数。
type NodeHit struct {
	*Node
	Score float32 `json:"score"`
}

// EdgeHit 图命中的关系:嵌入 *Edge + 分数。
type EdgeHit struct {
	*Edge
	Score float32 `json:"score"`
}

// Hit 检索结果容器:持有 Chunks/Nodes/Edges 三类命中数据。
//
// 设计对称性(存取镜像):
//   - StructuredDoc 是索引过程容器(Indexer.Save 接收)
//   - Hit 是检索过程容器(Indexer.Search 返回)
//   - 两者都持有 Chunks/Nodes/Edges 三类数据
//
// 融合:
//   - SemanticIndexer.Search → Hit{Chunks: [...]}
//   - GraphIndexer.SearchGraph → Hit{Nodes: [...], Edges: [...]}
//   - HyperIndexer.Search → Hit{Chunks: [...], Nodes: [...], Edges: [...]}(双线融合)
type Hit struct {
	Query  Query       `json:"-"`              // 触发本次检索的查询对象(不参与 JSON 序列化)
	Score  float32     `json:"score"`          // 综合分数(RRF 融合后的总分)
	Chunks []ChunkHit  `json:"chunks,omitempty"`  // 语义命中的分片(SemanticIndexer 填充)
	Nodes  []NodeHit   `json:"nodes,omitempty"`   // 图命中的实体(GraphIndexer 填充)
	Edges  []EdgeHit   `json:"edges,omitempty"`   // 图命中的关系(GraphIndexer 填充)
}
