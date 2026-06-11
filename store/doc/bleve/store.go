package bleve

import (
	"os"
	"sync"

	"github.com/DotNetAge/gorag/core"
	blevedb "github.com/blevesearch/bleve"
)

// BleveStore 基于 bleve 的全文搜索引擎
type BleveStore struct {
	dbPath string
	index  blevedb.Index
	mu     sync.RWMutex
}

// var _ core.FullTextStore = &BleveStore{}

// NewBleveStore 创建或打开 bleve 索引
func NewBleveStore(dbPath string) (core.FullTextStore, error) {
	store := &BleveStore{dbPath: dbPath}

	// 如果索引已存在，直接打开
	if _, err := os.Stat(dbPath); err == nil {
		index, err := blevedb.Open(dbPath)
		if err != nil {
			return nil, err
		}
		store.index = index
		return store, nil
	}

	// 否则创建新索引（使用默认映射，bleve 会自动处理 Chunk 结构）
	index, err := blevedb.New(dbPath, blevedb.NewIndexMapping())
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
// type SearchResult struct {
// 	ID      string
// 	Score   float64
// 	DocID   string
// 	Content string
// }

// Search 执行全文搜索，返回匹配的 chunk 信息
func (s *BleveStore) Search(query string, topK int) ([]core.FullTextSearchResult, error) {
	if query == "" {
		return nil, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	queryObj := blevedb.NewQueryStringQuery(query)
	searchRequest := blevedb.NewSearchRequest(queryObj)
	searchRequest.Size = topK
	searchRequest.Fields = []string{"doc_id", "content", "metadata", "chunk_meta"}

	result, err := s.index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	results := make([]core.FullTextSearchResult, 0, len(result.Hits))
	for _, hit := range result.Hits {
		sr := core.FullTextSearchResult{
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
		// 提取 metadata
		if metadata, ok := hit.Fields["metadata"].(map[string]any); ok {
			sr.Metadata = metadata
		}
		// 提取 chunk_meta
		if chunkMetaMap, ok := hit.Fields["chunk_meta"].(map[string]any); ok {
			sr.ChunkMeta = extractChunkMeta(chunkMetaMap)
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

// Count 返回全文索引中的文档总数
func (s *BleveStore) Count() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count, err := s.index.DocCount()
	if err != nil {
		return 0, err
	}
	return int(count), nil
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

// extractChunkMeta 从 map 中提取 ChunkMeta
func extractChunkMeta(m map[string]any) core.ChunkMeta {
	cm := core.ChunkMeta{}
	if index, ok := m["index"].(float64); ok {
		cm.Index = int(index)
	}
	if startPos, ok := m["start_pos"].(float64); ok {
		cm.StartPos = int(startPos)
	}
	if endPos, ok := m["end_pos"].(float64); ok {
		cm.EndPos = int(endPos)
	}
	if headingLevel, ok := m["heading_level"].(float64); ok {
		cm.HeadingLevel = int(headingLevel)
	}
	if headingPath, ok := m["heading_path"].([]any); ok {
		for _, h := range headingPath {
			if hs, ok := h.(string); ok {
				cm.HeadingPath = append(cm.HeadingPath, hs)
			}
		}
	}
	return cm
}
