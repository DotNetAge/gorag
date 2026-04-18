package indexer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/query"
	bledoc "github.com/DotNetAge/gorag/store/doc/bleve"
)

// fulltextIndexer 基于 bleve 的全文索引器
type fulltextIndexer struct {
	store core.FullTextStore
}

func NewFulltextIndexer(store core.FullTextStore) (core.Indexer, error) {
	return &fulltextIndexer{store: store}, nil
}

// NewFulltextIndexer 创建全文索引器
func NewFulltextIndexerWithFile(dbPath string) (core.Indexer, error) {
	store, err := bledoc.NewBleveStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create bleve store: %w", err)
	}
	return &fulltextIndexer{store: store}, nil
}

func (f *fulltextIndexer) Name() string {
	return "fulltext"
}

func (f *fulltextIndexer) Type() string {
	return "fulltext"
}

func (f *fulltextIndexer) Add(ctx context.Context, content string) (*core.Chunk, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}
	chunks, err := GetChunks(content)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from content")
	}
	if err := f.IndexChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return chunks[0], nil
}

func (f *fulltextIndexer) AddFile(ctx context.Context, filePath string) (*core.Chunk, error) {
	// 安全检查：防止路径遍历攻击
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("invalid file path: %w", err)
	}
	// 检查是否包含危险路径组件
	if strings.Contains(absPath, "..") {
		return nil, fmt.Errorf("file path contains invalid components")
	}
	chunks, err := GetFileChunks(absPath)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	if err := f.IndexChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return chunks[0], nil
}

func (f *fulltextIndexer) Search(ctx context.Context, query core.Query) ([]core.Hit, error) {
	if query.Raw() == "" {
		return nil, nil
	}

	// 获取 topK
	topK := 10

	results, err := f.store.Search(query.Raw(), topK)
	if err != nil {
		return nil, err
	}

	hits := make([]core.Hit, 0, len(results))
	for _, r := range results {
		hits = append(hits, core.Hit{
			ID:      r.ID,
			Score:   float32(r.Score),
			DocID:   r.DocID,
			Content: r.Content,
		})
	}
	return hits, nil
}

func (f *fulltextIndexer) Remove(ctx context.Context, chunkID string) error {
	return f.store.Delete(chunkID)
}

// IndexChunk indexes a pre-generated chunk (implements core.Indexer interface)
func (f *fulltextIndexer) IndexChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk cannot be nil")
	}
	return f.store.Index(chunk)
}

// IndexChunks indexes multiple pre-generated chunks in batch (implements core.ChunkIndexer interface)
func (f *fulltextIndexer) IndexChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	for _, chunk := range chunks {
		if err := f.store.Index(chunk); err != nil {
			return err
		}
	}
	return nil
}

// 私有：确保实现 core.Indexer 接口
var _ core.Indexer = (*fulltextIndexer)(nil)

// Ensure implementation of core.ChunkIndexer interface
var _ core.ChunkIndexer = (*fulltextIndexer)(nil)

// safeFulltextIndexer 线程安全的包装器
type safeFulltextIndexer struct {
	mu    sync.RWMutex
	inner *fulltextIndexer
}

func (f *safeFulltextIndexer) Name() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.inner.Name()
}

func (f *safeFulltextIndexer) Type() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.inner.Type()
}

func (f *safeFulltextIndexer) Add(ctx context.Context, content string) (*core.Chunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inner.Add(ctx, content)
}

func (f *safeFulltextIndexer) AddFile(ctx context.Context, filePath string) (*core.Chunk, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inner.AddFile(ctx, filePath)
}

func (f *safeFulltextIndexer) Search(ctx context.Context, query core.Query) ([]core.Hit, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.inner.Search(ctx, query)
}

func (f *safeFulltextIndexer) Remove(ctx context.Context, chunkID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inner.Remove(ctx, chunkID)
}

func (f *safeFulltextIndexer) IndexChunk(ctx context.Context, chunk *core.Chunk) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inner.IndexChunk(ctx, chunk)
}

func (f *safeFulltextIndexer) IndexChunks(ctx context.Context, chunks []*core.Chunk) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.inner.IndexChunks(ctx, chunks)
}

// NewSafeFulltextIndexer 创建线程安全的全文索引器
func NewSafeFulltextIndexer(dbPath string) (core.Indexer, error) {
	inner, err := NewFulltextIndexerWithFile(dbPath)
	if err != nil {
		return nil, err
	}
	return &safeFulltextIndexer{inner: inner.(*fulltextIndexer)}, nil
}

func (s *safeFulltextIndexer) NewQuery(terms string) core.Query {
	return query.NewFulltextQuery(terms)
}

func (s *fulltextIndexer) NewQuery(terms string) core.Query {
	return query.NewFulltextQuery(terms)
}
