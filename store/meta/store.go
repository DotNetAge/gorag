package meta

import (
	"time"
)

// Store 元数据存储接口，用于管理文档索引元数据和 chunk LLM 处理状态。
type Store interface {
	// ── 文档元数据 ──────────────────────────────────────────────────

	// SaveDocument 保存或更新文档元数据（按 absolute_path UPSERT）。
	SaveDocument(doc *Document) error

	// GetDocumentByPath 按绝对路径查询文档。
	GetDocumentByPath(absPath string) (*Document, error)

	// ListDocuments 按状态过滤文档列表。
	ListDocuments(status string) ([]*Document, error)

	// DeleteDocument 按绝对路径删除文档元数据。
	DeleteDocument(absPath string) error

	// ── Chunk LLM 处理状态 ──────────────────────────────────────────

	// SaveChunkLLMStatus 保存或更新 chunk 的 LLM 处理状态（按 chunk_id UPSERT）。
	SaveChunkLLMStatus(status *ChunkLLMStatus) error

	// GetChunkLLMStatus 按 chunk_id 查询 LLM 处理状态。
	GetChunkLLMStatus(chunkID string) (*ChunkLLMStatus, error)

	// GetChunkLLMStatusesByDocPath 按文档路径查询所有 chunk 的 LLM 处理状态。
	GetChunkLLMStatusesByDocPath(docPath string) ([]*ChunkLLMStatus, error)

	// DeleteChunkLLMStatusByDocPath 删除指定文档的所有 chunk LLM 处理状态。
	DeleteChunkLLMStatusByDocPath(docPath string) error

	// DeleteChunkLLMStatusByChunkID 删除指定 chunk 的 LLM 处理状态。
	DeleteChunkLLMStatusByChunkID(chunkID string) error

	// GetChunksNeedingLLM 查询需要 LLM 处理的 chunk 状态列表。
	// 当 summarized=false 表示需要摘要，refilled=false 表示需要实体提取。
	// limit <= 0 时不限制数量。
	GetChunksNeedingLLM(docPath string, summarized, refilled bool, limit int) ([]*ChunkLLMStatus, error)

	// ── 资源管理 ────────────────────────────────────────────────────

	// Close 关闭数据库。
	Close() error
}

// ChunkLLMStatus Chunk 的 LLM 处理状态。
// 用于追踪哪些 chunk 已完成摘要和实体提取，哪些需要（重新）处理。
type ChunkLLMStatus struct {
	ID              int64      // 自增主键
	ChunkID         string     // chunk 唯一标识
	DocPath         string     // 所属文档的绝对路径
	DocID           string     // 文档 ID（doc.DocID）
	ContentHash     string     // 最近一次 LLM 处理时的内容哈希
	ContentLength   int        // 内容长度（用于变更幅度检查）
	Summarized      bool       // 是否已完成摘要
	LastSummarizedAt *time.Time // 上次摘要时间
	Refilled        bool       // 是否已完成实体提取
	LastRefilledAt   *time.Time // 上次实体提取时间
	CreatedAt       time.Time  // 创建时间
	UpdatedAt       time.Time  // 更新时间
}

// Document 文档元数据。
type Document struct {
	ID           int64
	AbsolutePath string
	FileName     string
	Extension    string
	SizeBytes    int64
	ModifiedAt   time.Time
	ContentHash  string
	Status       string // indexed / failed / partial_deleted
	ChunkIDs     []string
	IndexedAt    *time.Time
	ErrorMessage string
	UpdatedAt    time.Time
}
