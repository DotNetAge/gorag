package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/logging"
	"github.com/DotNetAge/gorag/query"
)

// 使用向量数据库和向量模型进行索引及检索
type semanticIndexer struct {
	name     string
	db       core.VectorStore
	embedder core.Embedder
	logger   logging.Logger
}

// SemanticOption configures a semantic indexer.
type SemanticOption func(*semanticIndexer)

// WithSemanticLogger attaches a logger to the semantic indexer for observation logs.
func WithSemanticLogger(logger logging.Logger) SemanticOption {
	return func(s *semanticIndexer) {
		if logger != nil {
			s.logger = logger
		}
	}
}

func NewSemanticIndexer(db core.VectorStore, embedder core.Embedder, opts ...SemanticOption) core.Indexer {
	s := &semanticIndexer{
		name:     "semantic",
		db:       db,
		embedder: embedder,
		logger:   logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *semanticIndexer) Name() string {
	return s.name
}

func (s *semanticIndexer) Type() string {
	return "semantic"
}

func (s *semanticIndexer) Add(ctx context.Context, content string) ([]*core.Chunk, error) {
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
	if err := s.saveChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}

func (s *semanticIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
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
	if err := s.saveChunks(ctx, chunks); err != nil {
		return nil, err
	}
	return chunks, nil
}
// AddChunks 直接将分片插入向量数据库
func (s *semanticIndexer) AddChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks to add")
	}
	if err := s.saveChunks(ctx, chunks); err != nil {
		return err
	}
	return nil
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
		hit := core.Hit{
			ID:      vec.ChunkID,
			Score:   scores[i],
			Content: s.extractChunkContent(vec),
		}

		// 从 Vector.Metadata 中提取元信息
		if vec.Metadata != nil {
			// 提取 title
			if t, ok := vec.Metadata["title"].(string); ok {
				hit.Title = t
			}

			// 提取 doc_id
			if d, ok := vec.Metadata["doc_id"].(string); ok {
				hit.DocID = d
			}

			// 提取完整 metadata（排除内部使用字段）
			hit.Metadata = extractMetadata(vec.Metadata)

			// 提取 chunk_meta
			if cm, ok := vec.Metadata["chunk_meta"].(map[string]any); ok {
				hit.ChunkMeta = mapToChunkMeta(cm)
			}
		}

		hits = append(hits, hit)
	}

	return hits, nil
}

// GetByDocID retrieves all vectors belonging to the specified document.
// This is used for document reconstruction (knowledge traceability).
func (s *semanticIndexer) GetByDocID(ctx context.Context, docID string) ([]*core.Vector, error) {
	return s.db.GetByDocID(ctx, docID)
}

// ReconstructDocument reconstructs the original document from its stored chunks
// using the knowledge traceability system (doc_id → all chunks → sort by index → concatenate).
func (s *semanticIndexer) ReconstructDocument(ctx context.Context, docID string) (*core.ReconstructedDocument, error) {
	vectors, err := s.db.GetByDocID(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vectors by doc_id %s: %w", docID, err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("no chunks found for doc_id %s", docID)
	}
	doc := core.ReconstructDocument(vectors)
	return doc, nil
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

// IndexChunk indexes a pre-generated chunk (implements core.Indexer interface)
func (s *semanticIndexer) saveChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk cannot be nil")
	}
	return s.indexAndStore(ctx, chunk)
}

// StoreChunk stores a pre-built chunk directly in the index, skipping chunking.
// The chunk's Metadata is persisted as vector metadata for filter-based retrieval.
// This is used by the memory system to store MemoryChunk data.
func (s *semanticIndexer) StoreChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil || chunk.Content == "" {
		return fmt.Errorf("chunk content cannot be empty")
	}
	return s.indexAndStore(ctx, chunk)
}

// saveChunks indexes multiple pre-generated chunks in batch (implements core.ChunkIndexer interface).
// Emits a single batch-level INFO log "indexer.embedded" summarizing the embedding
// and upsert timings so the caller can see one line per batch instead of one per chunk.
func (s *semanticIndexer) saveChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	embedStart := time.Now()
	embedErrCount := 0
	for _, chunk := range chunks {
		if err := s.indexAndStore(ctx, chunk); err != nil {
			embedErrCount++
			s.logger.Error("indexer.embed failed", err, "chunk_id", chunk.ID)
		}
	}
	embedDur := time.Since(embedStart)

	s.logger.Info("indexer.embedded",
		"chunks", len(chunks),
		"failed", embedErrCount,
		"duration_ms", embedDur.Milliseconds(),
	)

	if embedErrCount > 0 {
		return fmt.Errorf("embedding failed for %d/%d chunks", embedErrCount, len(chunks))
	}
	return nil
}

