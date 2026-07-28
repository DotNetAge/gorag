package meta

import (
	"time"
)

// Store 元数据存储接口，用于管理文档索引元数据。
type Store interface {
	// ── 文档元数据 ──────────────────────────────────────────────────

	// SaveDocument 保存或更新文档元数据（按 absolute_path UPSERT）。
	SaveDocument(doc *Document) error

	// GetDocumentByPath 按绝对路径查询文档。
	GetDocumentByPath(absPath string) (*Document, error)

	// ListDocuments 按状态过滤文档列表。
	ListDocuments(status string) ([]*Document, error)

	// ListDocumentsWithProgress 查询文档列表及其 Chunk 计数。
	// status 为空时返回所有状态；filterPath 非空时按绝对路径前缀过滤。
	ListDocumentsWithProgress(status, filterPath string) ([]*DocumentProgress, error)

	// CountDocumentsByStatus 按索引状态统计文档数量。
	CountDocumentsByStatus() (map[string]int, error)

	// DeleteDocument 按绝对路径删除文档元数据。
	DeleteDocument(absPath string) error

	// ── Token 用量记录 ──────────────────────────────────────────────

	// SaveUsage 保存一次 LLM 调用的 token 用量记录。
	SaveUsage(usage *Usage) error

	// QueryUsages 查询最近的 token 用量记录，按时间倒序。
	// limit <= 0 时返回所有记录。
	QueryUsages(limit int) ([]*Usage, error)

	// QueryTotalUsageStats 查询聚合后的 token 用量统计。
	// 返回所有记录的总 tokens 数以及最近一次用量中的模型名。
	QueryTotalUsageStats() (totalTokens int64, model string, err error)

	// ── 资源管理 ────────────────────────────────────────────────────

	// Close 关闭数据库。
	Close() error
}

// 文档索引状态常量
const (
	DocStatusPending       = "pending"           // 待索引（已扫描但未处理）
	DocStatusIndexing      = "indexing"          // 索引进行中（worker 正在处理）
	DocStatusIndexed       = "indexed"           // 索引完成
	DocStatusFailed        = "failed"            // 索引失败
	DocStatusPartialDeleted = "partial_deleted"  // 部分删除
)

// Document 文档元数据。
type Document struct {
	ID           int64
	AbsolutePath string
	FileName     string
	Extension    string
	SizeBytes    int64
	ModifiedAt   time.Time
	ContentHash  string
	Status       string // pending / indexing / indexed / failed / partial_deleted
	ChunkIDs     []string
	IndexedAt    *time.Time
	ErrorMessage string
	UpdatedAt    time.Time
}

// DocumentProgress 文档维度索引进度汇总。
type DocumentProgress struct {
	AbsolutePath string     `json:"absolute_path"` // 绝对路径
	FileName     string     `json:"file_name"`     // 文件名
	Extension    string     `json:"extension"`     // 扩展名
	SizeBytes    int64      `json:"size_bytes"`    // 文件大小
	ModifiedAt   time.Time  `json:"modified_at"`   // 修改时间
	IndexStatus  string     `json:"index_status"`  // 文档索引状态
	ErrorMessage string     `json:"error_message"` // 错误信息
	IndexedAt    *time.Time `json:"indexed_at"`    // 索引完成时间
	TotalChunks  int        `json:"total_chunks"`  // Chunk 总数
}

// Usage LLM 调用 token 用量记录。
// 字段对应 OpenAI 标准响应中 Usage 对象的全部扁平化字段。
type Usage struct {
	ID                       int64     // 自增主键
	Model                    string    // 模型名（如 gpt-4o）
	Label                    string    // 调用方标记（如 "Summarizer(单条)" / "Refiller"）
	PromptTokens             int       // 输入 tokens 数
	CompletionTokens         int       // 输出 tokens 数
	TotalTokens              int       // 总 tokens 数
	CachedTokens             int       // 缓存命中的 prompt tokens（PromptTokensDetails.CachedTokens）
	PromptAudioTokens        int       // 输入中的音频 tokens（PromptTokensDetails.AudioTokens）
	ReasoningTokens          int       // 推理/思考 tokens（CompletionTokensDetails.ReasoningTokens）
	CompletionAudioTokens    int       // 输出中的音频 tokens（CompletionTokensDetails.AudioTokens）
	AcceptedPredictionTokens int       // 投机采样接受的 tokens（CompletionTokensDetails.AcceptedPredictionTokens）
	RejectedPredictionTokens int       // 投机采样拒绝的 tokens（CompletionTokensDetails.RejectedPredictionTokens）
	CreatedAt                time.Time // 记录时间
}
