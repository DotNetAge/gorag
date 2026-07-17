package core

// Region 是一个目录级的数据分区，提供跨文档的语义聚合与数据隔离。
//
// 一个 Region 对应一个被监控的目录（watched directory），
// 包含该目录下所有文档的 Chunk 与 Node。
// Region 的 Summary 由 LLM 对目录下所有 Chunk 的摘要聚合而成，
// 写入目录下的 README.md 文件，再通过 GraphIndexer 索引为普通文档。
//
// 生命周期：
//  1. GraphIndexer.AddFile 在写入 Chunk 时计算 region_id = sha256(dir)
//     并存入 Chunk/Vector 的 Metadata
//  2. RegionIndexer.IndexRegion 在目录下所有文件索引完成后被显式调用：
//     - 若目录下 README.md 已存在 → 直接复用，跳过 LLM 聚合
//     - 若不存在 → 从 VectorStore 查询该区域所有 Chunk → LLM 聚合摘要 → 写入 README.md
//  3. 编排层将 README.md 通过 GraphIndexer.AddFile 索引，
//     其内容被分块、提取实体/关系，写入 VectorStore + GraphStore
type Region struct {
	ID      string         `json:"id"`      // sha256(dir)
	Title   string         `json:"title"`   // dir 的 basename（文件夹名，无后缀）
	Summary string         `json:"summary"` // LLM 聚合摘要，写入 README.md（复用已有文件时为空）
	Tags    []string       `json:"tags"`    // 聚合标签（合并所有子 Chunk 的 tags）
	Dir     string         `json:"dir"`     // 目录绝对路径
	Meta    map[string]any `json:"meta,omitempty"`
}
