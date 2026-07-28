package core

// Chunk 元数据键名常量。
// 仅用于中频包内属性，高频跨包属性已提升为 Chunk 结构体字段。
const (
	MetaHeadingLevel       = "heading_level"        // Markdown 标题层级
	MetaHeadingPath        = "heading_path"         // Markdown 标题路径
	MetaSymbolType         = "symbol_type"          // 代码符号类型（function/class/method 等）
	MetaSignature          = "signature"            // 代码符号签名
	MetaVisibility         = "visibility"           // 代码符号可见性（exported/unexported）
	MetaReceiver           = "receiver"             // 代码符号接收者（如 Person）
	MetaPackage            = "package"              // 代码包名
	MetaMimeType           = "mime_type"            // MIME 类型
	MetaDirectory          = "directory"            // 目录路径
	MetaThumbnailSize      = "thumbnail_size"       // 图片缩略图大小
	MetaEntityIDs          = "entity_ids"           // 关联实体 ID 列表
	MetaIsParent           = "is_parent"            // 是否为父文档分块
	MetaIsRegionDescriptor = "is_region_descriptor" // 是否为 README.md 描述的 Region 分片
	MetaRegionGenerated    = "region_generated"     // 是否由 GoRAG 自动生成
)

// RegionDescriptorMarker 是生成式 README.md 文件的内容标记。
// MarkdownChunker 检测到该标记时，将 README.md 作为单个分片处理，不再按 heading 切分。
const RegionDescriptorMarker = "<!-- gorag-region-descriptor generated=\"true\" -->"

// Chunk 分片：可索引的最小语义单元，承载语义线。
//
// 设计要点：
//  1. 双线结构——通过 ParentID 自连结成可追溯的分块树；只在 VectorStore 中存在，
//     不作为 Node 写入 GraphStore；与实体 Node 的关联通过 Node.SourceChunkIDs 反向引用实现。
//  2. 多维度向量索引——Title/Summary/Content 三大属性分别向量化：
//     - Content 向量 ID = chunkID（主向量）
//     - Title 向量 ID = chunkID:title
//     - Summary 向量 ID = chunkID:summary
//     任一维度命中都能定位同一 Chunk，覆盖用户不同查询方式以提高召回率。
//     这里的「维度」是数据维度（data dimension），不是向量空间维度（vector space dimension）。
type Chunk struct {
	ID        string         `json:"id"`                   // Chunk 唯一标识（docID + 序号 + 内容哈希）
	ParentID  string         `json:"parent_id,omitempty"`  // 父 Chunk ID（空表示文档级 Chunk）；建立分块树，支持向上追溯
	DocID     string         `json:"doc_id"`               // 所属文档 ID（RawDoc.ID()）
	Title     string         `json:"title"`                // 分片标题（由 Chunker 从 Markdown heading/代码符号名等提取；向量化为 chunkID:title）
	Summary   string         `json:"summary"`              // 分片摘要（由 Chunker/Summarizer 生成；向量化为 chunkID:summary）
	Content   string         `json:"content"`              // 分片内容（清洗后纯文本；向量化为主向量 chunkID）
	Tags      []string       `json:"tags,omitempty"`       // 标签列表（由 Chunker/用户标注；用于分类和过滤）
	FileName  string         `json:"file_name,omitempty"`  // 文件名（filepath.Base 结果；全小写）
	Dir       string         `json:"dir,omitempty"`        // 目录绝对路径（filepath.Dir 结果；全小写）
	RegionID  string         `json:"region_id,omitempty"`  // 所属 Region ID（目录的 SHA256 哈希；由 Chunker 写入）
	Language  string         `json:"language,omitempty"`   // 代码语言（如 go/python；由 CodeChunker 写入）
	StartLine int            `json:"start_line,omitempty"` // 在源文件中的起始行号（由 CodeChunker 写入）
	EndLine   int            `json:"end_line,omitempty"`   // 在源文件中的结束行号（由 CodeChunker 写入）
	Index     int            `json:"index"`                // 分片在文档中的序号
	StartPos  int            `json:"start_pos"`            // 在原文中的起始字节位置
	EndPos    int            `json:"end_pos"`              // 在原文中的结束字节位置
	Metadata  map[string]any `json:"metadata,omitempty"`   // 扩展元数据（使用 Meta* 常量键名；高频属性已提升为字段）
}
