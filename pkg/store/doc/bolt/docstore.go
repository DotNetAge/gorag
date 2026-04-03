package bolt

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DotNetAge/gorag/pkg/core"
	bolt "go.etcd.io/bbolt"
)

var (
	docBucket      = []byte("documents")
	chunkBucket    = []byte("chunks")
	docChunkBucket = []byte("doc_chunks") // New Secondary Index: docID -> []chunkID
)

type boltDocStore struct {
	db *bolt.DB
}

// DefaultDocStore creates a Bolt DocStore using a default local file "gorag_docs.bolt".
func DefaultDocStore() (core.DocStore, error) {
	return NewDocStore("gorag_docs.bolt")
}

// NewDocStore creates a new BoltDB based document core.
func NewDocStore(path string) (core.DocStore, error) {
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

	err = db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{docBucket, chunkBucket, docChunkBucket}
		for _, b := range buckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
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
		// 1. Delete Document
		b := tx.Bucket(docBucket)
		if err := b.Delete([]byte(docID)); err != nil {
			return err
		}

		// 2. Retrieve chunk IDs associated with this document
		dcb := tx.Bucket(docChunkBucket)
		chunkIDsData := dcb.Get([]byte(docID))
		if chunkIDsData != nil {
			var chunkIDs []string
			if err := json.Unmarshal(chunkIDsData, &chunkIDs); err == nil {
				// 3. Delete individual chunks
				cb := tx.Bucket(chunkBucket)
				for _, cid := range chunkIDs {
					cb.Delete([]byte(cid))
				}
			}
			// 4. Delete the index entry
			dcb.Delete([]byte(docID))
		}
		return nil
	})
}

func (s *boltDocStore) SetChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		cb := tx.Bucket(chunkBucket)
		dcb := tx.Bucket(docChunkBucket)

		// Group chunks by Document ID to efficiently update the secondary index
		docToChunksMap := make(map[string][]string)

		for _, chunk := range chunks {
			data, err := json.Marshal(chunk)
			if err != nil {
				return err
			}
			if err := cb.Put([]byte(chunk.ID), data); err != nil {
				return err
			}

			docToChunksMap[chunk.DocumentID] = append(docToChunksMap[chunk.DocumentID], chunk.ID)
		}

		// Update Secondary Index (doc_chunks)
		for docID, newChunkIDs := range docToChunksMap {
			var existingIDs []string
			existingData := dcb.Get([]byte(docID))
			if existingData != nil {
				_ = json.Unmarshal(existingData, &existingIDs)
			}

			// Append new IDs (assuming no duplicates for simplicity,
			// in a real scenario we might want to check for duplicates or use a set)
			existingIDs = append(existingIDs, newChunkIDs...)

			indexData, err := json.Marshal(existingIDs)
			if err != nil {
				return err
			}
			if err := dcb.Put([]byte(docID), indexData); err != nil {
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
		dcb := tx.Bucket(docChunkBucket)
		chunkIDsData := dcb.Get([]byte(docID))
		if chunkIDsData == nil {
			return nil // No chunks found for this document, not necessarily an error
		}

		var chunkIDs []string
		if err := json.Unmarshal(chunkIDsData, &chunkIDs); err != nil {
			return err
		}

		cb := tx.Bucket(chunkBucket)
		for _, cid := range chunkIDs {
			chunkData := cb.Get([]byte(cid))
			if chunkData != nil {
				var chunk core.Chunk
				if err := json.Unmarshal(chunkData, &chunk); err == nil {
					chunks = append(chunks, &chunk)
				}
			}
		}
		return nil
	})
	return chunks, err
}

func (s *boltDocStore) Close() error {
	return s.db.Close()
}
