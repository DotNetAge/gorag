package indexer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/query"
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

// ── 多维度从属索引 ──────────────────────────────────────────────
// 为同一 chunk 增加多个向量维度（主索引 content 不变），解决短文本查询命中长内容向量困难的问题。
// 从属向量 ChunkID = <chunk_id>:<suffix>，metadata 留空（不重复存储主向量数据），
// 命中后回查主向量获取完整内容。

// vectorDimension 描述一个从属索引维度
type vectorDimension struct {
	suffix  string                      // ":title" / ":summary"
	extract func(map[string]any) string // 从 metadata 提取向量化文本
}

var (
	dimTitle = vectorDimension{":title", func(m map[string]any) string {
		t, _ := m["title"].(string)
		return t
	}}
	dimSummary = vectorDimension{":summary", func(m map[string]any) string {
		s, _ := m["summary"].(string)
		return s
	}}
)

// semanticDimensions 是 semanticIndexer 启用的从属维度（规则分块器无 summary，仅 title）
var semanticDimensions = []vectorDimension{dimTitle}

// graphDimensions 是 GraphIndexer 启用的从属维度（LLM 生成，含 title + summary）
var graphDimensions = []vectorDimension{dimTitle, dimSummary}

// indexDimensionVectors 为 chunk 生成所有从属维度的向量并 Upsert。
// 对应字段为空的维度自然跳过。供 semanticIndexer 和 GraphIndexer 共用。
func indexDimensionVectors(ctx context.Context, db core.VectorStore, embedder core.Embedder, chunkID string, metadata map[string]any, dims []vectorDimension) error {
	for _, dim := range dims {
		text := dim.extract(metadata)
		if text == "" {
			continue
		}
		vec, err := embedder.CalcText(text)
		if err != nil {
			return err
		}
		if vec == nil {
			continue
		}
		vec.ChunkID = chunkID + dim.suffix
		if err := db.Upsert(ctx, []*core.Vector{vec}); err != nil {
			return err
		}
	}
	return nil
}

// stripDimSuffix 检查 chunkID 是否为某个从属维度（以已知后缀结尾），
// 是则返回原 chunk_id 和 true，否则返回原值和 false。
func stripDimSuffix(chunkID string, dims []vectorDimension) (string, bool) {
	for _, dim := range dims {
		if strings.HasSuffix(chunkID, dim.suffix) {
			return strings.TrimSuffix(chunkID, dim.suffix), true
		}
	}
	return chunkID, false
}

// getVectorByChunkID 按 chunk_id 精确查询单个向量（用于从属索引回查）。
// 复用 ListFiltered + chunk_id exact 匹配，无需扩展 VectorStore 接口。
func getVectorByChunkID(ctx context.Context, db core.VectorStore, chunkID string) (*core.Vector, error) {
	vecs, _, err := db.ListFiltered(ctx, 0, 1, []core.FilterCondition{
		{Key: "chunk_id", Type: "exact", Value: chunkID},
	})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	return vecs[0], nil
}

