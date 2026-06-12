package indexer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/DotNetAge/gochat/client/openai"
	chat "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gorag/chunker"
	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/document"
	"github.com/DotNetAge/gorag/logging"
	"github.com/DotNetAge/gorag/query"
	"github.com/DotNetAge/gorag/utils"
)

// minContentLength 是 LLM 索引的最小内容长度（按字符数，非 token）。
// 短于此长度的文本直接静默丢弃，避免浪费 token。
const minContentLength = 20

// LLMIndexer 使用 LLM 进行文本分块、实体提取，并同时写入 VectorStore + GraphStore。
// 与 semanticIndexer / graphIndexer 平级，是 GoRAG 的第三个索引器实现。
//
// 数据流：
//
//	Add / AddFile → document (获取 docID) → Token 估算
//	  → 未超限 → LLM (分块+实体提取) → 写入 vectorDB + graphDB
//	  → 超限 → 切片 → N 次 LLM 调用 → 合并结果 → 写入
type LLMIndexer struct {
	model           ModelConfig
	embedder        core.Embedder
	vectorDB        core.VectorStore
	graphDB         core.GraphStore
	lastUsage       *TokenUsage
	logger          logging.Logger
	ontologyPrompts []string // 来自 WithOntologyTech 的自定义 ontology 提示
}

// LLMOption configures an LLMIndexer.
type LLMOption func(*LLMIndexer)

// WithLLMLogger attaches a logger to the LLMIndexer for observation logs.
func WithLLMLogger(logger logging.Logger) LLMOption {
	return func(idx *LLMIndexer) {
		if logger != nil {
			idx.logger = logger
		}
	}
}

// WithOntologyTech 为 LLMIndexer 附加自定义 ontology 提示文本。
// 这些提示会追加在 ModelConfig.Ontology 预设内容之后，可用于补充特定领域的
// 实体/关系定义。多次调用会累积所有提示。
func WithOntologyTech(prompts ...string) LLMOption {
	return func(idx *LLMIndexer) {
		idx.ontologyPrompts = append(idx.ontologyPrompts, prompts...)
	}
}

// New 创建 LLMIndexer
//
//   - model:    LLM 模型连接配置（APIKey, BaseURL, Model, MaxTokens）
//   - embedder: 文本向量化引擎
//   - vectorDB: 向量存储（写入 Chunk 向量，用于语义检索）
//   - graphDB:  图存储（写入实体/关系，用于知识图谱检索）
//   - opts:     可选配置（WithLLMLogger 等）
func New(
	model ModelConfig,
	embedder core.Embedder,
	vectorDB core.VectorStore,
	graphDB core.GraphStore,
	opts ...LLMOption,
) *LLMIndexer {
	if model.MaxTokens <= 0 {
		model.MaxTokens = defaultMaxTokens
	}
	idx := &LLMIndexer{
		model:    model,
		embedder: embedder,
		vectorDB: vectorDB,
		graphDB:  graphDB,
		logger:   logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(idx)
	}
	return idx
}

// ---------------------------------------------------------------------------
// core.Indexer 接口实现
// ---------------------------------------------------------------------------

func (idx *LLMIndexer) Name() string { return "llm" }

func (idx *LLMIndexer) Type() string { return "llm" }

