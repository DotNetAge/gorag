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

	// ListDocumentsWithProgress 查询文档列表及其 Chunk LLM 处理进度汇总。
	// status 为空时返回所有状态；filterPath 非空时按绝对路径前缀过滤。
	ListDocumentsWithProgress(status, filterPath string) ([]*DocumentProgress, error)

	// CountDocumentsByStatus 按索引状态统计文档数量。
	CountDocumentsByStatus() (map[string]int, error)

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

	// CountLLMStatus 聚合统计所有 chunk 的 LLM 处理状态。
	// 返回 total_chunks（总数）、summarized（已摘要数）、refilled（已实体提取数）。
	CountLLMStatus() (totalChunks, summarized, refilled int, err error)

	// GetChunksNeedingLLM 查询需要 LLM 处理的 chunk 状态列表。
	// 当 summarized=false 表示需要摘要，refilled=false 表示需要实体提取。
	// limit <= 0 时不限制数量。
	GetChunksNeedingLLM(docPath string, summarized, refilled bool, limit int) ([]*ChunkLLMStatus, error)

	// ResetAllLLMStatus 将所有 chunk 的 LLM 处理状态重置为未处理。
	// 用于清理之前被错误标记的 chunk，使 Update 能真正执行 LLM 处理。
	ResetAllLLMStatus() error

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

// 文档索引状态常量
const (
	DocStatusPending        = "pending"          // 待索引（已扫描但未被 worker 拾取）
	DocStatusIndexing       = "indexing"          // 索引进行中（worker 正在处理）
	DocStatusIndexed        = "indexed"           // 索引完成
	DocStatusFailed         = "failed"            // 索引失败
	DocStatusPartialDeleted = "partial_deleted"   // 部分删除
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

// DocumentProgress 文档维度索引与 LLM 处理进度汇总。
type DocumentProgress struct {
	AbsolutePath    string     `json:"absolute_path"`     // 绝对路径
	FileName        string     `json:"file_name"`         // 文件名
	Extension       string     `json:"extension"`         // 扩展名
	SizeBytes       int64      `json:"size_bytes"`        // 文件大小
	ModifiedAt      time.Time  `json:"modified_at"`       // 修改时间
	IndexStatus     string     `json:"index_status"`      // 文档索引状态
	ErrorMessage    string     `json:"error_message"`     // 错误信息
	IndexedAt       *time.Time `json:"indexed_at"`        // 索引完成时间
	TotalChunks     int        `json:"total_chunks"`      // Chunk 总数
	SummarizedCount int        `json:"summarized_count"`  // 已完成摘要的 Chunk 数
	RefilledCount   int        `json:"refilled_count"`    // 已完成实体提取的 Chunk 数
	LLMStatus       string     `json:"llm_status"`        // 派生 LLM 状态: none / partial / done
}

// LLM 派生状态常量
const (
	LLMStatusNone    = "none"    // 所有 Chunk 均未处理
	LLMStatusPartial = "partial" // 部分 Chunk 已处理
	LLMStatusDone    = "done"    // 所有 Chunk 已处理（summarized=true && refilled=true）
)

// DeriveLLMStatus 根据 Chunk 汇总统计计算派生 LLM 处理状态。
func DeriveLLMStatus(total, summarized, refilled int) string {
	if total == 0 {
		return LLMStatusNone
	}
	if summarized >= total && refilled >= total {
		return LLMStatusDone
	}
	if summarized > 0 || refilled > 0 {
		return LLMStatusPartial
	}
	return LLMStatusNone
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