// resolveDimensions 处理搜索结果中的从属维度向量：
// 后缀匹配的向量回查主向量获取完整数据，按原 chunk_id 去重保留较高分。
// 返回处理后的 results 和 scores（保持对齐）。供 semanticIndexer 和 GraphIndexer 共用。
func resolveDimensions(ctx context.Context, db core.VectorStore, results []*core.Vector, scores []float32, dims []vectorDimension) ([]*core.Vector, []float32, error) {
	type entry struct {
		vec   *core.Vector
		score float32
	}
	byID := make(map[string]*entry)
	order := make([]string, 0, len(results))
	for i, vec := range results {
		if vec == nil {
			continue
		}
		score := scores[i]
		chunkID := vec.ChunkID
		if baseID, ok := stripDimSuffix(chunkID, dims); ok {
			mainVec, err := getVectorByChunkID(ctx, db, baseID)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve dimension for %s: %w", chunkID, err)
			}
			if mainVec == nil {
				continue
			}
			vec = mainVec
			chunkID = baseID
		}
		if e, exists := byID[chunkID]; exists {
			if score > e.score {
				e.score = score
			}
			continue
		}
		byID[chunkID] = &entry{vec: vec, score: score}
		order = append(order, chunkID)
	}
	out := make([]*core.Vector, 0, len(order))
	outScores := make([]float32, 0, len(order))
	for _, id := range order {
		e := byID[id]
		out = append(out, e.vec)
		outScores = append(outScores, e.score)
	}
	return out, outScores, nil
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

	// 5. 从属维度处理：后缀匹配的向量回查主向量，按原 chunk_id 去重保留较高分
	results, scores, err = resolveDimensions(ctx, s.db, results, scores, semanticDimensions)
	if err != nil {
		return nil, err
	}

	// 6. 构建 Hit 返回
	// 注意：返回主向量的 ChunkID（chunk ID）而不是 vec.ID（UUID），
	// 与 fulltextIndexer.Search 返回的 ID 格式一致，确保混合搜索 RRF 融合时能正确匹配。
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
	// 删除主向量（通过 chunk_id 匹配：govector store 按 Payload["chunk_id"] filter 删除）
	if err := s.db.Delete(ctx, chunkID); err != nil {
		return err
	}
	// 联动删除所有从属维度向量（无对应维度的 chunk 删不到也无副作用）
	for _, dim := range semanticDimensions {
		if err := s.db.Delete(ctx, chunkID+dim.suffix); err != nil {
			// 从属维度向量可能不存在（无对应字段可提取），不视为错误
			s.logger.Warn("remove dimension vector: %v", err)
		}
	}
	return nil
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

// Refill 为已有分片补充从属维度的向量（title/summary 等）。
// 用于存量数据迁移到多维度索引。幂等：已存在的从属向量会跳过，支持中断重跑。
func (s *semanticIndexer) Refill(ctx context.Context) error {
	const pageSize = 100
	offset := 0
	refilled := 0
	for {
		vecs, err := s.db.List(ctx, offset, pageSize)
		if err != nil {
			return fmt.Errorf("refill list at offset %d: %w", offset, err)
		}
		if len(vecs) == 0 {
			break
		}
		for _, vec := range vecs {
			// 跳过从属维度向量，只处理主向量
			if _, isDim := stripDimSuffix(vec.ChunkID, semanticDimensions); isDim {
				continue
			}
			if vec.Metadata == nil {
				continue
			}
			// 遍历所有从属维度，逐个检查并补充
			for _, dim := range semanticDimensions {
				text := dim.extract(vec.Metadata)
				if text == "" {
					continue
				}
				dimID := vec.ChunkID + dim.suffix
				// 幂等检查：已存在则跳过
				existing, err := getVectorByChunkID(ctx, s.db, dimID)
				if err != nil {
					return fmt.Errorf("refill check %s: %w", dimID, err)
				}
				if existing != nil {
					continue
				}
				dimVec, err := s.embedder.CalcText(text)
				if err != nil {
					s.logger.Error("refill embed dimension failed", err, "chunk_id", dimID)
					continue
				}
				if dimVec == nil {
					continue
				}
				dimVec.ChunkID = dimID
				if err := s.db.Upsert(ctx, []*core.Vector{dimVec}); err != nil {
					s.logger.Error("refill upsert dimension failed", err, "chunk_id", dimID)
					continue
				}
				refilled++
			}
		}
		offset += len(vecs)
		if len(vecs) < pageSize {
			break
		}
	}
	s.logger.Info("indexer.refilled", "refilled", refilled)
	return nil
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
			continue
		}
		// 补充从属维度向量（title 等），无对应字段的 chunk 自然跳过
		if err := indexDimensionVectors(ctx, s.db, s.embedder, chunk.ID, chunk.Metadata, semanticDimensions); err != nil {
			embedErrCount++
			s.logger.Error("indexer.embed dimensions failed", err, "chunk_id", chunk.ID)
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

// Clear removes all vectors from the semantic indexer.
func (s *semanticIndexer) Clear(ctx context.Context) error {
	return s.db.Clear(ctx)
}