// Add 对一段文本执行 LLM 索引。
//
// 流程：document → Token 估算
//   - 未超限：单次 LLM 分块+实体提取 → 写入 vectorDB + graphDB
//   - 超限：按 80% maxTokens 切片 → 多次 LLM → 合并结果 → 写入
//
// 超短文本（< minContentLength 字符）会被静默丢弃。
func (idx *LLMIndexer) Add(ctx context.Context, content string) ([]*core.Chunk, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// 超短文本过滤
	if utf8.RuneCountInString(content) < minContentLength {
		idx.logger.Debug("content too short, skipped",
			"length", utf8.RuneCountInString(content),
			"min_length", minContentLength)
		return []*core.Chunk{}, nil
	}

	idx.logger.Info("indexing content",
		"length", utf8.RuneCountInString(content),
		"estimated_tokens", tokenEstimate(content))

	// 1. 通过 document.New 获取 docID
	mime := core.ParseMimeTypeFromText(content)
	doc := document.New(content, mime)
	docID := doc.GetID()

	// 2. Token 估算 → 切片或直接处理
	slices := idx.sliceContent(content)
	if len(slices) == 1 {
		idx.logger.Debug("single slice, calling LLM", "doc_id", docID)
		parsed, err := idx.llmIndex(ctx, docID, slices[0])
		if err != nil {
			return nil, err
		}
		return idx.writeToStores(ctx, docID, parsed)
	}

	// 多切片：逐片 LLM 处理
	idx.logger.Info("content exceeds limit, slicing",
		"slices", len(slices),
		"max_tokens", idx.model.MaxTokens)
	allResults := make([]*IndexData, 0, len(slices))
	for i, s := range slices {
		idx.logger.Debug("LLM call for slice", "slice", i+1, "total", len(slices))
		parsed, err := idx.llmIndex(ctx, docID, s)
		if err != nil {
			return nil, fmt.Errorf("slice indexing failed: %w", err)
		}
		allResults = append(allResults, parsed)
	}

	// 合并所有切片的结果后统一写入
	idx.logger.Debug("merging slice results", "slice_count", len(allResults))
	merged := mergeIndexData(allResults...)
	idx.logger.Debug("merge complete",
		"chunks", len(merged.Chunks),
		"entities", len(merged.Entities),
		"relations", len(merged.Relations))
	return idx.writeToStores(ctx, docID, merged)
}

// AddFile 从文件读取内容后执行 LLM 索引。
//
// 流程：document.Open（文档读取 + 清洗）→ Token 估算
//   - 未超限：单次 LLM → 写入
//   - 超限：返回错误，要求用户手动拆分文件
//
// 超短文件（< minContentLength 字符）会被静默丢弃。
func (idx *LLMIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path cannot be empty")
	}

	// 文件大小预检 — 避免 document.Open 无效 I/O
	if fi, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	} else if fi.Size() < int64(minContentLength) {
		idx.logger.Debug("file too small, skipped",
			"file", filePath,
			"size", fi.Size(),
			"min_length", minContentLength)
		return []*core.Chunk{}, nil
	}

	// 1. 通过 document.Open 打开并归一化文档内容
	doc, err := document.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	docID := doc.GetID()
	content := doc.GetContent()

	idx.logger.Info("indexing file",
		"file", filePath,
		"length", utf8.RuneCountInString(content),
		"estimated_tokens", tokenEstimate(content))

	// 超短文本过滤
	if utf8.RuneCountInString(content) < minContentLength {
		idx.logger.Debug("file content too short, skipped",
			"file", filePath,
			"length", utf8.RuneCountInString(content),
			"min_length", minContentLength)
		return []*core.Chunk{}, nil
	}

	// 2. Token 估算 — 超限直接失败
	safeLimit := int(float64(idx.model.MaxTokens) * inputTokenRatio * contentSafetyMargin)
	if tokenEstimate(content) > safeLimit {
		return nil, fmt.Errorf(
			"file content exceeds safe limit (~%d tokens > %d), please split manually",
			tokenEstimate(content), safeLimit,
		)
	}

	// 3. LLM 处理
	idx.logger.Debug("LLM call for file", "doc_id", docID, "file", filePath)
	parsed, err := idx.llmIndex(ctx, docID, content)
	if err != nil {
		return nil, err
	}

	return idx.writeToStores(ctx, docID, parsed)
}

