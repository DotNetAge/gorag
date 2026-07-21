package meta

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteStore 基于 SQLite 的元数据存储实现。
type sqliteStore struct {
	db *sql.DB
}

// NewSQLiteStore 创建基于 SQLite 的元数据存储。
//
// 参数：
//   - dbPath: 数据库文件路径（必须非空）
//
// 自动启用 WAL 模式以支持并发读 + 单写。
// 自动创建 documents 表（若不存在）。
func NewSQLiteStore(dbPath string) (Store, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("meta.NewSQLiteStore: 数据库路径不能为空")
	}
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("meta.NewSQLiteStore: 打开数据库失败: %w", err)
	}
	// 设置连接池限制（WAL 模式可支持并发读）
	db.SetMaxOpenConns(1) // SQLite 只支持单写

	s := &sqliteStore{db: db}
	if err := s.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("meta.NewSQLiteStore: 初始化数据库失败: %w", err)
	}
	return s, nil
}

// init 创建 documents 表（若不存在）。
func (s *sqliteStore) init() error {
	query := `CREATE TABLE IF NOT EXISTS documents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		absolute_path TEXT UNIQUE NOT NULL,
		file_name TEXT NOT NULL DEFAULT '',
		extension TEXT DEFAULT '',
		size_bytes INTEGER DEFAULT 0,
		modified_at TIMESTAMP,
		content_hash TEXT DEFAULT '',
		status TEXT NOT NULL DEFAULT '',
		chunk_ids TEXT DEFAULT '[]',
		indexed_at TIMESTAMP,
		error_message TEXT DEFAULT '',
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := s.db.Exec(query); err != nil {
		return fmt.Errorf("创建 documents 表失败: %w", err)
	}
	// 创建索引
	for _, idx := range []string{
		"CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status)",
		"CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(content_hash)",
	} {
		if _, err := s.db.Exec(idx); err != nil {
			return fmt.Errorf("创建索引失败: %w", err)
		}
	}
	return nil
}

// SaveDocument 实现 Store 接口。按 absolute_path UPSERT。
func (s *sqliteStore) SaveDocument(doc *Document) error {
	if doc == nil {
		return fmt.Errorf("SaveDocument: doc 不能为空")
	}
	if doc.AbsolutePath == "" {
		return fmt.Errorf("SaveDocument: AbsolutePath 不能为空")
	}
	chunkIDsJSON := "[]"
	if len(doc.ChunkIDs) > 0 {
		data, err := json.Marshal(doc.ChunkIDs)
		if err != nil {
			return fmt.Errorf("SaveDocument: 序列化 ChunkIDs 失败: %w", err)
		}
		chunkIDsJSON = string(data)
	}
	ext := ""
	if doc.Extension != "" {
		ext = doc.Extension
	} else {
		ext = extractExt(doc.AbsolutePath)
	}
	fileName := doc.FileName
	if fileName == "" {
		fileName = extractFileName(doc.AbsolutePath)
	}

	query := `INSERT INTO documents
		(absolute_path, file_name, extension, size_bytes, modified_at, content_hash, status, chunk_ids, indexed_at, error_message, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(absolute_path) DO UPDATE SET
		file_name=excluded.file_name,
		extension=excluded.extension,
		size_bytes=excluded.size_bytes,
		modified_at=excluded.modified_at,
		content_hash=excluded.content_hash,
		status=excluded.status,
		chunk_ids=excluded.chunk_ids,
		indexed_at=excluded.indexed_at,
		error_message=excluded.error_message,
		updated_at=CURRENT_TIMESTAMP`

	_, err := s.db.Exec(query,
		doc.AbsolutePath,
		fileName,
		ext,
		doc.SizeBytes,
		doc.ModifiedAt,
		doc.ContentHash,
		doc.Status,
		chunkIDsJSON,
		doc.IndexedAt,
		doc.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("SaveDocument: 写入失败: %w", err)
	}
	return nil
}

// GetDocumentByPath 实现 Store 接口。
func (s *sqliteStore) GetDocumentByPath(absPath string) (*Document, error) {
	if absPath == "" {
		return nil, fmt.Errorf("GetDocumentByPath: 路径不能为空")
	}
	row := s.db.QueryRow(`SELECT
		id, absolute_path, file_name, extension, size_bytes, modified_at,
		content_hash, status, chunk_ids, indexed_at, error_message, updated_at
		FROM documents WHERE absolute_path = ?`, absPath)

	return scanDocument(row)
}

// ListDocuments 实现 Store 接口。
func (s *sqliteStore) ListDocuments(status string) ([]*Document, error) {
	var rows *sql.Rows
	var err error
	if status == "" {
		rows, err = s.db.Query(`SELECT
			id, absolute_path, file_name, extension, size_bytes, modified_at,
			content_hash, status, chunk_ids, indexed_at, error_message, updated_at
			FROM documents ORDER BY absolute_path`)
	} else {
		rows, err = s.db.Query(`SELECT
			id, absolute_path, file_name, extension, size_bytes, modified_at,
			content_hash, status, chunk_ids, indexed_at, error_message, updated_at
			FROM documents WHERE status = ? ORDER BY absolute_path`, status)
	}
	if err != nil {
		return nil, fmt.Errorf("ListDocuments: 查询失败: %w", err)
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("ListDocuments: 扫描行失败: %w", err)
		}
		docs = append(docs, doc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListDocuments: 迭代失败: %w", err)
	}
	return docs, nil
}

// DeleteDocument 实现 Store 接口。
func (s *sqliteStore) DeleteDocument(absPath string) error {
	if absPath == "" {
		return fmt.Errorf("DeleteDocument: 路径不能为空")
	}
	result, err := s.db.Exec("DELETE FROM documents WHERE absolute_path = ?", absPath)
	if err != nil {
		return fmt.Errorf("DeleteDocument: 删除失败: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("DeleteDocument: 未找到记录: %s", absPath)
	}
	return nil
}

// Close 实现 Store 接口。
func (s *sqliteStore) Close() error {
	return s.db.Close()
}

// scanner 接口抽象，统一处理 *sql.Row 和 *sql.Rows 的 Scan。
type scanner interface {
	Scan(dest ...any) error
}

// scanDocument 从数据库行扫描 Document。
func scanDocument(row scanner) (*Document, error) {
	var (
		id            int64
		absolutePath  string
		fileName      string
		extension     string
		sizeBytes     int64
		modifiedAt    sql.NullTime
		contentHash   string
		status        string
		chunkIDsJSON  string
		indexedAt     sql.NullTime
		errorMessage  string
		updatedAt     time.Time
	)
	err := row.Scan(
		&id, &absolutePath, &fileName, &extension, &sizeBytes, &modifiedAt,
		&contentHash, &status, &chunkIDsJSON, &indexedAt, &errorMessage, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描文档记录失败: %w", err)
	}
	var chunkIDs []string
	if chunkIDsJSON != "" && chunkIDsJSON != "[]" {
		if err := json.Unmarshal([]byte(chunkIDsJSON), &chunkIDs); err != nil {
			// 解析失败时返回空切片
			chunkIDs = nil
		}
	}
	doc := &Document{
		ID:           id,
		AbsolutePath: absolutePath,
		FileName:     fileName,
		Extension:    extension,
		SizeBytes:    sizeBytes,
		ContentHash:  contentHash,
		Status:       status,
		ChunkIDs:     chunkIDs,
		ErrorMessage: errorMessage,
		UpdatedAt:    updatedAt,
	}
	if modifiedAt.Valid {
		doc.ModifiedAt = modifiedAt.Time
	}
	if indexedAt.Valid {
		doc.IndexedAt = &indexedAt.Time
	}
	return doc, nil
}

// extractExt 从绝对路径提取扩展名。
func extractExt(absPath string) string {
	for i := len(absPath) - 1; i >= 0; i-- {
		if absPath[i] == '.' {
			return absPath[i:]
		}
		if absPath[i] == '/' || absPath[i] == '\\' {
			break
		}
	}
	return ""
}

// extractFileName 从绝对路径提取文件名。
func extractFileName(absPath string) string {
	for i := len(absPath) - 1; i >= 0; i-- {
		if absPath[i] == '/' || absPath[i] == '\\' {
			return absPath[i+1:]
		}
	}
	return absPath
}
