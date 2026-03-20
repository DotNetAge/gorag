package bolt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/store"
	bolt "go.etcd.io/bbolt"
)

var (
	docBucket   = []byte("documents")
	chunkBucket = []byte("chunks")
)

type boltDocStore struct {
	db *bolt.DB
}

// NewDocStore creates a new BoltDB based document store.
func NewDocStore(path string) (store.DocStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory: %w", err)
		}
	}

	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt db: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(docBucket)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(chunkBucket)
		return err
	})

	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return &boltDocStore{db: db}, nil
}

func (s *boltDocStore) SetDocument(ctx context.Context, doc *core.Document) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(docBucket)
		return b.Put([]byte(doc.ID), data)
	})
}

func (s *boltDocStore) GetDocument(ctx context.Context, docID string) (*core.Document, error) {
	var doc core.Document
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(docBucket)
		v := b.Get([]byte(docID))
		if v == nil {
			return fmt.Errorf("document not found: %s", docID)
		}
		return json.Unmarshal(v, &doc)
	})
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

func (s *boltDocStore) DeleteDocument(ctx context.Context, docID string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(docBucket)
		return b.Delete([]byte(docID))
		// Note: We could also delete associated chunks here if needed
	})
}

func (s *boltDocStore) SetChunks(ctx context.Context, chunks []*core.Chunk) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(chunkBucket)
		for _, chunk := range chunks {
			data, err := json.Marshal(chunk)
			if err != nil {
				return err
			}
			if err := b.Put([]byte(chunk.ID), data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *boltDocStore) GetChunk(ctx context.Context, chunkID string) (*core.Chunk, error) {
	var chunk core.Chunk
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(chunkBucket)
		v := b.Get([]byte(chunkID))
		if v == nil {
			return fmt.Errorf("chunk not found: %s", chunkID)
		}
		return json.Unmarshal(v, &chunk)
	})
	if err != nil {
		return nil, err
	}
	return &chunk, nil
}

func (s *boltDocStore) GetChunksByDocID(ctx context.Context, docID string) ([]*core.Chunk, error) {
	var chunks []*core.Chunk
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(chunkBucket)
		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			var chunk core.Chunk
			if err := json.Unmarshal(v, &chunk); err != nil {
				continue
			}
			if chunk.DocumentID == docID {
				chunks = append(chunks, &chunk)
			}
		}
		return nil
	})
	return chunks, err
}

func (s *boltDocStore) Close() error {
	return s.db.Close()
}