func (s *semanticIndexer) NewQuery(terms string) core.Query {
	return query.NewSemanticQuery(terms, s.embedder)
}

// List returns paginated hits from the vector store.
// Converts Vector results to Hit format for browsing.
func (s *semanticIndexer) List(ctx context.Context, offset, limit int) ([]core.Hit, error) {
	vectors, err := s.db.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return []core.Hit{}, nil
	}

	hits := make([]core.Hit, 0, len(vectors))
	for _, vec := range vectors {
		hit := vectorToHit(vec)
		hits = append(hits, hit)
	}
	return hits, nil
}

// GetChunks returns all chunks belonging to the specified document.
// Converts vectors (with chunk metadata) back to Chunk objects.
func (s *semanticIndexer) GetChunks(ctx context.Context, docId string) ([]*core.Chunk, error) {
	vectors, err := s.db.GetByDocID(ctx, docId)
	if err != nil {
		return nil, fmt.Errorf("failed to get vectors by doc_id %s: %w", docId, err)
	}
	if len(vectors) == 0 {
		return []*core.Chunk{}, nil
	}

	chunks := make([]*core.Chunk, 0, len(vectors))
	for _, vec := range vectors {
		if vec == nil || vec.Metadata == nil {
			continue
		}
		chunk := &core.Chunk{
			ID:       vec.ChunkID,
			Content:  "",
			Metadata: map[string]any{},
		}
		if content, ok := vec.Metadata["content"].(string); ok {
			chunk.Content = content
		}
		if did, ok := vec.Metadata["doc_id"].(string); ok {
			chunk.DocID = did
		}
		if pid, ok := vec.Metadata["parent_id"].(string); ok {
			chunk.ParentID = pid
		}
		if mt, ok := vec.Metadata["mime_type"].(string); ok {
			chunk.MIMEType = mt
		}
		if cm, ok := vec.Metadata["chunk_meta"].(map[string]any); ok {
			chunk.ChunkMeta = mapToChunkMeta(cm)
		}
		// Copy non-internal metadata
		for k, v := range vec.Metadata {
			switch k {
			case "content", "doc_id", "parent_id", "mime_type", "chunk_meta":
				// skip internal fields already mapped above
			default:
				chunk.Metadata[k] = v
			}
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

// Close closes the underlying vector store to release resources (e.g., bbolt file locks)
func (s *semanticIndexer) Close(ctx context.Context) error {
	return s.db.Close(ctx)
}

// extractMetadata 从 Vector.Metadata 中提取原始 Chunk.Metadata
// 排除内部使用字段（doc_id, parent_id, content, mime_type, chunk_meta）
func extractMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}

	internalFields := map[string]bool{
		"doc_id":     true,
		"parent_id":  true,
		"content":    true,
		"mime_type":  true,
		"chunk_meta": true,
	}

	result := make(map[string]any)
	for k, v := range meta {
		if !internalFields[k] {
			result[k] = v
		}
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// mapToChunkMeta 将 map[string]any 转换为 core.ChunkMeta
func mapToChunkMeta(m map[string]any) core.ChunkMeta {
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

// vectorToHit converts a Vector to a Hit for browsing purposes.
func vectorToHit(vec *core.Vector) core.Hit {
	hit := core.Hit{
		ID:      vec.ChunkID,
		Content: "",
	}

	if vec == nil || vec.Metadata == nil {
		return hit
	}

	if content, ok := vec.Metadata["content"].(string); ok {
		hit.Content = content
	}
	if title, ok := vec.Metadata["title"].(string); ok {
		hit.Title = title
	}
	if docID, ok := vec.Metadata["doc_id"].(string); ok {
		hit.DocID = docID
	}
	if cm, ok := vec.Metadata["chunk_meta"].(map[string]any); ok {
		hit.ChunkMeta = mapToChunkMeta(cm)
	}

	// Copy non-internal metadata
	metadata := make(map[string]any)
	for k, v := range vec.Metadata {
		switch k {
		case "content", "title", "doc_id", "parent_id", "mime_type", "chunk_meta":
			// skip internal fields
		default:
			metadata[k] = v
		}
	}
	if len(metadata) > 0 {
		hit.Metadata = metadata
	}

	return hit
}

// Count returns the total number of indexed chunks.
func (s *semanticIndexer) Count(ctx context.Context) (int, error) {
	return s.db.Count(ctx)
}