// Search 执行向量检索（委托给 vectorDB）。
// 图检索的混合策略后续由 Query 对象设计时统一处理。
func (idx *LLMIndexer) Search(ctx context.Context, qry core.Query) ([]core.Hit, error) {
	sq, ok := qry.(*query.SemanticQuery)
	if !ok {
		return nil, fmt.Errorf("LLMIndexer.Search requires a *query.SemanticQuery")
	}

	queryVector := sq.Vector().Values

	results, scores, err := idx.vectorDB.Search(ctx, queryVector, 10, sq.Filters())
	if err != nil {
		return nil, err
	}

	hits := make([]core.Hit, 0, len(results))
	for i, vec := range results {
		hit := core.Hit{
			ID:    vec.ChunkID,
			Score: scores[i],
		}
		if vec.Metadata != nil {
			if c, ok := vec.Metadata["content"].(string); ok {
				hit.Content = c
			}
			if t, ok := vec.Metadata["title"].(string); ok {
				hit.Title = t
			}
			if d, ok := vec.Metadata["doc_id"].(string); ok {
				hit.DocID = d
			}
			hit.Metadata = extractMetadata(vec.Metadata)
			if cm, ok := vec.Metadata["chunk_meta"].(map[string]any); ok {
				hit.ChunkMeta = mapToChunkMeta(cm)
			}
		}
		hits = append(hits, hit)
	}
	return hits, nil
}

func (idx *LLMIndexer) NewQuery(terms string) core.Query {
	return query.NewSemanticQuery(terms, idx.embedder)
}

// Remove 从 vectorDB 中移除 chunk 向量。
// graphDB 中的关联节点/边不做级联删除（由 GraphIndexer 的 Cypher 语法处理）。
func (idx *LLMIndexer) Remove(ctx context.Context, chunkID string) error {
	return idx.vectorDB.Delete(ctx, chunkID)
}

// IndexChunk 索引单个预生成的 Chunk（实现 core.ChunkIndexer 接口）。
// 先调用 LLM 提取实体关系写入 graphDB，再向量化写入 vectorDB。
func (idx *LLMIndexer) IndexChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk cannot be nil")
	}

	// 1. graph: LLM 提取实体关系
	data, err := idx.llmIndex(ctx, chunk.DocID, chunk.Content)
	if err != nil {
		return fmt.Errorf("LLM extraction failed: %w", err)
	}
	if _, err := idx.writeToStores(ctx, chunk.DocID, data); err != nil {
		return fmt.Errorf("graph write failed: %w", err)
	}

	// 2. vector: 向量化并写入
	vec, err := idx.embedder.Calc(chunk)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}
	return idx.vectorDB.Upsert(ctx, []*core.Vector{vec})
}

