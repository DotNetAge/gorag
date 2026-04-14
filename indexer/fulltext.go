package indexer

import (
	"context"
	"fmt"
	"sync"

	"github.com/DotNetAge/gorag/core"
	bledoc "github.com/DotNetAge/gorag/store/doc/bleve"
)

// fulltextIndexer 基于 bleve 的全文索引器
type fulltextIndexer struct {
	store *bledoc.BleveStore
}

// NewFulltextIndexer 创建全文索引器
func NewFulltextIndexer(dbPath string) (core.Indexer, error) {
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
	chunks, err := GetChunks(content)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	for _, chunk := range chunks {
		if err := f.store.Index(chunk); err != nil {
			return nil, err
		}
	}
	return chunks[0], nil
}

func (f *fulltextIndexer) AddFile(ctx context.Context, filePath string) (*core.Chunk, error) {
	chunks, err := GetFileChunks(filePath)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	for _, chunk := range chunks {
		if err := f.store.Index(chunk); err != nil {
			return nil, err
		}
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

// 私有：确保实现 core.Indexer 接口
var _ core.Indexer = (*fulltextIndexer)(nil)

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

// NewSafeFulltextIndexer 创建线程安全的全文索引器
func NewSafeFulltextIndexer(dbPath string) (core.Indexer, error) {
	inner, err := NewFulltextIndexer(dbPath)
	if err != nil {
		return nil, err
	}
	return &safeFulltextIndexer{inner: inner.(*fulltextIndexer)}, nil
}

func (s *safeFulltextIndexer) NewQuery(terms string) core.Query {
	return FulltextQuery(terms)
}

func (s *fulltextIndexer) NewQuery(terms string) core.Query {
	return FulltextQuery(terms)
}
