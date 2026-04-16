package indexer

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/query"
)

// 使用向量数据库和向量模型进行索引及检索
type semanticIndexer struct {
	name     string
	db       core.VectorStore
	embedder core.Embedder
}

func NewSemanticIndexer(db core.VectorStore, embedder core.Embedder) core.Indexer {
	return &semanticIndexer{
		name:     "semantic",
		db:       db,
		embedder: embedder,
	}
}

func (s *semanticIndexer) Name() string {
	return s.name
}

func (s *semanticIndexer) Type() string {
	return "semantic"
}

func (s *semanticIndexer) Add(ctx context.Context, content string) (*core.Chunk, error) {
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
	for _, chunk := range chunks {
		if err := s.indexAndStore(ctx, chunk); err != nil {
			return nil, err
		}
	}
	return chunks[0], nil
}

func (s *semanticIndexer) AddFile(ctx context.Context, filePath string) (*core.Chunk, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}
	chunks, err := GetFileChunks(filePath)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("no chunks generated from file")
	}
	for _, chunk := range chunks {
		if err := s.indexAndStore(ctx, chunk); err != nil {
			return nil, err
		}
	}
	return chunks[0], nil
}

// indexAndStore 计算 chunk 向量并存储到数据库
func (s *semanticIndexer) indexAndStore(ctx context.Context, chunk *core.Chunk) error {
	vector, err := s.embedder.Calc(chunk)
	if err != nil {
		return err
	}
	return s.db.Upsert(ctx, []*core.Vector{vector})
}

func (s *semanticIndexer) Search(ctx context.Context, q core.Query) ([]core.Hit, error) {
	// 1. 从查询获取向量 - 优先使用 Query 中的预计算向量，否则实时计算
	var queryVector []float32
	query, ok := q.(*query.SemanticQuery)
	if !ok {
		return nil, fmt.Errorf("invalid query type: expected *semanticQuery, got %T", q)
	}

	if query.Vector() != nil {
		queryVector = query.Vector().Values
	} else {
		// 实时计算查询向量
		vec, err := s.embedder.CalcText(query.Raw())
		if err != nil {
			return nil, err
		}
		queryVector = vec.Values
	}

	// 2. 获取过滤器
	filters := query.Filters()

	// 3. 向量相似度搜索
	// TODO: topK 应该从 query 中获取，当前使用默认值
	topK := 10
	results, scores, err := s.db.Search(ctx, queryVector, topK, filters)
	if err != nil {
		return nil, err
	}

	// 4. ParentDoc 处理：如果结果是子块，替换为父块
	results = s.resolveParentChunks(results)

	// 5. 构建 Hit 返回
	// 注意：返回 vec.ChunkID（chunk ID）而不是 vec.ID（UUID）
	// 这是为了与 fulltextIndexer.Search 返回的 ID 格式一致
	// 确保混合搜索 RRF 融合时能正确匹配
	hits := make([]core.Hit, 0, len(results))
	for i, vec := range results {
		docID := ""
		if vec.Metadata != nil {
			if d, ok := vec.Metadata["doc_id"].(string); ok {
				docID = d
			}
		}
		hits = append(hits, core.Hit{
			ID:      vec.ChunkID, // 使用 ChunkID 而不是 UUID
			Score:   scores[i],
			Content: s.extractChunkContent(vec),
			DocID:   docID,
		})
	}

	return hits, nil
}

// resolveParentChunks 处理 ParentDoc 分块结果
// 如果匹配到子块，用父块替换；父块直接返回
func (s *semanticIndexer) resolveParentChunks(vectors []*core.Vector) []*core.Vector {
	if len(vectors) == 0 {
		return vectors
	}

	type replacement struct {
		childIdx  int
		parentIdx int
	}
	var replacements []replacement

	// 识别子块和父块
	for i, vec := range vectors {
		if vec == nil || vec.Metadata == nil {
			continue
		}
		if isParent, _ := vec.Metadata["is_parent"].(bool); !isParent {
			if parentID, ok := vec.Metadata["parent_id"].(string); ok && parentID != "" {
				for j, pv := range vectors {
					// 比较 pv.ChunkID（chunk ID）而不是 pv.ID（UUID）
					if pv != nil && pv.ChunkID == parentID {
						replacements = append(replacements, replacement{childIdx: i, parentIdx: j})
						break
					}
				}
			}
		}
	}

	// 执行替换并去重
	if len(replacements) > 0 {
		for _, r := range replacements {
			vectors[r.childIdx] = vectors[r.parentIdx]
		}
		vectors = deduplicateVectors(vectors)
	}

	return vectors
}

// deduplicateVectors 去除重复的向量（按 ChunkID 去重，保留第一个出现的）
func deduplicateVectors(vectors []*core.Vector) []*core.Vector {
	seen := make(map[string]bool)
	result := make([]*core.Vector, 0, len(vectors))
	for _, vec := range vectors {
		if vec == nil {
			continue
		}
		// 使用 ChunkID 而不是 vec.ID (UUID) 进行去重
		if !seen[vec.ChunkID] {
			seen[vec.ChunkID] = true
			result = append(result, vec)
		}
	}
	return result
}

// extractChunkContent 从 Vector 的 metadata 中提取 chunk 内容
func (s *semanticIndexer) extractChunkContent(vec *core.Vector) string {
	if vec == nil || vec.Metadata == nil {
		return ""
	}
	if content, ok := vec.Metadata["content"].(string); ok {
		return content
	}
	return ""
}

func (s *semanticIndexer) Remove(ctx context.Context, chunkID string) error {
	// 删除时通过 chunk_id 匹配：
	// - govector store 会识别 chunk_ 前缀，按 Payload["chunk_id"] filter 删除
	// - 这确保只删除指定分块的向量，保留同一文档的其他分块
	return s.db.Delete(ctx, chunkID)
}

func (s *semanticIndexer) NewQuery(terms string) core.Query {
	return query.NewSemanticQuery(terms, s.embedder)
}