// IndexChunks 批量索引预生成的 Chunk（实现 core.ChunkIndexer 接口）。
func (idx *LLMIndexer) IndexChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	for _, chunk := range chunks {
		if err := idx.IndexChunk(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// List 从 vectorDB 中分页获取结果。
func (idx *LLMIndexer) List(ctx context.Context, offset, limit int) ([]core.Hit, error) {
	vectors, err := idx.vectorDB.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	hits := make([]core.Hit, 0, len(vectors))
	for _, vec := range vectors {
		hits = append(hits, vectorToHit(vec))
	}
	return hits, nil
}

// GetChunks 根据 docID 从 vectorDB 中获取所有 Chunk。
func (idx *LLMIndexer) GetChunks(ctx context.Context, docID string) ([]*core.Chunk, error) {
	vectors, err := idx.vectorDB.GetByDocID(ctx, docID)
	if err != nil {
		return nil, fmt.Errorf("failed to get vectors by doc_id %s: %w", docID, err)
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
		if cm, ok := vec.Metadata["chunk_meta"].(map[string]any); ok {
			chunk.ChunkMeta = mapToChunkMeta(cm)
		}
		for k, v := range vec.Metadata {
			switch k {
			case "content", "doc_id", "parent_id", "mime_type", "chunk_meta":
			default:
				chunk.Metadata[k] = v
			}
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

// Count 返回 vectorDB 中的索引总数。
func (idx *LLMIndexer) Count(ctx context.Context) (int, error) {
	return idx.vectorDB.Count(ctx)
}

// Close 关闭底层存储。
func (idx *LLMIndexer) Close(ctx context.Context) error {
	if err := idx.vectorDB.Close(ctx); err != nil {
		return err
	}
	return idx.graphDB.Close(ctx)
}

// ---------------------------------------------------------------------------
// 扩展方法
// ---------------------------------------------------------------------------

// LastTokenUsage 返回最近一次 LLM 调用的 Token 用量。
func (idx *LLMIndexer) LastTokenUsage() *TokenUsage {
	return idx.lastUsage
}

// ---------------------------------------------------------------------------
// 内部：上下文防爆
// ---------------------------------------------------------------------------

// sliceContent 将超长内容按行切片，每行添加绝对行号前缀。
// 切片上限 = MaxTokens * inputTokenRatio * contentSafetyMargin。
// 返回单元素表示无需切片。
//
// 输出格式（行号帮助 LLM 准确定位 start_line / end_line）：
//
//	0: func main() {
//	1:     fmt.Println("hello")
//	2: }
func (idx *LLMIndexer) sliceContent(content string) []string {
	safeLimit := int(float64(idx.model.MaxTokens) * inputTokenRatio * contentSafetyMargin)
	if safeLimit <= 0 {
		return []string{numberedContent(strings.Split(content, "\n"), 0)}
	}

	if tokenEstimate(content) <= safeLimit {
		return []string{numberedContent(strings.Split(content, "\n"), 0)}
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return []string{""}
	}

	avgTokensPerLine := tokenEstimate(content) / len(lines)
	if avgTokensPerLine < 1 {
		avgTokensPerLine = 1
	}

	linesPerSlice := safeLimit / avgTokensPerLine
	if linesPerSlice <= 0 {
		linesPerSlice = 1
	}
	if linesPerSlice >= len(lines) {
		return []string{numberedContent(lines, 0)}
	}

	var slices []string
	for i := 0; i < len(lines); i += linesPerSlice {
		end := i + linesPerSlice
		if end > len(lines) {
			end = len(lines)
		}
		slices = append(slices, numberedContent(lines[i:end], i))
	}
	return slices
}

// numberedContent 为每一行添加绝对行号前缀。
//
//	numberedContent([]string{"func main() {", "}"}, 10)
//	=> "10: func main() {\n11: }"
func numberedContent(lines []string, startLine int) string {
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(fmt.Sprintf("%d: %s", startLine+i, line))
	}
	return b.String()
}

// tokenEstimate 估算文本的 token 数量。
// 使用 char/4 的粗略估算，配合 80% 安全边际足以防爆。
func tokenEstimate(text string) int {
	return len(text) / 4
}

// ---------------------------------------------------------------------------
// 内部：LLM 调用
// ---------------------------------------------------------------------------

// llmIndex 调用 LLM 进行文本分块 + 实体关系提取。
func (idx *LLMIndexer) llmIndex(ctx context.Context, docID, content string) (*IndexData, error) {
	idx.logger.Debug("LLM call starting",
		"doc_id", docID,
		"content_length", utf8.RuneCountInString(content),
		"model", idx.model.Model)

	client, err := openai.NewOpenAI(chat.Config{
		APIKey:  idx.model.APIKey,
		Model:   idx.model.Model,
		BaseURL: idx.model.BaseURL,
		Timeout: 5 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	lang := idx.model.Language
	if lang == "" {
		lang = "English"
	}

	messages := buildSystemMessages(docID, lang, idx.model.Ontology, idx.ontologyPrompts...)
	messages = append(messages, chat.NewUserMessage(content))

	resp, err := client.Chat(ctx, messages, chat.WithThinking(idx.model.ThinkingBudget))
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 记录 Token 用量
	if resp.Usage != nil {
		idx.lastUsage = &TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		idx.logger.Debug("LLM call completed",
			"doc_id", docID,
			"prompt_tokens", idx.lastUsage.PromptTokens,
			"completion_tokens", idx.lastUsage.CompletionTokens,
			"total_tokens", idx.lastUsage.TotalTokens)
	} else {
		idx.logger.Debug("LLM call completed (no usage info)", "doc_id", docID)
	}

	parsed, err := parseIndexData(resp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	return parsed, nil
}

// ---------------------------------------------------------------------------
// 内部：写入存储
// ---------------------------------------------------------------------------

// writeToStores 将 LLM 解析结果写入 vectorDB + graphDB，
// 同时构造 Chunk 列表返回。
//
// 处理顺序：
//  1. 扫描 Entities 构建 ordinal→NodeID 映射（所有后续步骤依赖此映射）
//  2. 处理 Chunks：将 entity_ids 中的序数解析为真实 NodeID
//  3. 批量写入 vectorDB
//  4. 批量写入 graphDB（Nodes + Edges）
func (idx *LLMIndexer) writeToStores(
	ctx context.Context, docID string, data *IndexData,
) ([]*core.Chunk, error) {
	idx.logger.Debug("writing to stores",
		"doc_id", docID,
		"chunks", len(data.Chunks),
		"entities", len(data.Entities),
		"relations", len(data.Relations))

	// ── 0. 构建 ordinal→NodeID 映射 ──────────────────────────────────
	ordinalToNodeID := make(map[int]string, len(data.Entities))
	entityNameByOrdinal := make(map[int]string, len(data.Entities))
	for _, e := range data.Entities {
		if e.Name == "" {
			continue
		}
		nodeID := utils.GenerateID([]byte(e.Name + docID))
		ordinalToNodeID[e.ID] = nodeID
		entityNameByOrdinal[e.ID] = e.Name
	}

	// ── 1. 构造 Chunk（entity_ids 解析为真实 NodeID）─────────────────
	chunks := make([]*core.Chunk, 0, len(data.Chunks))
	chunkVectors := make([]*core.Vector, 0, len(data.Chunks))

	for i, c := range data.Chunks {
		if c.Content == "" {
			continue
		}

		chunkID := chunker.GenerateChunkID(docID, i, c.Content)

		// 从 metadata 中提取 summary
		summary, _ := c.Metadata["summary"].(string)

		// 从 metadata 中提取 title
		title, _ := c.Metadata["title"].(string)

		// 从 metadata 中提取 tags
		tags, _ := c.Metadata["tags"].([]any)
		tagStrs := make([]string, 0, len(tags))
		for _, t := range tags {
			if s, ok := t.(string); ok {
				tagStrs = append(tagStrs, s)
			}
		}

		// 从 metadata 中提取 entity_ids 序数 → 解析为真实 NodeID
		entityIDs, _ := c.Metadata["entity_ids"].([]any)
		resolvedIDs := make([]string, 0, len(entityIDs))
		for _, id := range entityIDs {
			if ordinal, ok := id.(float64); ok {
				if nodeID, ok2 := ordinalToNodeID[int(ordinal)]; ok2 {
					resolvedIDs = append(resolvedIDs, nodeID)
				}
			}
		}

		chunk := &core.Chunk{
			ID:      chunkID,
			DocID:   docID,
			Title:   title,
			Content: c.Content,
			ChunkMeta: core.ChunkMeta{
				Index:        i,
				StartPos:     firstPos(c.ChunkMeta.Positions),
				EndPos:       lastPos(c.ChunkMeta.Positions),
				HeadingLevel: 0,
				HeadingPath:  []string{},
			},
			Metadata: map[string]any{
				"title":      title,
				"summary":    summary,
				"tags":       tagStrs,
				"entity_ids": resolvedIDs,
				"positions":  c.ChunkMeta.Positions,
			},
		}
		chunks = append(chunks, chunk)

		// 向量化
		vec, err := idx.embedder.CalcText(c.Content)
		if err != nil {
			return chunks, fmt.Errorf("embedding chunk %d failed: %w", i, err)
		}
		vec.ChunkID = chunkID
		vec.ID = utils.GenerateID([]byte("vec_" + chunkID))
		vec.Metadata = map[string]any{
			"doc_id":     docID,
			"title":      title,
			"content":    c.Content,
			"summary":    summary,
			"tags":       tagStrs,
			"positions":  c.ChunkMeta.Positions,
			"entity_ids": resolvedIDs,
			"chunk_meta": map[string]any{
				"index":         float64(i),
				"start_pos":     float64(firstPos(c.ChunkMeta.Positions)),
				"end_pos":       float64(lastPos(c.ChunkMeta.Positions)),
				"heading_level": float64(0),
				"heading_path":  []any{},
			},
		}
		chunkVectors = append(chunkVectors, vec)
	}

	// ── 2. 批量写入 vectorDB ───────────────────────────────────────────
	if len(chunkVectors) > 0 {
		if err := idx.vectorDB.Upsert(ctx, chunkVectors); err != nil {
			return chunks, fmt.Errorf("vectorDB upsert failed: %w", err)
		}
	}

	// ── 3. 收集 chunkID 列表（用于图节点的 source 绑定）─────────────────
	allChunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		allChunkIDs[i] = c.ID
	}

	// ── 4. 构造 Node ──────────────────────────────────────────────────
	nodes := make([]*core.Node, 0, len(data.Entities))
	for _, e := range data.Entities {
		if e.Name == "" {
			continue
		}
		nodeID := ordinalToNodeID[e.ID]
		desc, _ := e.Properties["description"].(string)
		nodes = append(nodes, &core.Node{
			ID:   nodeID,
			Type: e.Type,
			Name: e.Name,
			Properties: map[string]any{
				"description": desc,
				"confidence":  0.9,
			},
			SourceChunkIDs: allChunkIDs,
			SourceDocIDs:   []string{docID},
		})
	}
	if len(nodes) > 0 {
		if err := idx.graphDB.UpsertNodes(ctx, nodes); err != nil {
			return chunks, fmt.Errorf("graphDB upsert nodes failed: %w", err)
		}
	}

	// ── 5. 构造 Edge ──────────────────────────────────────────────────
	edges := make([]*core.Edge, 0, len(data.Relations))
	for _, r := range data.Relations {
		sourceName, ok := entityNameByOrdinal[r.Source]
		if !ok {
			continue
		}
		targetName, ok := entityNameByOrdinal[r.Target]
		if !ok {
			continue
		}
		sourceID := ordinalToNodeID[r.Source]
		targetID := ordinalToNodeID[r.Target]

		desc, _ := r.Properties["description"].(string)
		predicate := r.Predicate
		if predicate == "" {
			predicate = r.Type
		}

		edges = append(edges, &core.Edge{
			ID:        utils.GenerateID([]byte(sourceName + r.Type + targetName + docID)),
			Type:      r.Type,
			Source:    sourceID,
			Target:    targetID,
			Predicate: predicate,
			Properties: map[string]any{
				"description": desc,
				"confidence":  0.9,
			},
			SourceChunkIDs: allChunkIDs,
			SourceDocIDs:   []string{docID},
		})
	}
	if len(edges) > 0 {
		if err := idx.graphDB.UpsertEdges(ctx, edges); err != nil {
			return chunks, fmt.Errorf("graphDB upsert edges failed: %w", err)
		}
	}

	return chunks, nil
}

// firstPos 返回 positions 中第一个位置段的起始行号。
// 若无位置信息则返回 0。
func firstPos(positions [][2]int) int {
	if len(positions) == 0 {
		return 0
	}
	return positions[0][0]
}

// lastPos 返回 positions 中最后一个位置段的结束行号。
// 若无位置信息则返回 0。
func lastPos(positions [][2]int) int {
	if len(positions) == 0 {
		return 0
	}
	return positions[len(positions)-1][1]
}
