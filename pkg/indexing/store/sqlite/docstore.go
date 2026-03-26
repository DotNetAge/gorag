package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	_ "modernc.org/sqlite"
)

type sqliteDocStore struct {
	db *sql.DB
}

// DefaultDocStore creates a SQLite DocStore using a default local file "gorag_docs.db".
func DefaultDocStore() (store.DocStore, error) {
	return NewDocStore("gorag_docs.db")
}

// NewDocStore creates a new SQLite based document store.
func NewDocStore(path string) (store.DocStore, error) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Optimization: WAL mode for better concurrency
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_synchronous=NORMAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite db: %w", err)
	}

	// Create tables
	schemas := []string{
		`CREATE TABLE IF NOT EXISTS documents (
			id TEXT PRIMARY KEY,
			content TEXT,
			metadata TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS chunks (
			id TEXT PRIMARY KEY,
			document_id TEXT,
			content TEXT,
			metadata TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_doc_id ON chunks(document_id)`,
	}

	for _, schema := range schemas {
		if _, err := db.Exec(schema); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to create table: %w", err)
		}
	}

	return &sqliteDocStore{db: db}, nil
}

func (s *sqliteDocStore) SetDocument(ctx context.Context, doc *core.Document) error {
	metadata, err := json.Marshal(doc.Metadata)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO documents (id, content, metadata) VALUES (?, ?, ?)",
		doc.ID, doc.Content, string(metadata))
	return err
}

func (s *sqliteDocStore) GetDocument(ctx context.Context, docID string) (*core.Document, error) {
	var doc core.Document
	var metadataStr string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, content, metadata FROM documents WHERE id = ?", docID).
		Scan(&doc.ID, &doc.Content, &metadataStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("document not found: %s", docID)
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(metadataStr), &doc.Metadata); err != nil {
		return nil, err
	}

	return &doc, nil
}

func (s *sqliteDocStore) DeleteDocument(ctx context.Context, docID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM documents WHERE id = ?", docID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM chunks WHERE document_id = ?", docID); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *sqliteDocStore) SetChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, "INSERT OR REPLACE INTO chunks (id, document_id, content, metadata) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		metadata, err := json.Marshal(chunk.Metadata)
		if err != nil {
			return err
		}
		if _, err := stmt.ExecContext(ctx, chunk.ID, chunk.DocumentID, chunk.Content, string(metadata)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *sqliteDocStore) GetChunk(ctx context.Context, chunkID string) (*core.Chunk, error) {
	var chunk core.Chunk
	var metadataStr string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, document_id, content, metadata FROM chunks WHERE id = ?", chunkID).
		Scan(&chunk.ID, &chunk.DocumentID, &chunk.Content, &metadataStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("chunk not found: %s", chunkID)
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(metadataStr), &chunk.Metadata); err != nil {
		return nil, err
	}

	return &chunk, nil
}

func (s *sqliteDocStore) GetChunksByDocID(ctx context.Context, docID string) ([]*core.Chunk, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, document_id, content, metadata FROM chunks WHERE document_id = ?", docID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []*core.Chunk
	for rows.Next() {
		var chunk core.Chunk
		var metadataStr string
		if err := rows.Scan(&chunk.ID, &chunk.DocumentID, &chunk.Content, &metadataStr); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(metadataStr), &chunk.Metadata); err != nil {
			return nil, err
		}
		chunks = append(chunks, &chunk)
	}
	return chunks, nil
}

func (s *sqliteDocStore) Close() error {
	return s.db.Close()
}
