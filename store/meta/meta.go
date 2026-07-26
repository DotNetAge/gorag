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

// init 创建 documents、chunk_llm_status 和 usages 表（若不存在）。
func (s *sqliteStore) init() error {
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
		`CREATE TABLE IF NOT EXISTS chunk_llm_status (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			chunk_id TEXT UNIQUE NOT NULL,
			doc_path TEXT NOT NULL DEFAULT '',
			doc_id TEXT NOT NULL DEFAULT '',
			content_hash TEXT NOT NULL DEFAULT '',
			content_length INTEGER NOT NULL DEFAULT 0,
			summarized INTEGER NOT NULL DEFAULT 0,
			last_summarized_at TIMESTAMP,
			refilled INTEGER NOT NULL DEFAULT 0,
			last_refilled_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
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
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("创建表失败: %w", err)
		}
	}
	// 创建索引
	for _, idx := range []string{
		"CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status)",
		"CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(content_hash)",
		"CREATE INDEX IF NOT EXISTS idx_cls_doc_path ON chunk_llm_status(doc_path)",
		"CREATE INDEX IF NOT EXISTS idx_cls_chunk_id ON chunk_llm_status(chunk_id)",
		"CREATE INDEX IF NOT EXISTS idx_cls_summarized ON chunk_llm_status(summarized)",
		"CREATE INDEX IF NOT EXISTS idx_cls_refilled ON chunk_llm_status(refilled)",
		"CREATE INDEX IF NOT EXISTS idx_usages_created_at ON usages(created_at DESC)",
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

// CountDocumentsByStatus 实现 Store 接口。
func (s *sqliteStore) CountDocumentsByStatus() (map[string]int, error) {
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM documents GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("CountDocumentsByStatus: 查询失败: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("CountDocumentsByStatus: 扫描失败: %w", err)
		}
		counts[status] = count
	}
	return counts, rows.Err()
}

// CountLLMStatus 实现 Store 接口。
func (s *sqliteStore) CountLLMStatus() (totalChunks, summarized, refilled int, err error) {
	row := s.db.QueryRow(`SELECT
		COUNT(*),
		COALESCE(SUM(CASE WHEN summarized = 1 THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN refilled = 1 THEN 1 ELSE 0 END), 0)
		FROM chunk_llm_status`)
	err = row.Scan(&totalChunks, &summarized, &refilled)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("CountLLMStatus: 查询失败: %w", err)
	}
	return
}

// ListDocumentsWithProgress 实现 Store 接口。
func (s *sqliteStore) ListDocumentsWithProgress(status, filterPath string) ([]*DocumentProgress, error) {
	query := `SELECT
		d.absolute_path, d.file_name, d.extension, d.size_bytes, d.modified_at,
		d.status, d.error_message, d.indexed_at,
		COUNT(cls.chunk_id) AS total_chunks,
		COALESCE(SUM(cls.summarized), 0) AS summarized_count,
		COALESCE(SUM(cls.refilled), 0) AS refilled_count
		FROM documents d
		LEFT JOIN chunk_llm_status cls ON d.absolute_path = cls.doc_path`

	var conditions []string
	var args []any

	if status != "" {
		conditions = append(conditions, "d.status = ?")
		args = append(args, status)
	}
	if filterPath != "" {
		// 确保 filterPath 以路径分隔符结尾，避免部分匹配
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

	query += " GROUP BY d.absolute_path ORDER BY d.absolute_path"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListDocumentsWithProgress: 查询失败: %w", err)
	}
	defer rows.Close()

	var results []*DocumentProgress
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
			summarized int
			refilled   int
		)
		if err := rows.Scan(&absPath, &fileName, &ext, &sizeBytes, &modifiedAt,
			&statusVal, &errMsg, &indexedAt,
			&total, &summarized, &refilled); err != nil {
			return nil, fmt.Errorf("ListDocumentsWithProgress: 扫描失败: %w", err)
		}

		prog := &DocumentProgress{
			AbsolutePath:    absPath,
			FileName:        fileName,
			Extension:       ext,
			SizeBytes:       sizeBytes,
			IndexStatus:     statusVal,
			ErrorMessage:    errMsg,
			TotalChunks:     total,
			SummarizedCount: summarized,
			RefilledCount:   refilled,
			LLMStatus:       DeriveLLMStatus(total, summarized, refilled),
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
		return nil, fmt.Errorf("ListDocumentsWithProgress: 迭代失败: %w", err)
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

// ═════════════════════════════════════════════════════════════════════
// Usage CRUD
// ═════════════════════════════════════════════════════════════════════

// SaveUsage 实现 Store 接口。插入一条 token 用量记录。
func (s *sqliteStore) SaveUsage(usage *Usage) error {
	if usage == nil {
		return fmt.Errorf("SaveUsage: usage 不能为空")
	}
	_, err := s.db.Exec(
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
}

// QueryUsages 实现 Store 接口。按时间倒序查询最近的 token 用量记录。
func (s *sqliteStore) QueryUsages(limit int) ([]*Usage, error) {
	query := "SELECT id, model, label, prompt_tokens, completion_tokens, total_tokens, cached_tokens, prompt_audio_tokens, reasoning_tokens, completion_audio_tokens, accepted_prediction_tokens, rejected_prediction_tokens, created_at FROM usages ORDER BY created_at DESC"
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("QueryUsages: 查询失败: %w", err)
	}
	defer rows.Close()

	var usages []*Usage
	for rows.Next() {
		u, err := scanUsage(rows)
		if err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("QueryUsages: 遍历结果失败: %w", err)
	}
	if usages == nil {
		return []*Usage{}, nil
	}
	return usages, nil
}

// QueryTotalUsageStats 实现 Store 接口。聚合查询总 tokens 和最新模型名。
func (s *sqliteStore) QueryTotalUsageStats() (totalTokens int64, model string, err error) {
	err = s.db.QueryRow("SELECT COALESCE(SUM(total_tokens), 0), COALESCE((SELECT model FROM usages ORDER BY id DESC LIMIT 1), '') FROM usages").Scan(&totalTokens, &model)
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

// ═════════════════════════════════════════════════════════════════════
// ChunkLLMStatus CRUD
// ═════════════════════════════════════════════════════════════════════

// SaveChunkLLMStatus 实现 Store 接口。按 chunk_id UPSERT。
func (s *sqliteStore) SaveChunkLLMStatus(status *ChunkLLMStatus) error {
	if status == nil {
		return fmt.Errorf("SaveChunkLLMStatus: status 不能为空")
	}
	if status.ChunkID == "" {
		return fmt.Errorf("SaveChunkLLMStatus: ChunkID 不能为空")
	}

	query := `INSERT INTO chunk_llm_status
		(chunk_id, doc_path, doc_id, content_hash, content_length,
		 summarized, last_summarized_at, refilled, last_refilled_at,
		 updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(chunk_id) DO UPDATE SET
		doc_path=excluded.doc_path,
		doc_id=excluded.doc_id,
		content_hash=excluded.content_hash,
		content_length=excluded.content_length,
		summarized=excluded.summarized,
		last_summarized_at=excluded.last_summarized_at,
		refilled=excluded.refilled,
		last_refilled_at=excluded.last_refilled_at,
		updated_at=CURRENT_TIMESTAMP`

	_, err := s.db.Exec(query,
		status.ChunkID,
		status.DocPath,
		status.DocID,
		status.ContentHash,
		status.ContentLength,
		boolToInt(status.Summarized),
		status.LastSummarizedAt,
		boolToInt(status.Refilled),
		status.LastRefilledAt,
	)
	if err != nil {
		return fmt.Errorf("SaveChunkLLMStatus: 写入失败: %w", err)
	}
	return nil
}

// GetChunkLLMStatus 实现 Store 接口。
func (s *sqliteStore) GetChunkLLMStatus(chunkID string) (*ChunkLLMStatus, error) {
	if chunkID == "" {
		return nil, fmt.Errorf("GetChunkLLMStatus: chunkID 不能为空")
	}
	row := s.db.QueryRow(`SELECT
		id, chunk_id, doc_path, doc_id, content_hash, content_length,
		summarized, last_summarized_at, refilled, last_refilled_at,
		created_at, updated_at
		FROM chunk_llm_status WHERE chunk_id = ?`, chunkID)

	return scanChunkLLMStatus(row)
}

// GetChunkLLMStatusesByDocPath 实现 Store 接口。
func (s *sqliteStore) GetChunkLLMStatusesByDocPath(docPath string) ([]*ChunkLLMStatus, error) {
	if docPath == "" {
		return nil, fmt.Errorf("GetChunkLLMStatusesByDocPath: docPath 不能为空")
	}
	rows, err := s.db.Query(`SELECT
		id, chunk_id, doc_path, doc_id, content_hash, content_length,
		summarized, last_summarized_at, refilled, last_refilled_at,
		created_at, updated_at
		FROM chunk_llm_status WHERE doc_path = ?`, docPath)
	if err != nil {
		return nil, fmt.Errorf("GetChunkLLMStatusesByDocPath: 查询失败: %w", err)
	}
	defer rows.Close()

	var results []*ChunkLLMStatus
	for rows.Next() {
		s, err := scanChunkLLMStatus(rows)
		if err != nil {
			return nil, fmt.Errorf("GetChunkLLMStatusesByDocPath: 扫描失败: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// DeleteChunkLLMStatusByDocPath 实现 Store 接口。
func (s *sqliteStore) DeleteChunkLLMStatusByDocPath(docPath string) error {
	if docPath == "" {
		return fmt.Errorf("DeleteChunkLLMStatusByDocPath: docPath 不能为空")
	}
	_, err := s.db.Exec("DELETE FROM chunk_llm_status WHERE doc_path = ?", docPath)
	if err != nil {
		return fmt.Errorf("DeleteChunkLLMStatusByDocPath: 删除失败: %w", err)
	}
	return nil
}

// DeleteChunkLLMStatusByChunkID 实现 Store 接口。
func (s *sqliteStore) DeleteChunkLLMStatusByChunkID(chunkID string) error {
	if chunkID == "" {
		return fmt.Errorf("DeleteChunkLLMStatusByChunkID: chunkID 不能为空")
	}
	_, err := s.db.Exec("DELETE FROM chunk_llm_status WHERE chunk_id = ?", chunkID)
	if err != nil {
		return fmt.Errorf("DeleteChunkLLMStatusByChunkID: 删除失败: %w", err)
	}
	return nil
}

// GetChunksNeedingLLM 实现 Store 接口。
func (s *sqliteStore) GetChunksNeedingLLM(docPath string, summarized, refilled bool, limit int) ([]*ChunkLLMStatus, error) {
	// 构建 WHERE 条件：summarized=false OR refilled=false
	needSummarized := !summarized
	needRefilled := !refilled

	var conditions []string
	var args []any

	if docPath != "" {
		conditions = append(conditions, "doc_path = ?")
		args = append(args, docPath)
	}

	var orParts []string
	if needSummarized {
		orParts = append(orParts, "summarized = 0")
	}
	if needRefilled {
		orParts = append(orParts, "refilled = 0")
	}
	if len(orParts) > 0 {
		conditions = append(conditions, "("+joinOr(orParts)+")")
	}

	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + joinAnd(conditions)
	}

	query := `SELECT
		id, chunk_id, doc_path, doc_id, content_hash, content_length,
		summarized, last_summarized_at, refilled, last_refilled_at,
		created_at, updated_at
		FROM chunk_llm_status` + where + ` ORDER BY doc_path, chunk_id`

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("GetChunksNeedingLLM: 查询失败: %w", err)
	}
	defer rows.Close()

	var results []*ChunkLLMStatus
	for rows.Next() {
		st, err := scanChunkLLMStatus(rows)
		if err != nil {
			return nil, fmt.Errorf("GetChunksNeedingLLM: 扫描失败: %w", err)
		}
		results = append(results, st)
	}
	return results, rows.Err()
}

// ResetAllLLMStatus 实现 Store 接口。
// 将所有 chunk 的 summarized 和 refilled 重置为 0，同时清空时间戳。
func (s *sqliteStore) ResetAllLLMStatus() error {
	_, err := s.db.Exec(`UPDATE chunk_llm_status SET
		summarized = 0,
		refilled = 0,
		last_summarized_at = NULL,
		last_refilled_at = NULL,
		updated_at = CURRENT_TIMESTAMP
		WHERE summarized = 1 OR refilled = 1`)
	if err != nil {
		return fmt.Errorf("ResetAllLLMStatus: 重置失败: %w", err)
	}
	return nil
}

// ── ChunkLLMStatus 辅助函数 ───────────────────────────────────────

// scanChunkLLMStatus 从数据库行扫描 ChunkLLMStatus。
func scanChunkLLMStatus(row scanner) (*ChunkLLMStatus, error) {
	var (
		id             int64
		chunkID        string
		docPath        string
		docID          string
		contentHash    string
		contentLength  int
		summarized     int
		lastSumAt      sql.NullTime
		refilled       int
		lastRefAt      sql.NullTime
		createdAt      time.Time
		updatedAt      time.Time
	)
	err := row.Scan(
		&id, &chunkID, &docPath, &docID, &contentHash, &contentLength,
		&summarized, &lastSumAt, &refilled, &lastRefAt,
		&createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描 chunk_llm_status 失败: %w", err)
	}
	st := &ChunkLLMStatus{
		ID:            id,
		ChunkID:       chunkID,
		DocPath:       docPath,
		DocID:         docID,
		ContentHash:   contentHash,
		ContentLength: contentLength,
		Summarized:    intToBool(summarized),
		Refilled:      intToBool(refilled),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
	if lastSumAt.Valid {
		st.LastSummarizedAt = &lastSumAt.Time
	}
	if lastRefAt.Valid {
		st.LastRefilledAt = &lastRefAt.Time
	}
	return st, nil
}

// boolToInt 将 bool 转换为 sqlite 的 int（0/1）。
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// intToBool 将 sqlite 的 int（0/1）转换为 bool。
func intToBool(i int) bool {
	return i != 0
}

// joinAnd 用 AND 连接条件字符串。
func joinAnd(parts []string) string {
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " AND " + parts[i]
	}
	return result
}

// joinOr 用 OR 连接条件字符串。
func joinOr(parts []string) string {
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += " OR " + parts[i]
	}
	return result
}
