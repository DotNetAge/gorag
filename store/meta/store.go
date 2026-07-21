package meta

import (
	"time"
)

// Store 元数据存储接口，用于管理文档索引元数据。
type Store interface {
	// SaveDocument 保存或更新文档元数据（按 absolute_path UPSERT）。
	SaveDocument(doc *Document) error

	// GetDocumentByPath 按绝对路径查询文档。
	GetDocumentByPath(absPath string) (*Document, error)

	// ListDocuments 按状态过滤文档列表。
	ListDocuments(status string) ([]*Document, error)

	// DeleteDocument 按绝对路径删除文档元数据。
	DeleteDocument(absPath string) error

	// Close 关闭数据库。
	Close() error
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
