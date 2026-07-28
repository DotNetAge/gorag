package meta

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteStore 基于 SQLite 的元数据存储实现。
//
// 每次数据库操作独立打开与关闭连接，不持久持有 DB 实例。
// 设计目的：避免与 mrag serve 等长驻进程争抢 bbolt 文件锁，
// 确保 CLI 只读命令（status/diagnose）可安全访问数据目录。
type sqliteStore struct {
	dbPath string
}

// NewSQLiteStore 创建基于 SQLite 的元数据存储。
//
// 参数：
//   - dbPath: 数据库文件路径（必须非空）
//
// 构造函数仅校验路径有效性并创建表结构，不持久持有连接。
// 实际数据库连接在每操作时按需打开、操作完成后立即释放。
func NewSQLiteStore(dbPath string) (Store, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("meta.NewSQLiteStore: 数据库路径不能为空")
	}
	s := &sqliteStore{dbPath: dbPath}
	// 验证连接可用并创建表结构
	if err := s.withDB(func(db *sql.DB) error {
		return s.init(db)
	}); err != nil {
		return nil, fmt.Errorf("meta.NewSQLiteStore: 初始化数据库失败: %w", err)
	}
	return s, nil
}

// open 打开一个新的 SQLite 连接。每次操作独立调用。
func (s *sqliteStore) open() (*sql.DB, error) {
	db, err := sql.Open("sqlite", s.dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 只支持单写
	return db, nil
}

// withDB 打开连接 → 执行操作 → 关闭连接。
// 所有公开方法通过此辅助函数实现「操作时打开、完成后释放」。
func (s *sqliteStore) withDB(fn func(*sql.DB) error) error {
	db, err := s.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return fn(db)
}

// Close 实现 Store 接口。open-close 模式下无需显式关闭，保留为空操作以兼容接口。
func (s *sqliteStore) Close() error { return nil }

// init 创建 documents 和 usages 表（若不存在）。
func (s *sqliteStore) init(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS documents (
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
		)`,
		`CREATE TABLE IF NOT EXISTS usages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			model TEXT NOT NULL DEFAULT '',
			label TEXT NOT NULL DEFAULT '',
			prompt_tokens INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0,
			cached_tokens INTEGER NOT NULL DEFAULT 0,
			prompt_audio_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			completion_audio_tokens INTEGER NOT NULL DEFAULT 0,
			accepted_prediction_tokens INTEGER NOT NULL DEFAULT 0,
			rejected_prediction_tokens INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}
	// 创建索引
	for _, idx := range []string{
		"CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status)",
		"CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(content_hash)",
		"CREATE INDEX IF NOT EXISTS idx_usages_created_at ON usages(created_at DESC)",
	} {
		if _, err := db.Exec(idx); err != nil {
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
	return s.withDB(func(db *sql.DB) error {
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

		_, err := db.Exec(query,
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
	})
}

// GetDocumentByPath 实现 Store 接口。
func (s *sqliteStore) GetDocumentByPath(absPath string) (*Document, error) {
	if absPath == "" {
		return nil, fmt.Errorf("GetDocumentByPath: 路径不能为空")
	}
	var doc *Document
	err := s.withDB(func(db *sql.DB) error {
		row := db.QueryRow(`SELECT
			id, absolute_path, file_name, extension, size_bytes, modified_at,
			content_hash, status, chunk_ids, indexed_at, error_message, updated_at
			FROM documents WHERE absolute_path = ?`, absPath)
		var err error
		doc, err = scanDocument(row)
		return err
	})
	return doc, err
}

// ListDocuments 实现 Store 接口。
func (s *sqliteStore) ListDocuments(status string) ([]*Document, error) {
	var docs []*Document
	err := s.withDB(func(db *sql.DB) error {
		var rows *sql.Rows
		var err error
		if status == "" {
			rows, err = db.Query(`SELECT
				id, absolute_path, file_name, extension, size_bytes, modified_at,
				content_hash, status, chunk_ids, indexed_at, error_message, updated_at
				FROM documents ORDER BY absolute_path`)
		} else {
			rows, err = db.Query(`SELECT
				id, absolute_path, file_name, extension, size_bytes, modified_at,
				content_hash, status, chunk_ids, indexed_at, error_message, updated_at
				FROM documents WHERE status = ? ORDER BY absolute_path`, status)
		}
		if err != nil {
			return fmt.Errorf("ListDocuments: 查询失败: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			doc, err := scanDocument(rows)
			if err != nil {
				return fmt.Errorf("ListDocuments: 扫描行失败: %w", err)
			}
			docs = append(docs, doc)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("ListDocuments: 迭代失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return docs, nil
}

// CountDocumentsByStatus 实现 Store 接口。
func (s *sqliteStore) CountDocumentsByStatus() (map[string]int, error) {
	counts := make(map[string]int)
	err := s.withDB(func(db *sql.DB) error {
		rows, err := db.Query(`SELECT status, COUNT(*) FROM documents GROUP BY status`)
		if err != nil {
			return fmt.Errorf("CountDocumentsByStatus: 查询失败: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var status string
			var count int
			if err := rows.Scan(&status, &count); err != nil {
				return fmt.Errorf("CountDocumentsByStatus: 扫描失败: %w", err)
			}
			counts[status] = count
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

// ListDocumentsWithProgress 实现 Store 接口。
func (s *sqliteStore) ListDocumentsWithProgress(status, filterPath string) ([]*DocumentProgress, error) {
	var results []*DocumentProgress
	err := s.withDB(func(db *sql.DB) error {
		query := `SELECT
			d.absolute_path, d.file_name, d.extension, d.size_bytes, d.modified_at,
			d.status, d.error_message, d.indexed_at,
			0 AS total_chunks
			FROM documents d`

		var conditions []string
		var args []any

		if status != "" {
			conditions = append(conditions, "d.status = ?")
			args = append(args, status)
		}
		if filterPath != "" {
			fp := filterPath
			if !strings.HasSuffix(fp, string(filepath.Separator)) {
				fp += string(filepath.Separator)
			}
			conditions = append(conditions, "d.absolute_path LIKE ?")
			args = append(args, fp+"%")
		}

		if len(conditions) > 0 {
			query += " WHERE " + strings.Join(conditions, " AND ")
		}

		query += " ORDER BY d.absolute_path"

		rows, err := db.Query(query, args...)
		if err != nil {
			return fmt.Errorf("ListDocumentsWithProgress: 查询失败: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var (
				absPath    string
				fileName   string
				ext        string
				sizeBytes  int64
				modifiedAt sql.NullTime
				statusVal  string
				errMsg     string
				indexedAt  sql.NullTime
				total      int
			)
			if err := rows.Scan(&absPath, &fileName, &ext, &sizeBytes, &modifiedAt,
				&statusVal, &errMsg, &indexedAt,
				&total); err != nil {
				return fmt.Errorf("ListDocumentsWithProgress: 扫描失败: %w", err)
			}

			prog := &DocumentProgress{
				AbsolutePath: absPath,
				FileName:     fileName,
				Extension:    ext,
				SizeBytes:    sizeBytes,
				IndexStatus:  statusVal,
				ErrorMessage: errMsg,
				TotalChunks:  total,
			}
			if modifiedAt.Valid {
				prog.ModifiedAt = modifiedAt.Time
			}
			if indexedAt.Valid {
				prog.IndexedAt = &indexedAt.Time
			}
			results = append(results, prog)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("ListDocumentsWithProgress: 迭代失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if results == nil {
		return []*DocumentProgress{}, nil
	}
	return results, nil
}

// DeleteDocument 实现 Store 接口。
func (s *sqliteStore) DeleteDocument(absPath string) error {
	if absPath == "" {
		return fmt.Errorf("DeleteDocument: 路径不能为空")
	}
	return s.withDB(func(db *sql.DB) error {
		result, err := db.Exec("DELETE FROM documents WHERE absolute_path = ?", absPath)
		if err != nil {
			return fmt.Errorf("DeleteDocument: 删除失败: %w", err)
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return fmt.Errorf("DeleteDocument: 未找到记录: %s", absPath)
		}
		return nil
	})
}

// ═════════════════════════════════════════════════════════════════════
// Usage CRUD
// ═════════════════════════════════════════════════════════════════════

// SaveUsage 实现 Store 接口。插入一条 token 用量记录。
func (s *sqliteStore) SaveUsage(usage *Usage) error {
	if usage == nil {
		return fmt.Errorf("SaveUsage: usage 不能为空")
	}
	return s.withDB(func(db *sql.DB) error {
		_, err := db.Exec(
			`INSERT INTO usages (model, label, prompt_tokens, completion_tokens, total_tokens, cached_tokens, prompt_audio_tokens, reasoning_tokens, completion_audio_tokens, accepted_prediction_tokens, rejected_prediction_tokens, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			usage.Model,
			usage.Label,
			usage.PromptTokens,
			usage.CompletionTokens,
			usage.TotalTokens,
			usage.CachedTokens,
			usage.PromptAudioTokens,
			usage.ReasoningTokens,
			usage.CompletionAudioTokens,
			usage.AcceptedPredictionTokens,
			usage.RejectedPredictionTokens,
			usage.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("SaveUsage: 插入记录失败: %w", err)
		}
		return nil
	})
}

// QueryUsages 实现 Store 接口。按时间倒序查询最近的 token 用量记录。
func (s *sqliteStore) QueryUsages(limit int) ([]*Usage, error) {
	var usages []*Usage
	err := s.withDB(func(db *sql.DB) error {
		query := "SELECT id, model, label, prompt_tokens, completion_tokens, total_tokens, cached_tokens, prompt_audio_tokens, reasoning_tokens, completion_audio_tokens, accepted_prediction_tokens, rejected_prediction_tokens, created_at FROM usages ORDER BY created_at DESC"
		if limit > 0 {
			query += fmt.Sprintf(" LIMIT %d", limit)
		}
		rows, err := db.Query(query)
		if err != nil {
			return fmt.Errorf("QueryUsages: 查询失败: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			u, err := scanUsage(rows)
			if err != nil {
				return err
			}
			usages = append(usages, u)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("QueryUsages: 遍历结果失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if usages == nil {
		return []*Usage{}, nil
	}
	return usages, nil
}

// QueryTotalUsageStats 实现 Store 接口。聚合查询总 tokens 和最新模型名。
func (s *sqliteStore) QueryTotalUsageStats() (totalTokens int64, model string, err error) {
	err = s.withDB(func(db *sql.DB) error {
		return db.QueryRow("SELECT COALESCE(SUM(total_tokens), 0), COALESCE((SELECT model FROM usages ORDER BY id DESC LIMIT 1), '') FROM usages").Scan(&totalTokens, &model)
	})
	return
}

// scanUsage 从数据库行扫描 Usage。
func scanUsage(row scanner) (*Usage, error) {
	var u Usage
	err := row.Scan(
		&u.ID, &u.Model, &u.Label,
		&u.PromptTokens, &u.CompletionTokens, &u.TotalTokens,
		&u.CachedTokens, &u.PromptAudioTokens,
		&u.ReasoningTokens, &u.CompletionAudioTokens,
		&u.AcceptedPredictionTokens, &u.RejectedPredictionTokens,
		&u.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("扫描 usage 记录失败: %w", err)
	}
	return &u, nil
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
