package doc

import (
	"os"
	"sync"

	"github.com/DotNetAge/gorag/core"
	"github.com/blevesearch/bleve"
)

// BleveStore 基于 bleve 的全文搜索引擎
type BleveStore struct {
	dbPath string
	index  bleve.Index
	mu     sync.RWMutex
}

// NewBleveStore 创建或打开 bleve 索引
func NewBleveStore(dbPath string) (*BleveStore, error) {
	store := &BleveStore{dbPath: dbPath}

	// 如果索引已存在，直接打开
	if _, err := os.Stat(dbPath); err == nil {
		index, err := bleve.Open(dbPath)
		if err != nil {
			return nil, err
		}
		store.index = index
		return store, nil
	}

	// 否则创建新索引（使用默认映射，bleve 会自动处理 Chunk 结构）
	index, err := bleve.New(dbPath, bleve.NewIndexMapping())
	if err != nil {
		return nil, err
	}
	store.index = index
	return store, nil
}

// Index 将 chunk 索引到 bleve
func (s *BleveStore) Index(chunk *core.Chunk) error {
	if chunk == nil || chunk.ID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index.Index(chunk.ID, chunk)
}

// SearchResult 全文搜索结果
type SearchResult struct {
	ID       string
	Score    float64
	DocID    string
	Content  string
}

// Search 执行全文搜索，返回匹配的 chunk 信息
func (s *BleveStore) Search(query string, topK int) ([]SearchResult, error) {
	if query == "" {
		return nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	queryObj := bleve.NewQueryStringQuery(query)
	searchRequest := bleve.NewSearchRequest(queryObj)
	searchRequest.Size = topK
	searchRequest.Fields = []string{"doc_id", "content"}

	result, err := s.index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sr := SearchResult{
			ID:    hit.ID,
			Score: hit.Score,
		}
		// 从 Fields 中提取 doc_id 和 content
		if docID, ok := hit.Fields["doc_id"].(string); ok {
			sr.DocID = docID
		}
		if content, ok := hit.Fields["content"].(string); ok {
			sr.Content = content
		}
		results = append(results, sr)
	}
	return results, nil
}

// Delete 从索引中删除 chunk
func (s *BleveStore) Delete(chunkID string) error {
	if chunkID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index.Delete(chunkID)
}

// Close 关闭索引
func (s *BleveStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index != nil {
		return s.index.Close()
	}
	return nil
}
