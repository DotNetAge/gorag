package indexer

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DotNetAge/gorag/v2/chunker"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/document"
	"github.com/DotNetAge/gorag/v2/logging"
	"github.com/DotNetAge/gorag/v2/query"
)

// semanticIndexer 语义索引器：使用向量数据库和向量模型进行索引及检索。
//
// 实现 4 个接口：
//   - Indexer（核心）：Name / AddFile / Search / NewQuery
//   - IndexerStore（存储）：Save
//   - IndexerAdmin（管理）：List / GetChunks / Count / Remove / Clear
//   - IndexerCloser（资源）：Close
//
// 不实现 TreeViewBuilder / GraphSearcher（仅 GraphIndexer 实现）。
type semanticIndexer struct {
	name     string
	db       core.VectorStore
	embedder core.Embedder
	chunker  chunker.Chunker // 可选注入，nil 时 AddFile 内部用 chunker.New 路由
	logger   logging.Logger
}

// SemanticOption 配置 semanticIndexer 的可选参数。
type SemanticOption func(*semanticIndexer)

// WithSemanticLogger 为语义索引器附加日志记录器。
func WithSemanticLogger(logger logging.Logger) SemanticOption {
	return func(s *semanticIndexer) {
		if logger != nil {
			s.logger = logger
		}
	}
}

// WithSemanticChunker 注入自定义 Chunker，覆盖默认的 chunker.New 路由。
// 不传则 AddFile 内部按 RawDoc.Type 调用 chunker.New 选择实现。
func WithSemanticChunker(c chunker.Chunker) SemanticOption {
	return func(s *semanticIndexer) {
		if c != nil {
			s.chunker = c
		}
	}
}

