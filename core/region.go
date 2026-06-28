package core

// Region 是一个目录级的数据分区，提供跨文档的语义聚合与数据隔离。
//
// 一个 Region 对应一个被监控的目录（watched directory），
// 包含该目录下所有文档的 Chunk 与 Node。
// Region 的 Summary 由 LLM 对目录下所有 Chunk 的摘要聚合而成，
// 可被向量化后用于语义检索与数据过滤。
//
// 生命周期：
//  1. GraphIndexer.AddFile 在写入 Chunk 时计算 region_id = sha256(dir)
//     并存入 Chunk/Vector 的 Metadata
//  2. RegionIndexer.IndexRegion 在目录下所有文件索引完成后被显式调用，
//     从 VectorStore 查询该区域所有 Chunk → LLM 聚合摘要 → 向量化 → 写回 VectorStore
type Region struct {
	ID      string            `json:"id"`      // sha256(dir)
	Title   string            `json:"title"`   // dir 的 basename（文件夹名）
	Summary string            `json:"summary"` // LLM 聚合摘要，可向量化
	Tags    []string          `json:"tags"`    // 聚合标签（合并所有子 Chunk 的 tags）
	Dir     string            `json:"dir"`     // 目录绝对路径
	Meta    map[string]any    `json:"meta,omitempty"`
}