// NewSemanticIndexer 创建语义索引器，返回 Indexer 接口。
//
// 必传参数：
//   - db：向量存储，nil 返回 error
//   - embedder：向量计算器，nil 返回 error
//
// 可选参数通过 WithSemanticLogger / WithSemanticChunker 注入。
func NewSemanticIndexer(db core.VectorStore, embedder core.Embedder, opts ...SemanticOption) (Indexer, error) {
	if db == nil {
		return nil, fmt.Errorf("NewSemanticIndexer: db 不能为空")
	}
	if embedder == nil {
		return nil, fmt.Errorf("NewSemanticIndexer: embedder 不能为空")
	}
	s := &semanticIndexer{
		name:     "semantic",
		db:       db,
		embedder: embedder,
		logger:   logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Name 实现 Indexer 接口。
func (s *semanticIndexer) Name() string {
	return s.name
}

// AddFile 实现 Indexer 接口：从文件读取内容后执行索引全流程。
//
// 流程：document.Open → core.NewStructuredDoc → chunker.Chunk → doc.SetChunks → s.Save。
// 返回本次生成的 []*core.Chunk（从 ChunkResult.Chunks 转换）。
// filePath 必须为绝对路径。
func (s *semanticIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
	if filePath == "" {
		return nil, fmt.Errorf("AddFile: filePath 不能为空")
	}
	s.logger.Debug("语义索引器: 开始索引文件", "file", filePath)
	// 1. 读取并归一化文件
	raw, err := document.Open(filePath)
	if err != nil {
		s.logger.Error("语义索引器: 打开文件失败", err, "file", filePath)
		return nil, fmt.Errorf("AddFile: 打开文件失败: %w", err)
	}
	// 1a. 非多模态 embedder 跳过图片文件（如 BGE 不支持图片索引）
	if raw.Type() == document.RawDocImage && !s.embedder.Multimoding() {
		s.logger.Info("语义索引器: embedder 不支持图片索引，跳过图片文件", "file", filePath)
		return nil, nil
	}
	s.logger.Debug("语义索引器: 文件归一化完成",
		"file", filePath,
		"doc_type", raw.Type(),
		"doc_id", raw.ID())
	// 2. 创建结构化文档容器（Chunks/Nodes/Edges 此时均为空）
	doc, err := core.Structurize(raw)
	if err != nil {
		s.logger.Error("语义索引器: 创建 StructuredDoc 失败", err, "file", filePath)
		return nil, fmt.Errorf("AddFile: 创建 StructuredDoc 失败: %w", err)
	}
	// 3. 选择 Chunker：注入优先，否则按 RawDoc.Type 路由
	c := s.chunker
	if c == nil {
		c, err = chunker.New(raw)
		if err != nil {
			s.logger.Error("语义索引器: 选择 Chunker 失败", err,
				"file", filePath, "doc_type", raw.Type())
			return nil, fmt.Errorf("AddFile: 选择 Chunker 失败: %w", err)
		}
	}
	// 4. 分块（ChunkResult 含 Chunks/Nodes/Edges，本索引器仅消费 Chunks）
	result, err := c.Chunk(raw)
	if err != nil {
		s.logger.Error("语义索引器: 分块失败", err, "file", filePath)
		return nil, fmt.Errorf("AddFile: 分块失败: %w", err)
	}
	if len(result.Chunks) == 0 {
		s.logger.Warn("语义索引器: 文件未生成任何分片", "file", filePath)
		return nil, fmt.Errorf("AddFile: 文件未生成任何分片")
	}
	s.logger.Debug("语义索引器: 分块完成",
		"file", filePath,
		"chunks", len(result.Chunks),
		"nodes", len(result.Nodes),
		"edges", len(result.Edges))
	// 5. 写入 doc 并调用 Save 走统一的向量化与存储路径
	doc.SetChunks(result.Chunks)
	if err := s.Save(ctx, doc); err != nil {
		return nil, err
	}
	// 6. 转换为 []*core.Chunk 返回（Chunks 为值切片，取地址需先复制到本地变量）
	chunks := make([]*core.Chunk, 0, len(result.Chunks))
	for i := range result.Chunks {
		ch := result.Chunks[i]
		chunks = append(chunks, &ch)
	}
	s.logger.Debug("语义索引器: 索引文件完成", "file", filePath, "chunks", len(chunks))
	return chunks, nil
}

// Save 实现 IndexerStore 接口：将 StructuredDoc 写入 VectorStore。
//
// 流程：
//  1. 从 doc.Chunks() 读取分片
//  2. 对每个 chunk 按 Content / Title / Summary 三个维度生成向量：
//     - 主向量（Content）：ChunkID = chunk.ID，携带完整 metadata
//     - 从属向量（Title）：ChunkID = chunk.ID + ":title"，不存 metadata（Title 非空时）
//     - 从属向量（Summary）：ChunkID = chunk.ID + ":summary"，不存 metadata（Summary 非空时）
//  3. 主向量 metadata 包含 content/title/summary/doc_id/source/region_id 等字段（使用 core.VecMeta* 常量键名），便于 Search 重建 Chunk
func (s *semanticIndexer) Save(ctx context.Context, doc core.StructuredDoc) error {
	if doc == nil {
		return fmt.Errorf("Save: doc 不能为空")
	}
	chunks := doc.Chunks()
	if len(chunks) == 0 {
		s.logger.Debug("语义索引器: 无分片可保存，跳过 Save")
		return nil // 无分片可保存，不视为错误
	}
	s.logger.Debug("语义索引器: 开始保存分片", "chunks", len(chunks))

	embedStart := time.Now()
	embedErrCount := 0
	for i := range chunks {
		if err := s.saveOneChunk(ctx, &chunks[i]); err != nil {
			embedErrCount++
			s.logger.Error("语义索引器: 向量化失败", err,
				"chunk_id", chunks[i].ID,
				"doc_id", chunks[i].DocID)
			continue
		}
	}
	embedDur := time.Since(embedStart)

	s.logger.Info("语义索引器: 向量化完成",
		"chunks", len(chunks),
		"failed", embedErrCount,
		"duration_ms", embedDur.Milliseconds(),
	)

	if embedErrCount > 0 {
		return fmt.Errorf("向量化失败 %d/%d 个分片", embedErrCount, len(chunks))
	}
	return nil
}

// saveOneChunk 为单个 chunk 生成 Content / Title / Summary 三个维度的向量并写入 VectorStore。
//
// 设计要点（从属维度是概念而非数据维度）：
//   - 主向量（Content）：ChunkID = chunk.ID，携带完整 metadata，是数据之源
//   - 从属向量（Title/Summary）：ChunkID = chunk.ID + ":title" / ":summary"
//     **不存 metadata**——通过 ChunkID 后缀即可识别其所属维度，
//     命中后通过 stripDimSuffix 反解出主 ChunkID，再回查主向量获取完整 Chunk 数据
//   - 这样从属向量只占用最小存储空间，维度信息隐含在 ChunkID 编码中
func (s *semanticIndexer) saveOneChunk(ctx context.Context, chunk *core.Chunk) error {
	s.logger.Debug("语义索引器: 开始向量化分片",
		"chunk_id", chunk.ID,
		"doc_id", chunk.DocID,
		"has_title", chunk.Title != "",
		"has_summary", chunk.Summary != "",
		"content_len", utf8.RuneCountInString(chunk.Content))
	// 1. 主向量（Content）——携带完整 metadata
	if chunk.Content != "" {
		mainVec, err := s.embedder.CalcText(chunk.Content)
		if err != nil {
			return fmt.Errorf("编码 Content 失败: %w", err)
		}
		if mainVec != nil {
			mainVec.ChunkID = chunk.ID
			mainVec.Metadata = buildVectorMetadata(chunk)
			if err := s.db.Upsert(ctx, []*core.Vector{mainVec}); err != nil {
				return fmt.Errorf("写入主向量失败: %w", err)
			}
		}
	}

	// 2. 从属向量（Title）——不存 metadata，通过 ChunkID 后缀标识
	if chunk.Title != "" {
		titleVec, err := s.embedder.CalcText(chunk.Title)
		if err != nil {
			return fmt.Errorf("编码 Title 失败: %w", err)
		}
		if titleVec != nil {
			titleVec.ChunkID = chunk.ID + ":title"
			// 从属向量不存 metadata：命中后通过后缀反解回查主向量
			if err := s.db.Upsert(ctx, []*core.Vector{titleVec}); err != nil {
				return fmt.Errorf("写入 Title 从属向量失败: %w", err)
			}
		}
	}

	// 3. 从属向量（Summary）——不存 metadata，通过 ChunkID 后缀标识
	if chunk.Summary != "" {
		summaryVec, err := s.embedder.CalcText(chunk.Summary)
		if err != nil {
			return fmt.Errorf("编码 Summary 失败: %w", err)
		}
		if summaryVec != nil {
			summaryVec.ChunkID = chunk.ID + ":summary"
			// 从属向量不存 metadata：命中后通过后缀反解回查主向量
			if err := s.db.Upsert(ctx, []*core.Vector{summaryVec}); err != nil {
				return fmt.Errorf("写入 Summary 从属向量失败: %w", err)
			}
		}
	}
	return nil
}

// vecMetaKeys 集合用于过滤已映射到 Chunk 顶层字段的 VecMeta* 键名。
// buildVectorMetadata 与 vectorToChunk 在复制 Metadata 时跳过这些键，
// 避免顶层字段与 Metadata 中的同名键重复存储。
var vecMetaKeys = map[string]bool{
	core.VecMetaContent:   true,
	core.VecMetaTitle:     true,
	core.VecMetaSummary:   true,
	core.VecMetaDocID:     true,
	core.VecMetaParentID:  true,
	core.VecMetaDir:       true,
	core.VecMetaFileName:  true,
	core.VecMetaRegionID:  true,
	core.VecMetaLanguage:  true,
	core.VecMetaTags:      true,
	core.VecMetaStartLine: true,
	core.VecMetaEndLine:   true,
	core.VecMetaStartPos:  true,
	core.VecMetaEndPos:    true,
	core.VecMetaIndex:     true,
}

// toIntFromMeta 从 VecMeta 元数据中安全提取 int 值。
// govector 存储整数时为 int64（protobuf），JSON 反序列化后为 float64。
func toIntFromMeta(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int64:
		return int(val)
	case float64:
		return int(val)
	case int:
		return val
	}
	return 0
}

// buildVectorMetadata 从 Chunk 构造 Vector 的 metadata 快照。
// 顶层字段使用 core.VecMeta* 常量键名序列化，与 vectorToChunk 反向对应。
// Metadata 中的非 VecMeta 键原样复制（如 directory/heading_level 等扩展字段）。
func buildVectorMetadata(chunk *core.Chunk) map[string]any {
	m := map[string]any{
		core.VecMetaContent:  chunk.Content,
		core.VecMetaDocID:    chunk.DocID,
		core.VecMetaParentID: chunk.ParentID,
		core.VecMetaDir:      chunk.Dir,
		core.VecMetaFileName: chunk.FileName,
		core.VecMetaRegionID: chunk.RegionID,
	}
	if chunk.Title != "" {
		m[core.VecMetaTitle] = chunk.Title
	}
	if chunk.Summary != "" {
		m[core.VecMetaSummary] = chunk.Summary
	}
	if chunk.Language != "" {
		m[core.VecMetaLanguage] = chunk.Language
	}
	if len(chunk.Tags) > 0 {
		m[core.VecMetaTags] = chunk.Tags
	}
	if chunk.StartLine > 0 {
		m[core.VecMetaStartLine] = chunk.StartLine
	}
	if chunk.EndLine > 0 {
		m[core.VecMetaEndLine] = chunk.EndLine
	}
	// 保存原始字节偏移位置；0 也是合法起始位置，因此只要有一个非零就写入
	if chunk.StartPos != 0 || chunk.EndPos != 0 {
		m[core.VecMetaStartPos] = chunk.StartPos
		m[core.VecMetaEndPos] = chunk.EndPos
	}
	// 始终保存 Index，用于查询时按文档顺序排序
	m[core.VecMetaIndex] = chunk.Index
	// 复制 Metadata 中的其他扩展属性（跳过已映射到顶层字段的 VecMeta* 键）
	for k, v := range chunk.Metadata {
		if _, isVecMeta := vecMetaKeys[k]; isVecMeta {
			continue
		}
		m[k] = v
	}
	return m
}

// ── 从属向量维度辅助 ──────────────────────────────────────────────
// 为同一 chunk 增加附属向量（title/summary），解决短文本查询命中长内容向量困难的问题。
// 从属向量 ChunkID = <chunk_id>:<suffix>，**不存 metadata**，
// 命中后通过后缀反解出主 ChunkID，回查主向量获取完整 Chunk 数据。

// vectorDimension 描述一个从属向量维度（概念维度，非数据维度）。
// 维度信息隐含在 ChunkID 的后缀编码中，不需要额外的 Dim 字段。
type vectorDimension struct {
	suffix  string                   // ":title" / ":summary"
	extract func(*core.Chunk) string // 从主向量对应的 Chunk 提取向量化文本（供 Refill 使用）
}

var (
	dimTitle   = vectorDimension{":title", func(c *core.Chunk) string { return c.Title }}
	dimSummary = vectorDimension{":summary", func(c *core.Chunk) string { return c.Summary }}
)

// semanticDimensions 是 SemanticIndexer 启用的从属维度（title + summary）。
var semanticDimensions = []vectorDimension{dimTitle, dimSummary}

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
// 复用 List + chunk_id exact 匹配，无需扩展 VectorStore 接口。
func getVectorByChunkID(ctx context.Context, db core.VectorStore, chunkID string) (*core.Vector, error) {
	vecs, _, err := db.List(ctx, 0, 1, []core.FilterCondition{
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
// 返回处理后的 results 和 scores（保持对齐）。
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
				return nil, nil, fmt.Errorf("回查从属维度 %s 失败: %w", chunkID, err)
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

// Search 实现 Indexer 接口：执行语义检索，返回 *core.Hit 容器。
//
// 流程：
//  1. 从 Query 取/算查询向量
//  2. VectorStore.Search 取 topK
//  3. resolveParentChunks 处理父子块替换
//  4. resolveDimensions 处理从属维度回查与去重
//  5. 构建 *core.Hit（填充 Chunks）
func (s *semanticIndexer) Search(ctx context.Context, q core.Query) (*core.Hit, error) {
	if q == nil {
		return nil, fmt.Errorf("Search: query 不能为空")
	}
	s.logger.Debug("语义索引器: 开始检索", "query", q.Raw(), "type", q.Type())

	// 1. 从查询获取向量 - 优先使用 Query 中的预计算向量，否则实时计算
	var queryVector []float32
	if emb := q.Embedding(); emb != nil {
		queryVector = emb
		s.logger.Debug("语义索引器: 使用预计算查询向量", "dim", len(queryVector))
	} else {
		vec, err := s.embedder.CalcText(q.Raw())
		if err != nil {
			s.logger.Error("语义索引器: 计算查询向量失败", err, "query", q.Raw())
			return nil, fmt.Errorf("Search: 计算查询向量失败: %w", err)
		}
		queryVector = vec.Values
		// 缓存到 Query，避免重复计算
		q.SetEmbedding(queryVector)
		s.logger.Debug("语义索引器: 查询向量计算完成", "dim", len(queryVector))
	}

	// 2. 获取过滤器
	filters := q.Filters()

	// 3. 向量相似度搜索
	topK := 10
	results, scores, err := s.db.Search(ctx, queryVector, topK, filters)
	if err != nil {
		s.logger.Error("语义索引器: 向量检索失败", err,
			"query", q.Raw(), "top_k", topK)
		return nil, fmt.Errorf("Search: 向量检索失败: %w", err)
	}
	s.logger.Debug("语义索引器: 向量检索完成",
		"results", len(results),
		"top_k", topK)

	// 4. ParentDoc 处理：如果结果是子块，替换为父块
	results = s.resolveParentChunks(results)

	// 5. 从属维度处理：后缀匹配的向量回查主向量，按原 chunk_id 去重保留较高分
	results, scores, err = resolveDimensions(ctx, s.db, results, scores, semanticDimensions)
	if err != nil {
		s.logger.Error("语义索引器: 从属维度处理失败", err)
		return nil, err
	}

	// 6. 构建 *core.Hit 返回（填充 Chunks）
	//    resolveDimensions 已将从属向量替换为主向量，此处所有 vec 均为主向量
	chunkHits := make([]core.ChunkHit, 0, len(results))
	for i, vec := range results {
		if vec == nil {
			continue
		}
		chunk := vectorToChunk(vec)
		chunkHits = append(chunkHits, core.ChunkHit{
			Chunk: chunk,
			Score: scores[i],
		})
	}

	s.logger.Debug("语义索引器: 检索完成",
		"query", q.Raw(),
		"raw_results", len(results),
		"chunk_hits", len(chunkHits),
		"top_score", topScore(scores))

	return &core.Hit{
		Query:  q,
		Score:  topScore(scores),
		Chunks: chunkHits,
	}, nil
}

// topScore 返回分数切片中的最高分（空切片返回 0）。
func topScore(scores []float32) float32 {
	if len(scores) == 0 {
		return 0
	}
	max := scores[0]
	for _, s := range scores[1:] {
		if s > max {
			max = s
		}
	}
	return max
}

// vectorToChunk 从 Vector 的 metadata 重建 Chunk 对象。
// 顶层字段使用 core.VecMeta* 常量键名反序列化，与 buildVectorMetadata 反向对应。
// 非 VecMeta 键原样复制到 Metadata（如 directory/heading_level 等扩展字段）。
func vectorToChunk(vec *core.Vector) *core.Chunk {
	chunk := &core.Chunk{
		ID:       vec.ChunkID,
		Metadata: map[string]any{},
	}
	if vec.Metadata == nil {
		return chunk
	}
	if v, ok := vec.Metadata[core.VecMetaContent].(string); ok {
		chunk.Content = v
	}
	if v, ok := vec.Metadata[core.VecMetaTitle].(string); ok {
		chunk.Title = v
	}
	if v, ok := vec.Metadata[core.VecMetaSummary].(string); ok {
		chunk.Summary = v
	}
	if v, ok := vec.Metadata[core.VecMetaDocID].(string); ok {
		chunk.DocID = v
	}
	if v, ok := vec.Metadata[core.VecMetaParentID].(string); ok {
		chunk.ParentID = v
	}
	if v, ok := vec.Metadata[core.VecMetaDir].(string); ok {
		chunk.Dir = v
	}
	if v, ok := vec.Metadata[core.VecMetaFileName].(string); ok {
		chunk.FileName = v
	}
	if v, ok := vec.Metadata[core.VecMetaRegionID].(string); ok {
		chunk.RegionID = v
	}
	if v, ok := vec.Metadata[core.VecMetaLanguage].(string); ok {
		chunk.Language = v
	}
	if tags, ok := vec.Metadata[core.VecMetaTags].([]string); ok {
		chunk.Tags = tags
	} else if tagsAny, ok := vec.Metadata[core.VecMetaTags].([]any); ok {
		// JSON 反序列化后可能是 []any
		for _, t := range tagsAny {
			if s, ok := t.(string); ok {
				chunk.Tags = append(chunk.Tags, s)
			}
		}
	}
	chunk.StartLine = toIntFromMeta(vec.Metadata[core.VecMetaStartLine])
	chunk.EndLine = toIntFromMeta(vec.Metadata[core.VecMetaEndLine])
	chunk.StartPos = toIntFromMeta(vec.Metadata[core.VecMetaStartPos])
	chunk.EndPos = toIntFromMeta(vec.Metadata[core.VecMetaEndPos])
	chunk.Index = toIntFromMeta(vec.Metadata[core.VecMetaIndex])
	// 复制非 VecMeta 键到 Metadata
	for k, v := range vec.Metadata {
		if _, isVecMeta := vecMetaKeys[k]; isVecMeta {
			continue
		}
		chunk.Metadata[k] = v
	}
	return chunk
}

// GetByDocID 检索指定文档的所有向量（非接口方法，供文档重建使用）。
func (s *semanticIndexer) GetByDocID(ctx context.Context, docID string) ([]*core.Vector, error) {
	return s.db.GetByDocID(ctx, docID)
}

// ReconstructDocument 从存储的分片重建原文档（非接口方法）。
// 降级实现：按 chunk_id 顺序拼接 content。
func (s *semanticIndexer) ReconstructDocument(ctx context.Context, docID string) (string, error) {
	vectors, err := s.db.GetByDocID(ctx, docID)
	if err != nil {
		s.logger.Error("语义索引器: 重建文档获取向量失败", err, "doc_id", docID)
		return "", fmt.Errorf("获取文档 %s 的向量失败: %w", docID, err)
	}
	if len(vectors) == 0 {
		s.logger.Warn("语义索引器: 重建文档未找到分片", "doc_id", docID)
		return "", fmt.Errorf("文档 %s 未找到任何分片", docID)
	}
	// 降级实现：按 chunk_id 顺序拼接 content
	var parts []string
	for _, v := range vectors {
		if v == nil || v.Metadata == nil {
			continue
		}
		if c, ok := v.Metadata[core.VecMetaContent].(string); ok && c != "" {
			parts = append(parts, c)
		}
	}
	s.logger.Debug("语义索引器: 文档重建完成",
		"doc_id", docID, "vectors", len(vectors), "parts", len(parts))
	return fmt.Sprintf("%s", parts), nil
}

// resolveParentChunks 处理 ParentDoc 分块结果。
// 如果匹配到子块，用父块替换；父块直接返回。
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
		if isParent, _ := vec.Metadata[core.MetaIsParent].(bool); !isParent {
			if parentID, ok := vec.Metadata[core.VecMetaParentID].(string); ok && parentID != "" {
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

// deduplicateVectors 去除重复的向量（按 ChunkID 去重，保留第一个出现的）。
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

// Remove 实现 IndexerAdmin 接口：按 chunkID 移除索引项。
// 联动删除所有从属维度向量（无对应维度的 chunk 删不到也无副作用）。
func (s *semanticIndexer) Remove(ctx context.Context, chunkID string) error {
	if chunkID == "" {
		return fmt.Errorf("Remove: chunkID 不能为空")
	}
	s.logger.Debug("语义索引器: 删除分片", "chunk_id", chunkID)
	// 删除主向量
	if err := s.db.Delete(ctx, chunkID); err != nil {
		s.logger.Error("语义索引器: 删除主向量失败", err, "chunk_id", chunkID)
		return err
	}
	// 联动删除所有从属维度向量
	for _, dim := range semanticDimensions {
		dimID := chunkID + dim.suffix
		if err := s.db.Delete(ctx, dimID); err != nil {
			// 从属维度向量可能不存在（无对应字段可提取），不视为错误
			s.logger.Debug("语义索引器: 从属维度向量不存在或删除失败（可忽略）",
				"chunk_id", dimID, "error", err.Error())
		}
	}
	s.logger.Debug("语义索引器: 删除分片完成", "chunk_id", chunkID)
	return nil
}

// UpdateVectorMetadata 按 chunkID 更新主向量（Content 维度）的 metadata。
//
// 与 Remove + Save 的区别：直接在原向量上修改 metadata 并以原 ID 写回
// （VectorStore.Upsert 对已存在 ID 为 replace 语义），不触发向量删除，
// 避免 HNSW 图在 Remove 后结构不一致的问题。从属维度向量不存 metadata，
// 无需处理。
//
// 典型用途：删除会话时把记忆 chunk 的 session_id 归属更新到最近使用的会话。
func (s *semanticIndexer) UpdateVectorMetadata(ctx context.Context, chunkID string, patch map[string]any) error {
	if chunkID == "" {
		return fmt.Errorf("UpdateVectorMetadata: chunkID 不能为空")
	}
	all, _, err := s.db.List(ctx, 0, 1<<30, nil)
	if err != nil {
		return fmt.Errorf("UpdateVectorMetadata: 查询向量失败: %w", err)
	}
	var target *core.Vector
	for _, vec := range all {
		if vec == nil || vec.ChunkID != chunkID {
			continue
		}
		if _, isDim := stripDimSuffix(vec.ChunkID, semanticDimensions); isDim {
			continue // 只更新主向量
		}
		target = vec
		break
	}
	if target == nil {
		return fmt.Errorf("UpdateVectorMetadata: chunkID %s 不存在", chunkID)
	}
	// 复制一份再修改，避免污染底层存储缓存
	cloned := *target
	if cloned.Metadata == nil {
		cloned.Metadata = map[string]any{}
	}
	for k, v := range patch {
		cloned.Metadata[k] = v
	}
	return s.db.Upsert(ctx, []*core.Vector{&cloned})
}

// Refill 为已有分片补充从属维度的向量（title/summary 等）（非接口方法）。
// 用于存量数据迁移到多维度索引。幂等：已存在的从属向量会跳过，支持中断重跑。
func (s *semanticIndexer) Refill(ctx context.Context) error {
	const pageSize = 100
	offset := 0
	refilled := 0
	s.logger.Debug("语义索引器: 开始 Refill 从属维度向量")
	for {
		vecs, _, err := s.db.List(ctx, offset, pageSize, nil)
		if err != nil {
			s.logger.Error("语义索引器: Refill 分页查询失败", err, "offset", offset)
			return fmt.Errorf("Refill 分页查询 offset %d 失败: %w", offset, err)
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
			// 主向量 metadata 重建为 Chunk，供 dim.extract 读取顶层字段
			chunk := vectorToChunk(vec)
			// 遍历所有从属维度，逐个检查并补充
			for _, dim := range semanticDimensions {
				text := dim.extract(chunk)
				if text == "" {
					continue
				}
				dimID := vec.ChunkID + dim.suffix
				// 幂等检查：已存在则跳过
				existing, err := getVectorByChunkID(ctx, s.db, dimID)
				if err != nil {
					s.logger.Error("语义索引器: Refill 检查从属维度失败", err, "chunk_id", dimID)
					return fmt.Errorf("Refill 检查 %s 失败: %w", dimID, err)
				}
				if existing != nil {
					continue
				}
				dimVec, err := s.embedder.CalcText(text)
				if err != nil {
					s.logger.Error("语义索引器: Refill 编码从属维度失败", err, "chunk_id", dimID)
					continue
				}
				if dimVec == nil {
					continue
				}
				dimVec.ChunkID = dimID
				if err := s.db.Upsert(ctx, []*core.Vector{dimVec}); err != nil {
					s.logger.Error("语义索引器: Refill 写入从属维度失败", err, "chunk_id", dimID)
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
	s.logger.Info("语义索引器: Refill 完成", "refilled", refilled)
	return nil
}

// NewQuery 实现 Indexer 接口：构造查询对象。
// 使用 query.New(text) 创建默认实现，预设查询类型为 "semantic"。
func (s *semanticIndexer) NewQuery(terms string) core.Query {
	return query.New(terms)
}

// List 实现 IndexerAdmin 接口：分页浏览已索引的 Chunk。
//
// 多维度向量索引会为同一 Chunk 写入 Content/Title/Summary 三个向量，
// 其中从属向量（:title / :summary）不携带 metadata，仅服务于搜索召回。
// List 面向「浏览 Chunk」语义，因此只返回主向量（Content 维度）：
//   - 先获取全部匹配向量（VectorStore.List 内部已将全部点加载到内存，此处不额外增加开销）
//   - 过滤掉从属维度向量
//   - 在内存中按主向量分页
//
// 这样返回的 total 与分页都与 Chunk 语义一致，避免出现空内容的维度占位项。
func (s *semanticIndexer) List(ctx context.Context, offset, limit int, filters []core.FilterCondition) ([]core.Chunk, int, error) {
	all, _, err := s.db.List(ctx, 0, 1<<30, filters)
	if err != nil {
		s.logger.Error("语义索引器: List 查询失败", err,
			"offset", offset, "limit", limit)
		return nil, 0, fmt.Errorf("List: 查询失败: %w", err)
	}

	main := make([]*core.Vector, 0, len(all))
	for _, vec := range all {
		if vec == nil {
			continue
		}
		if _, isDim := stripDimSuffix(vec.ChunkID, semanticDimensions); isDim {
			continue
		}
		main = append(main, vec)
	}
	total := len(main)

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	end := min(offset+limit, total)
	if offset >= total || end <= offset {
		return []core.Chunk{}, total, nil
	}

	chunks := make([]core.Chunk, 0, end-offset)
	for _, vec := range main[offset:end] {
		chunks = append(chunks, *vectorToChunk(vec))
	}
	s.logger.Debug("语义索引器: List 完成",
		"offset", offset, "limit", limit, "total", total, "returned", len(chunks))
	return chunks, total, nil
}

// GetChunks 实现 IndexerAdmin 接口：按 docID 获取该文档的所有 Chunk。
func (s *semanticIndexer) GetChunks(ctx context.Context, docID string) ([]*core.Chunk, error) {
	if docID == "" {
		return nil, fmt.Errorf("GetChunks: docID 不能为空")
	}
	vectors, err := s.db.GetByDocID(ctx, docID)
	if err != nil {
		s.logger.Error("语义索引器: GetChunks 获取向量失败", err, "doc_id", docID)
		return nil, fmt.Errorf("GetChunks: 获取文档 %s 的向量失败: %w", docID, err)
	}
	if len(vectors) == 0 {
		return []*core.Chunk{}, nil
	}

	chunks := make([]*core.Chunk, 0, len(vectors))
	for _, vec := range vectors {
		if vec == nil {
			continue
		}
		chunks = append(chunks, vectorToChunk(vec))
	}
	s.logger.Debug("语义索引器: GetChunks 完成",
		"doc_id", docID, "chunks", len(chunks))
	return chunks, nil
}

// Count 实现 IndexerAdmin 接口：返回已索引的 Chunk 总数。
//
// 多维度向量索引会为同一 Chunk 写入 Content/Title/Summary 三个向量，
// 从属向量（:title / :summary）仅服务于搜索召回，不属于独立 Chunk。
// 因此 Count 与 List 保持一致的口径：过滤掉从属维度向量，只统计主向量，
// 保证返回值与 Chunk 语义一致，避免偏高 2~3 倍。
func (s *semanticIndexer) Count(ctx context.Context) (int, error) {
	all, _, err := s.db.List(ctx, 0, 1<<30, nil)
	if err != nil {
		s.logger.Error("语义索引器: Count 查询失败", err)
		return 0, fmt.Errorf("Count: 查询失败: %w", err)
	}
	total := 0
	for _, vec := range all {
		if vec == nil {
			continue
		}
		if _, isDim := stripDimSuffix(vec.ChunkID, semanticDimensions); isDim {
			continue
		}
		total++
	}
	return total, nil
}

// Clear 实现 IndexerAdmin 接口：清空索引。
func (s *semanticIndexer) Clear(ctx context.Context) error {
	s.logger.Debug("语义索引器: 清空索引")
	if err := s.db.Clear(ctx); err != nil {
		s.logger.Error("语义索引器: 清空索引失败", err)
		return err
	}
	s.logger.Info("语义索引器: 索引已清空")
	return nil
}

// Close 实现 IndexerCloser 接口：释放底层向量存储资源。
func (s *semanticIndexer) Close(ctx context.Context) error {
	s.logger.Debug("语义索引器: 关闭存储")
	if err := s.db.Close(ctx); err != nil {
		s.logger.Error("语义索引器: 关闭存储失败", err)
		return err
	}
	s.logger.Debug("语义索引器: 存储已关闭")
	return nil
}

// Flush 实现 IndexerFlusher 接口：强制将写入缓冲刷入持久化存储。
func (s *semanticIndexer) Flush(ctx context.Context) error {
	s.logger.Debug("语义索引器: 刷入持久化存储")
	if err := s.db.Flush(ctx); err != nil {
		s.logger.Error("语义索引器: 刷入持久化存储失败", err)
		return err
	}
	s.logger.Debug("语义索引器: 存储已刷入磁盘")
	return nil
}
