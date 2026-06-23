package indexer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	"gopkg.in/yaml.v3"
)

// minContentLength 是图索引的最小内容长度（按字符数，非 token）。
// 短于此长度的文本直接静默丢弃，避免浪费 token。
const minContentLength = 20

// IndexError 包含 LLM 索引失败的详细信息，传递给 OnFail 钩子。
type IndexError struct {
	DocID     string          // 文档 ID
	Err       error           // 原始错误
	ErrorType string          // 错误分类: network | timeout | rate_limit | auth | api | unknown
	Attempts  int             // 重试次数
	Duration  time.Duration   // 总耗时
	Messages  []chat.Message  // 请求消息快照（值传递，只读）
}

// classifyLLMError 对 LLM 调用错误进行分类。
func classifyLLMError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	errMsg := err.Error()
	if strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "rate_limit") ||
		strings.Contains(errMsg, "429") {
		return "rate_limit"
	}
	if strings.Contains(errMsg, "unauthorized") ||
		strings.Contains(errMsg, "401") ||
		strings.Contains(errMsg, "403") ||
		strings.Contains(errMsg, "authentication") {
		return "auth"
	}
	if strings.Contains(errMsg, "read tcp") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "i/o timeout") ||
		strings.Contains(errMsg, "TLS handshake") {
		return "network"
	}
	if strings.Contains(errMsg, "context deadline exceeded") ||
		strings.Contains(errMsg, "timeout") {
		return "timeout"
	}
	return "unknown"
}

// GraphIndexer 使用 LLM 进行文本分块、实体提取，并同时写入 VectorStore + GraphStore。
// 是 GoRAG 的全面索引器，同时写入 VectorStore + GraphStore。
//
// 数据流：
//
//	Add / AddFile → document (获取 docID) → Token 估算
//	  → 未超限 → LLM (分块+实体提取) → 写入 vectorDB + graphDB
//	  → 超限 → 切片 → N 次 LLM 调用 → 合并结果 → 写入
type GraphIndexer struct {
	model      ModelConfig
	embedder   core.Embedder
	vectorDB   core.VectorStore
	graphDB    core.GraphStore
	lastUsage        *TokenUsage // 最近一次 LLM 调用的 Token 用量
	cumulativeUsage  *TokenUsage // 从创建/重置起累积的 Token 用量，多切片场景使用
	mu               sync.Mutex
	logger           logging.Logger
	entityDefs       []EntityDef // 来自 WithSchemas 的自定义实体类型定义
	chatClient       chat.Client // 缓存的 LLM client，懒加载初始化后复用

	// ── 统计计数器（累积值，跨多次 Add/AddFile 调用） ──
	entitiesCreated int // 累计写入 graphDB 的实体数量
	relsCreated    int // 累计写入 graphDB 的关系数量
	statsMu        sync.Mutex

	// ── 钩子回调（只读观察者模式） ──
	OnRequest  func(docID string, messages []chat.Message, thinkingBudget int)
	OnResponse func(docID string, resp *chat.Response)
	OnFail     func(docID string, err *IndexError)
}

// getChatClient 返回缓存的 LLM client，首次调用时懒加载初始化。
func (idx *GraphIndexer) getChatClient() (chat.Client, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if idx.chatClient != nil {
		return idx.chatClient, nil
	}
	client, err := openai.NewOpenAI(chat.Config{
		APIKey:  idx.model.APIKey,
		Model:   idx.model.Model,
		BaseURL: idx.model.BaseURL,
		Timeout: 10 * time.Minute,
	})
	if err != nil {
		return nil, err
	}
	idx.chatClient = client
	return idx.chatClient, nil
}

// GraphOption configures a GraphIndexer.
type GraphOption func(*GraphIndexer)

// WithLogger attaches a logger to the GraphIndexer for observation logs.
func WithLogger(logger logging.Logger) GraphOption {
	return func(idx *GraphIndexer) {
		if logger != nil {
			idx.logger = logger
		}
	}
}

// WithSchemas 为 GraphIndexer 指定实体类型定义。
// 每项是一个 EntityDef（Prompt + Schema），会分别注入 Prompt 的 ### Entity Types
// 和 ### Entity Schema 段。多次调用会累积所有定义。
// 不调用该方法时，使用一组通用的默认实体类型。
func WithSchemas(entityDefs ...EntityDef) GraphOption {
	return func(idx *GraphIndexer) {
		idx.entityDefs = append(idx.entityDefs, entityDefs...)
	}
}

// entityTypeFile 定义 entities-*.yml 配置文件的结构。
type entityTypeFile struct {
	Domain string       `yaml:"domain"`
	Title  string       `yaml:"title"`
	Types  []entityType `yaml:"types"`
}

type entityType struct {
	Name   string `yaml:"name"`
	Title  string `yaml:"title"`
	Desc   string `yaml:"desc"`
	Prompt string `yaml:"prompt,omitempty"`
	Schema string `yaml:"schema,omitempty"`
}

// ParseEntityDefsYAML 解析实体类型定义的 YAML 数据，返回 EntityDef 列表。
// YAML 中每项支持两个输出字段：
//   - prompt：直接使用；为空时自动生成为 "**{Name}** — {Desc}"
//   - schema：可选字段，直接使用
func ParseEntityDefsYAML(data []byte) ([]EntityDef, error) {
	var f entityTypeFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse entity defs yaml: %w", err)
	}
	if len(f.Types) == 0 {
		return nil, nil
	}
	defs := make([]EntityDef, 0, len(f.Types))
	for _, t := range f.Types {
		if t.Name == "" {
			continue
		}
		prompt := t.Prompt
		if prompt == "" {
			prompt = "**" + t.Name + "** — " + t.Desc
		}
		defs = append(defs, EntityDef{Prompt: prompt, Schema: t.Schema})
	}
	return defs, nil
}

// WithSchemasFromFS 从文件系统（如 embed.FS）中读取匹配 glob 模式的实体类型配置文件，
// 解析后注入到 GraphIndexer。匹配多个文件时会合并所有实体类型定义。
//
// 用法：
//
//	//go:embed settings/entities-*.yml
//	var runtimeFS embed.FS
//
//	idx := New(cfg, embedder, vdb, gdb,
//	    WithSchemasFromFS(runtimeFS, "settings/entities-*.yml"),
//	)
func WithSchemasFromFS(fsys fs.FS, glob string) GraphOption {
	return func(idx *GraphIndexer) {
		matches, err := fs.Glob(fsys, glob)
		if err != nil {
			return
		}
		for _, match := range matches {
			data, err := fs.ReadFile(fsys, match)
			if err != nil {
				continue
			}
			defs, err := ParseEntityDefsYAML(data)
			if err != nil {
				continue
			}
			idx.entityDefs = append(idx.entityDefs, defs...)
		}
	}
}


// New 创建 GraphIndexer
//
//   - model:    LLM 模型连接配置（APIKey, BaseURL, Model, MaxTokens）
//   - embedder: 文本向量化引擎
//   - vectorDB: 向量存储（写入 Chunk 向量，用于语义检索）
//   - graphDB:  图存储（写入实体/关系，用于知识图谱检索）
//   - opts:     可选配置（WithLogger、WithSchemas 等）
func New(
	model ModelConfig,
	embedder core.Embedder,
	vectorDB core.VectorStore,
	graphDB core.GraphStore,
	opts ...GraphOption,
) *GraphIndexer {
	if model.MaxTokens <= 0 {
		model.MaxTokens = defaultMaxTokens
	}
	idx := &GraphIndexer{
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

func (idx *GraphIndexer) Name() string { return "graph" }

func (idx *GraphIndexer) Type() string { return "graph" }

// SetEntityDefs 运行时更新实体类型定义列表。
// 用于用户在界面上保存知识标签选择后，同步到正在运行的 GraphIndexer。
// 下次索引调用会使用新的实体定义。
func (idx *GraphIndexer) SetEntityDefs(defs []EntityDef) {
	idx.entityDefs = defs
}

// Add 对一段文本执行 LLM 索引。
//
// 流程：document → Token 估算
//   - 未超限：单次 LLM 分块+实体提取 → 写入 vectorDB + graphDB
//   - 超限：按 80% maxTokens 切片 → 多次 LLM → 合并结果 → 写入
//
// 超短文本（< minContentLength 字符）会被静默丢弃。
func (idx *GraphIndexer) Add(ctx context.Context, content string) ([]*core.Chunk, error) {
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

	// 2. 检测内容类型 → 选择 System Prompt（代码域 / 文本域）
	lang := idx.model.Language
	if lang == "" {
		lang = "English"
	}
	systemMsgs := buildSystemMessages(docID, lang, idx.entityDefs)
	if isCodeContent(content) {
		systemMsgs = buildCodeSystemMessages(docID, lang)
	}

	// 3. 分页：按行将内容拆为多页，每页不超过 MaxTokens × 80%
	pages, totalTokens, err := idx.splitIntoPages(content)
	if err != nil {
		return nil, err
	}

	// 4. 构建 messages：SystemMessage + 每页一条 UserMessage（末尾加 [Lx-Ly] 标记）
	messages := make([]chat.Message, 0, len(systemMsgs)+len(pages))
	messages = append(messages, systemMsgs...)
	for _, p := range pages {
		pageContent := p.content + fmt.Sprintf("\n[L%d-L%d]", p.startLine, p.endLine)
		messages = append(messages, chat.NewUserMessage(pageContent))
	}

	idx.logger.Info("sending multi-page request",
		"doc_id", docID,
		"pages", len(pages),
		"estimated_tokens", totalTokens)

	// 5. 单次 LLM 调用，LLM 一次看到所有页面，统一返回分块/实体/关系
	parsed, err := idx.llmIndex(ctx, docID, content, messages)
	if err != nil {
		return nil, err
	}
	return idx.writeToStores(ctx, docID, parsed, "", mime)
}

// AddFile 从文件读取内容后执行 LLM 索引。
//
// 流程：document.Open（文档读取 + 清洗）→ Token 估算
//   - 未超限：单次 LLM → 写入
//   - 超限：返回错误，要求用户手动拆分文件
//
// 超短文件（< minContentLength 字符）会被静默丢弃。
func (idx *GraphIndexer) AddFile(ctx context.Context, filePath string) ([]*core.Chunk, error) {
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

	// 2. 检测文件类型 → 选择 System Prompt（代码域 / 文本域）
	lang := idx.model.Language
	if lang == "" {
		lang = "English"
	}
	ext := strings.ToLower(filepath.Ext(filePath))
	systemMsgs := buildSystemMessages(docID, lang, idx.entityDefs)
	if isCodeExt(ext) {
		systemMsgs = buildCodeSystemMessages(docID, lang)
	}

	// 3. 分页：按行将内容拆为多页，每页不超过 MaxTokens × 80%
	pages, totalTokens, err := idx.splitIntoPages(content)
	if err != nil {
		return nil, err
	}

	// 4. 构建 messages：SystemMessage + 每页一条 UserMessage（末尾加 [Lx-Ly] 标记）
	messages := make([]chat.Message, 0, len(systemMsgs)+len(pages))
	messages = append(messages, systemMsgs...)
	for _, p := range pages {
		pageContent := p.content + fmt.Sprintf("\n[L%d-L%d]", p.startLine, p.endLine)
		messages = append(messages, chat.NewUserMessage(pageContent))
	}

	idx.logger.Info("sending multi-page request",
		"doc_id", docID,
		"pages", len(pages),
		"estimated_tokens", totalTokens)

	// 5. 单次 LLM 调用，LLM 一次看到所有页面，统一返回分块/实体/关系
	parsed, err := idx.llmIndex(ctx, docID, content, messages)
	if err != nil {
		return nil, err
	}
	return idx.writeToStores(ctx, docID, parsed, doc.GetSource(), doc.GetMimeType())
}

// Search 按查询类型路由搜索策略：
//
//   - *query.GraphQuery 含 RawCypher → graphDB 直接执行 Cypher → 转 Hits
//   - *query.GraphQuery 含 TextQuery（无 RawCypher）→ 内部 LLM 转 Cypher → graphDB 执行 → 转 Hits
//   - *query.GraphQuery 不含 Text/Raw → 向量检索 → Nodes/Edges 融合 → 多跳遍历 → Hits
//   - *query.SemanticQuery → 向量检索 → Nodes/Edges 融合 → 返回 Hits
//
// 无论哪种查询类型，返回的 Hits 均包含 Entities/Relations 字段。
func (idx *GraphIndexer) Search(ctx context.Context, qry core.Query) ([]core.Hit, error) {
	switch q := qry.(type) {
	case *query.GraphQuery:
		if raw := q.RawCypher(); raw != "" {
			return idx.searchCypher(ctx, raw, q.Limit)
		}
		if text := q.TextQuery(); text != "" {
			cypher, err := idx.text2Cypher(ctx, text)
			if err != nil {
				return nil, fmt.Errorf("text2cypher failed: %w", err)
			}
			return idx.searchCypher(ctx, cypher, q.Limit)
		}
		return idx.searchGraph(ctx, q)
	case *query.SemanticQuery:
		return idx.searchSemantic(ctx, q)
	default:
		return nil, fmt.Errorf("GraphIndexer.Search: unsupported query type %T", qry)
	}
}

// searchGraph GraphQuery 路由：向量检索 → Nodes/Edges 融合 → 多跳遍历 → Hits
func (idx *GraphIndexer) searchGraph(ctx context.Context, q *query.GraphQuery) ([]core.Hit, error) {
	// 1. 向量检索（由 GraphIndexer 的 embedder 计算向量）
	queryVector, err := idx.embedder.CalcText(q.Raw())
	if err != nil {
		return nil, fmt.Errorf("embedding failed: %w", err)
	}
	results, scores, err := idx.vectorDB.Search(ctx, queryVector.Values, q.Limit, q.Filters())
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	// 2. 收集 chunkIDs
	chunkIDs := make([]string, len(results))
	for i, r := range results {
		chunkIDs[i] = r.ChunkID
	}

	// 3. Nodes/Edges 融合 + 多跳遍历
	return idx.enrichHits(ctx, results, scores, chunkIDs, q.Depth, q.EdgeTypes)
}

// searchSemantic SemanticQuery 路由：向量检索 → Nodes/Edges 融合 → Hits
func (idx *GraphIndexer) searchSemantic(ctx context.Context, q *query.SemanticQuery) ([]core.Hit, error) {
	queryVector := q.Vector().Values
	results, scores, err := idx.vectorDB.Search(ctx, queryVector, 10, q.Filters())
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	// 收集 chunkIDs 用于图融合（SemanticQuery 不做多跳，depth=1 即仅直接关联）
	chunkIDs := make([]string, len(results))
	for i, r := range results {
		chunkIDs[i] = r.ChunkID
	}
	return idx.enrichHits(ctx, results, scores, chunkIDs, 1, nil)
}

// searchCypher 将 Cypher 查询交给 graphDB 执行，结果转成 Hits。
// 每行结果作为一个 Hit，Content 为 key=value 格式的文本描述，便于 LLM 或用户阅读。
func (idx *GraphIndexer) searchCypher(ctx context.Context, cypher string, limit int) ([]core.Hit, error) {
	rows, err := idx.graphDB.Query(ctx, cypher, nil)
	if err != nil {
		return nil, fmt.Errorf("cypher query failed: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	// 限制返回行数
	if limit <= 0 {
		limit = 10
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}

	hits := make([]core.Hit, 0, len(rows))
	for i, row := range rows {
		hit := cypherRowToHit(row, float32(1.0-float64(i)/float64(len(rows))))
		hits = append(hits, hit)
	}
	return hits, nil
}

// cypherRowToHit 将 Cypher 查询结果的单行 map 转换为 Hit。
// Content 为 key=value 格式文本，Score 按排名递减。
func cypherRowToHit(row map[string]any, score float32) core.Hit {
	hit := core.Hit{
		ID:    fmt.Sprintf("cypher-%d", time.Now().UnixNano()),
		Score: score,
	}

	var parts []string
	for k, v := range row {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	hit.Content = strings.Join(parts, ", ")

	// 尝试从行中提取结构化数据
	if id, ok := row["id"].(string); ok {
		hit.ID = id
	}
	if name, ok := row["name"].(string); ok {
		hit.Title = name
	}
	return hit
}

// enrichHits 对向量检索结果执行 Nodes/Edges 融合，返回带 Entities/Relations 的 Hits。
// depth=1 仅查询直接关联的 Nodes/Edges，depth>1 执行多跳遍历。
func (idx *GraphIndexer) enrichHits(
	ctx context.Context,
	results []*core.Vector,
	scores []float32,
	chunkIDs []string,
	depth int,
	edgeTypes []string,
) ([]core.Hit, error) {
	// 1. 查询关联的 Nodes
	nodes, err := idx.graphDB.GetNodesByChunkIDs(ctx, chunkIDs)
	if err != nil {
		// graphDB 不可用时降级为纯向量结果
		return idx.toSimpleHits(results, scores), nil
	}
	if len(nodes) == 0 {
		return idx.toSimpleHits(results, scores), nil
	}

	// 2. 收集起始节点 ID
	nodeIDs := make([]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}

	// 3. 多跳遍历（depth>1 时）
	hopNodes, hopEdges := []*core.Node{}, []*core.Edge{}
	if depth > 1 {
		hopNodes, hopEdges, err = idx.graphDB.GetMultiHopPaths(ctx, nodeIDs, edgeTypes, depth, 10)
		if err != nil {
			// 多跳失败时降级
			hopEdges, err = idx.graphDB.GetEdgesByChunkIDs(ctx, chunkIDs)
			if err != nil {
				hopEdges = nil
			}
		}
	} else {
		// depth=1 仅查询直接关联边
		hopEdges, err = idx.graphDB.GetEdgesByChunkIDs(ctx, chunkIDs)
		if err != nil {
			hopEdges = nil
		}
	}

	// 4. 合并所有 Nodes 和 Edges
	edgeMap := make(map[string]*core.Edge)
	for _, e := range hopEdges {
		edgeMap[e.ID] = e
	}
	nodeMap := make(map[string]*core.Node)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}
	for _, n := range hopNodes {
		if _, exists := nodeMap[n.ID]; !exists {
			nodeMap[n.ID] = n
		}
	}

	allNodes := make([]*core.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		allNodes = append(allNodes, n)
	}
	allEdges := make([]*core.Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		allEdges = append(allEdges, e)
	}

	// 5. 构建 Hits：每个向量结果按 chunkID 关联对应的 Nodes/Edges
	hits := make([]core.Hit, 0, len(results))
	seenChunk := make(map[string]bool)

	for i, vec := range results {
		if seenChunk[vec.ChunkID] {
			continue
		}
		seenChunk[vec.ChunkID] = true

		hit := idx.vectorToHit(vec, scores[i])
		hit.Entities = idx.nodesForChunk(allNodes, vec.ChunkID)
		hit.Relations = idx.edgesForChunk(allEdges, vec.ChunkID)
		hits = append(hits, hit)
	}

	return hits, nil
}

// toSimpleHits 从向量检索结果构建基础 Hits（无图融合的降级路径）。
func (idx *GraphIndexer) toSimpleHits(results []*core.Vector, scores []float32) []core.Hit {
	hits := make([]core.Hit, 0, len(results))
	for i, vec := range results {
		hit := vectorToHit(vec)
		hit.Score = scores[i]
		hits = append(hits, hit)
	}
	return hits
}

// vectorToHit 从单个向量结果构建 Hit，携带分数。
func (idx *GraphIndexer) vectorToHit(vec *core.Vector, score float32) core.Hit {
	return core.Hit{
		ID:    vec.ChunkID,
		Score: score,
		Metadata: func() map[string]any {
			m := make(map[string]any, len(vec.Metadata)+1)
			m["chunk_id"] = vec.ChunkID
			for k, v := range vec.Metadata {
				m[k] = v
			}
			return m
		}(),
	}
}

// nodesForChunk 从节点列表中筛选属于指定 chunk 的节点。
func (idx *GraphIndexer) nodesForChunk(nodes []*core.Node, chunkID string) []*core.Node {
	var result []*core.Node
	for _, n := range nodes {
		for _, cid := range n.SourceChunkIDs {
			if cid == chunkID {
				result = append(result, n)
				break
			}
		}
	}
	return result
}

// edgesForChunk 从边列表中筛选属于指定 chunk 的边。
func (idx *GraphIndexer) edgesForChunk(edges []*core.Edge, chunkID string) []*core.Edge {
	var result []*core.Edge
	for _, e := range edges {
		for _, cid := range e.SourceChunkIDs {
			if cid == chunkID {
				result = append(result, e)
				break
			}
		}
	}
	return result
}

// SearchByChunkIDs 通过 Chunk IDs 查询关联的图结构（外部调用入口，无需向量检索）。
// 流程：Chunk IDs → 查询关联 Nodes → 多跳遍历 → 路径评分 → Hit
//
// 支持选项：
//   - depth: 遍历深度（默认 1，即直接邻居）
//   - limit: 返回结果数量上限
//   - edgeTypes: 关系类型过滤，仅遍历指定类型的边
//
// 注意：此方法不执行向量检索，直接使用提供的 chunkIDs。前端分页列表等场景可用。
func (idx *GraphIndexer) SearchByChunkIDs(ctx context.Context, chunkIDs []string, depth, limit int, edgeTypes ...[]string) ([]core.Hit, error) {
	if len(chunkIDs) == 0 {
		return nil, nil
	}

	// 直接查询 graphDB，无向量检索
	nodes, err := idx.graphDB.GetNodesByChunkIDs(ctx, chunkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes by chunk IDs: %w", err)
	}
	if len(nodes) == 0 {
		return nil, nil
	}

	nodeIDs := make([]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}

	var types []string
	if len(edgeTypes) > 0 && len(edgeTypes[0]) > 0 {
		types = edgeTypes[0]
	}

	hopNodes, hopEdges := []*core.Node{}, []*core.Edge{}
	if depth > 0 {
		hopNodes, hopEdges, err = idx.graphDB.GetMultiHopPaths(ctx, nodeIDs, types, depth, limit)
		if err != nil {
			hopEdges, err = idx.graphDB.GetEdgesByChunkIDs(ctx, chunkIDs)
			if err != nil {
				hopEdges = nil
			}
		}
	}

	edgeMap := make(map[string]*core.Edge)
	for _, e := range hopEdges {
		edgeMap[e.ID] = e
	}
	if depth <= 0 {
		directEdges, err := idx.graphDB.GetEdgesByChunkIDs(ctx, chunkIDs)
		if err == nil {
			for _, e := range directEdges {
				if _, exists := edgeMap[e.ID]; !exists {
					edgeMap[e.ID] = e
				}
			}
		}
	}

	nodeMap := make(map[string]*core.Node)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}
	for _, n := range hopNodes {
		if _, exists := nodeMap[n.ID]; !exists {
			nodeMap[n.ID] = n
		}
	}

	allNodes := make([]*core.Node, 0, len(nodeMap))
	for _, n := range nodeMap {
		allNodes = append(allNodes, n)
	}
	allEdges := make([]*core.Edge, 0, len(edgeMap))
	for _, e := range edgeMap {
		allEdges = append(allEdges, e)
	}

	// 收集所有 Chunk IDs
	allChunkIDs := make([]string, 0)
	{
		seen := make(map[string]bool)
		for _, node := range allNodes {
			for _, cid := range node.SourceChunkIDs {
				if !seen[cid] {
					allChunkIDs = append(allChunkIDs, cid)
					seen[cid] = true
				}
			}
		}
	}

	// 构建 Hit
	hits := make([]core.Hit, 0, len(allChunkIDs))
	seenChunkHit := make(map[string]bool)

	for _, chunkID := range allChunkIDs {
		if seenChunkHit[chunkID] {
			continue
		}
		seenChunkHit[chunkID] = true

		entities := idx.nodesForChunk(allNodes, chunkID)
		relations := idx.edgesForChunk(allEdges, chunkID)
		score := scoreGraphResult(entities, relations)

		hits = append(hits, core.Hit{
			ID:        chunkID,
			Score:     score,
			Entities:  entities,
			Relations: relations,
		})
	}

	return hits, nil
}

func (idx *GraphIndexer) NewQuery(terms string) core.Query {
	return query.NewGraphQuery(terms)
}

// Remove 从 vectorDB 和 graphDB 中移除与 chunkID 关联的所有数据。
func (idx *GraphIndexer) Remove(ctx context.Context, chunkID string) error {
	// 从 vectorDB 移除 chunk 向量
	if err := idx.vectorDB.Delete(ctx, chunkID); err != nil {
		return err
	}
	// 从 graphDB 移除关联的节点和边（级联删除）
	if idx.graphDB != nil {
		q := `MATCH (n) WHERE $chunkID IN n.source_chunk_ids DETACH DELETE n`
		_, err := idx.graphDB.Query(ctx, q, map[string]any{"chunkID": chunkID})
		if err != nil {
			idx.logger.Warn("graphDB cleanup failed during Remove", "chunkID", chunkID, "error", err)
		}
	}
	return nil
}

// StoreChunk stores a pre-built chunk directly in the index, skipping LLM processing
// and entity extraction. The chunk's Metadata is persisted as vector metadata
// for filter-based retrieval. This is used by the memory system to store raw memory data
// without running through the LLM chunking/entity pipeline.
func (idx *GraphIndexer) StoreChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil || chunk.Content == "" {
		return fmt.Errorf("chunk content cannot be empty")
	}
	return idx.saveChunk(ctx, chunk)
}

// saveChunk 索引单个预生成的 Chunk。
// IndexChunks 是"分块后的处理入口"，不做 LLM 调用，只做向量化 + 存储。
//
// LLM 分块 + 实体提取在 Add 路径中完成。IndexChunk/IndexChunks 由 Add 内调，
// 或由 HybridIndexer 在合并分发预分块内容时调用。
func (idx *GraphIndexer) saveChunk(ctx context.Context, chunk *core.Chunk) error {
	if chunk == nil {
		return fmt.Errorf("chunk cannot be nil")
	}

	vec, err := idx.embedder.Calc(chunk)
	if err != nil {
		return fmt.Errorf("embedding failed: %w", err)
	}
	return idx.vectorDB.Upsert(ctx, []*core.Vector{vec})
}

// IndexChunks 批量索引预生成的 Chunk（实现 core.ChunkIndexer 接口）。
func (idx *GraphIndexer) saveChunks(ctx context.Context, chunks []*core.Chunk) error {
	if len(chunks) == 0 {
		return nil
	}

	for _, chunk := range chunks {
		if err := idx.saveChunk(ctx, chunk); err != nil {
			return err
		}
	}
	return nil
}

// List 从 vectorDB 中分页获取结果。
func (idx *GraphIndexer) List(ctx context.Context, offset, limit int) ([]core.Hit, error) {
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
func (idx *GraphIndexer) GetChunks(ctx context.Context, docID string) ([]*core.Chunk, error) {
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
func (idx *GraphIndexer) Count(ctx context.Context) (int, error) {
	return idx.vectorDB.Count(ctx)
}

// Close 关闭底层存储。
func (idx *GraphIndexer) Close(ctx context.Context) error {
	if err := idx.vectorDB.Close(ctx); err != nil {
		return err
	}
	return idx.graphDB.Close(ctx)
}

// ---------------------------------------------------------------------------
// 扩展方法
// ---------------------------------------------------------------------------

// CypherQuery 执行原始的 Cypher 查询，供外部 Agent/LLM 生成高级图查询。
// 参数 params 为 Cypher 查询的命名参数映射。
func (idx *GraphIndexer) CypherQuery(ctx context.Context, q string, params map[string]any) ([]map[string]any, error) {
	if idx.graphDB == nil {
		return nil, fmt.Errorf("graphDB not available")
	}
	return idx.graphDB.Query(ctx, q, params)
}

// VectorDB 返回 GraphIndexer 持有的向量数据库实例。
// 外部可通过此方法直接操作向量存储（如批量删除、统计等）。
func (idx *GraphIndexer) VectorDB() core.VectorStore {
	return idx.vectorDB
}

// GraphDB 返回 GraphIndexer 持有的图数据库实例。
// 外部可通过此方法直接操作图存储（如自定义 Cypher 查询、图分析等）。
func (idx *GraphIndexer) GraphDB() core.GraphStore {
	return idx.graphDB
}

// LastTokenUsage 返回最近一次 LLM 调用的 Token 用量（单次值）。
func (idx *GraphIndexer) LastTokenUsage() *TokenUsage {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.lastUsage
}

// CumulativeTokenUsage 返回从创建/重置起累积的 Token 用量（多切片场景使用）。
func (idx *GraphIndexer) CumulativeTokenUsage() *TokenUsage {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.cumulativeUsage
}

// EntityStats 返回自上次 ResetEntityStats 以来累计创建的实体和关系数量。
func (idx *GraphIndexer) EntityStats() (entities, rels int) {
	idx.statsMu.Lock()
	defer idx.statsMu.Unlock()
	return idx.entitiesCreated, idx.relsCreated
}

// ResetEntityStats 将实体/关系计数器归零（通常在每次 Sync 开始前调用）。
func (idx *GraphIndexer) ResetEntityStats() {
	idx.statsMu.Lock()
	defer idx.statsMu.Unlock()
	idx.entitiesCreated = 0
	idx.relsCreated = 0
}

// ---------------------------------------------------------------------------
// 内部：分页与上下文防爆
// ---------------------------------------------------------------------------

// pageInfo 表示一页内容及其在原文件中的行号范围。
type pageInfo struct {
	startLine int
	endLine   int
	content   string
}

// splitIntoPages 将内容按行分页，每页不超过 MaxTokens × 80%。
// 总内容超过 ContextLength × 80% 时返回错误。
//
// 调用方应将每页构建为单独的一条 UserMessage，末尾加 [L{start}-L{end}] 标记。
// 所有 UserMessage 在同一个 LLM 请求中发送，LLM 一次推理即可完成全部索引。
func (idx *GraphIndexer) splitIntoPages(content string) ([]pageInfo, int, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, 0, nil
	}

	totalTokens := tokenEstimate(content)
	totalLines := len(lines)

	// 总内容阈值检查：不超过 ContextLength × 80%
	contextLen := idx.model.ContextLength
	if contextLen <= 0 {
		contextLen = defaultMaxTokens
	}
	maxTotalTokens := int(float64(contextLen) * 0.8)
	if totalTokens > maxTotalTokens {
		return nil, totalTokens, fmt.Errorf(
			"content too large: estimated %d tokens exceeds 80%% of context length (%d)",
			totalTokens, maxTotalTokens)
	}

	// 每页预算：MaxTokens × 80%
	maxPageTokens := int(float64(idx.model.MaxTokens) * 0.8)
	if maxPageTokens <= 0 {
		maxPageTokens = defaultMaxTokens
	}

	avgTokensPerLine := totalTokens / totalLines
	if avgTokensPerLine < 1 {
		avgTokensPerLine = 1
	}

	linesPerPage := maxPageTokens / avgTokensPerLine
	if linesPerPage <= 0 {
		linesPerPage = 1
	}

	var pages []pageInfo
	for i := 0; i < totalLines; i += linesPerPage {
		end := i + linesPerPage
		if end > totalLines {
			end = totalLines
		}
		pages = append(pages, pageInfo{
			startLine: i,
			endLine:   end - 1,
			content:   strings.Join(lines[i:end], "\n"),
		})
	}
	return pages, totalTokens, nil
}

// tokenEstimate 估算文本的 token 数量。
// 使用 char/4 的粗略估算，配合 80% 安全边际足以防爆。
func tokenEstimate(text string) int {
	return utf8.RuneCountInString(text) / 2
}

// ---------------------------------------------------------------------------
// 内部：LLM 调用
// ---------------------------------------------------------------------------

// llmIndex 调用 LLM 进行文本分块 + 实体关系提取。
// messages 应由调用方预先构建（含 SystemMessage + 多条 UserMessage 分页）。
// fullContent 为原始全文，用于 LLM 响应解析失败时的兜底。
// 内置重试机制：最多重试 2 次（首次失败后间隔 2s、4s 指数退避）。
func (idx *GraphIndexer) llmIndex(ctx context.Context, docID, fullContent string, messages []chat.Message) (*IndexData, error) {
	idx.logger.Debug("LLM call starting",
		"doc_id", docID,
		"num_messages", len(messages),
		"model", idx.model.Model)

	client, err := idx.getChatClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM client: %w", err)
	}

	// ── OnRequest 钩子（只读，值传递） ──
	if idx.OnRequest != nil {
		msgs := make([]chat.Message, len(messages))
		copy(msgs, messages)
		idx.OnRequest(docID, msgs, idx.model.ThinkingBudget)
	}

	start := time.Now()
	var resp *chat.Response
	var lastErr error
	var attempt int
	for attempt = 0; attempt <= 2; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*2) * time.Second
			idx.logger.Warn("retrying LLM call", "attempt", attempt, "backoff", backoff, "error", lastErr)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		resp, lastErr = client.Chat(ctx, messages, chat.WithThinking(idx.model.ThinkingBudget))
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		// ── OnFail 钩子（所有重试均失败） ──
		if idx.OnFail != nil {
			msgs := make([]chat.Message, len(messages))
			copy(msgs, messages)
			idx.OnFail(docID, &IndexError{
				DocID:     docID,
				Err:       lastErr,
				ErrorType: classifyLLMError(lastErr),
				Attempts:  attempt,
				Duration:  time.Since(start),
				Messages:  msgs,
			})
		}
		return nil, fmt.Errorf("LLM call failed after 3 attempts: %w", lastErr)
	}

	// ── OnResponse 钩子（LLM 调用成功） ──
	if idx.OnResponse != nil {
		idx.OnResponse(docID, resp)
	}

	// 记录 Token 用量
	if resp.Usage != nil {
		idx.mu.Lock()
		// lastUsage：最近一次调用的数据（覆盖）
		idx.lastUsage = &TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
		// cumulativeUsage：累积（多切片路径使用）
		if idx.cumulativeUsage == nil {
			idx.cumulativeUsage = &TokenUsage{}
		}
		idx.cumulativeUsage.PromptTokens += resp.Usage.PromptTokens
		idx.cumulativeUsage.CompletionTokens += resp.Usage.CompletionTokens
		idx.cumulativeUsage.TotalTokens += resp.Usage.TotalTokens
		idx.mu.Unlock()
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
		// 降级：JSON 解析失败时尝试修复典型错误，再失败则兜底为单一全文 chunk
		idx.logger.Warn("LLM response parse failed, falling back to single-chunk index", "error", err)
		parsed = &IndexData{
			Chunks: []struct {
				Content   string         `json:"content"`
				Metadata  map[string]any `json:"metadata,omitempty"`
				ChunkMeta struct {
					Positions [][2]int `json:"positions"`
				} `json:"chunk_meta,omitempty"`
			}{
				{
					Content: fullContent,
					Metadata: map[string]any{
						"title":      "content",
						"summary":    "",
						"entity_ids": []any{},
					},
					ChunkMeta: struct {
					Positions [][2]int `json:"positions"`
				}{
					Positions: [][2]int{{0, 0}},
				},
				},
			},
		}
	}

	return parsed, nil
}

// text2Cypher 使用 GraphIndexer 内部的 LLM 将自然语言查询转换为 Cypher 语句。
// 复用与 llmIndex 相同的模型配置和客户端创建方式。
func (idx *GraphIndexer) text2Cypher(ctx context.Context, text string) (string, error) {
	client, err := idx.getChatClient()
	if err != nil {
		return "", fmt.Errorf("failed to create LLM client: %w", err)
	}

	prompt := fmt.Sprintf(`You are a Cypher query generation expert for gograph, an embedded Go graph database.

## Node Data Model

Each node has a label matching its entity category (PascalCase). Query by label using MATCH (n:LabelName).
Access node properties uniformly via n.propertyName syntax.

  n.ID                -- unique identifier (string)
  n.name              -- entity name (e.g. "Zhang San", "Alibaba")
  n.source_chunk_ids  -- []string, IDs of source chunks that mention this entity
  n.source_doc_ids    -- []string, IDs of source documents
  n.confidence        -- float (optional), extraction confidence
  n.frequency         -- int (optional), occurrence count
  n.*                 -- any custom property from dynamic schema

Entity category labels: Person, Organization, Location, Technology, Product, Event, Entity

To query by type:   MATCH (n:Person) RETURN n
To query by name:   MATCH (n) WHERE n.name = $name RETURN n

## Edge (Relationship) Data Model

  r.ID                -- unique identifier (string)
  r.type              -- relationship type, e.g. 'KNOWS', 'WORKS_FOR', 'LOCATED_IN', 'BELONGS_TO', 'RELATED_TO'
  r.predicate         -- human-readable description (e.g. "works at", "located in")
  r.source_chunk_ids  -- []string
  r.source_doc_ids    -- []string
  r.confidence        -- float (optional)
  r.score             -- float (optional)
  r.evidence          -- string (optional), text evidence
  r.*                 -- any custom property

Access edge fields uniformly via r.propertyName.

## RETURN Result Shape

RETURN n gives: {id, labels: ["Person"], properties: {ID, name, source_chunk_ids, ...}}
RETURN r gives: {id, type, startNodeID, endNodeID, properties: {ID, predicate, ...}}

## Cypher Syntax Reference

  MATCH (n:Person) RETURN n                                          -- filter by label
  MATCH (n) WHERE n.name = $name RETURN n                           -- parameterized filter
  MATCH (n:Person {name: 'Zhang San'}) RETURN n                     -- label + property shorthand
  MATCH (a:Person)-[r:KNOWS]->(b:Person) RETURN a, r, b             -- relationship traversal
  MATCH (n) WHERE $cid IN n.source_chunk_ids RETURN n               -- array contains
  RETURN n.ID, n.name, n.source_chunk_ids                            -- specific fields
  ORDER BY n.name SKIP 10 LIMIT 20                                   -- pagination
  MATCH (n {ID: $id}) DETACH DELETE n                                -- delete

## Instructions

Convert the following natural language query into a valid Cypher query.

Rules:
1. Node entity category is a LABEL, not a property -- use MATCH (n:Person), never WHERE n.type = 'Person'
2. Entity names are in property n.name -- use WHERE n.name = $name or (n {name: 'Zhang San'})
3. Relationship queries use (source)-[r:TYPE]->(target) patterns
4. RETURN both nodes and relationships when relevant, e.g. RETURN a, r, b
5. Use LIMIT 20 to control result size
6. Use parameterized queries ($name, $id) when filtering by specific values
7. Output ONLY the Cypher query, no explanation, no markdown code blocks

## User Query
%s

Output the Cypher query directly:`, text)

	messages := []chat.Message{
		chat.NewSystemMessage(prompt),
	}

	var resp *chat.Response
	var lastErr error
	for attempt := 0; attempt <= 2; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*2) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		resp, lastErr = client.Chat(ctx, messages)
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("text2cypher LLM call failed after 3 attempts: %w", lastErr)
	}

	// 清理响应：移除可能的 markdown 代码块标记
	cypher := strings.TrimSpace(resp.Content)
	cypher = strings.TrimPrefix(cypher, "```cypher")
	cypher = strings.TrimPrefix(cypher, "```")
	cypher = strings.TrimSuffix(cypher, "```")
	cypher = strings.TrimSpace(cypher)

	if cypher == "" {
		return "", fmt.Errorf("LLM returned empty Cypher query")
	}

	return cypher, nil
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
//
// sourceFile 传入原始文件路径（AddFile 路径）或空字符串（Add 路径）。
// mimeType 传入文档的 MIME 类型。
func (idx *GraphIndexer) writeToStores(
	ctx context.Context, docID string, data *IndexData, sourceFile, mimeType string,
) ([]*core.Chunk, error) {
	idx.logger.Debug("writing to stores",
		"doc_id", docID,
		"chunks", len(data.Chunks),
		"entities", len(data.Entities),
		"relations", len(data.Relations),
		"source_file", sourceFile)

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
	entityToChunks := make(map[string][]string) // entity NodeID → chunkIDs

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

		// 建立 entity→chunk 的逆向映射（用于图节点/边的 source 绑定）
		for _, nodeID := range resolvedIDs {
			entityToChunks[nodeID] = append(entityToChunks[nodeID], chunkID)
		}

		chunk := &core.Chunk{
			ID:      chunkID,
			ParentID:"",
			DocID:   docID,
			MIMEType: mimeType,
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
				"source_file": sourceFile,
				"parent_id":   "",
				"mime_type":   mimeType,
				"title":       title,
				"summary":     summary,
				"tags":        tagStrs,
				"entity_ids":  resolvedIDs,
				"positions":   c.ChunkMeta.Positions,
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
			"doc_id":      docID,
			"source_file": sourceFile,
			"parent_id":   "",
			"mime_type":   mimeType,
			"title":       title,
			"content":     c.Content,
			"summary":     summary,
			"tags":        tagStrs,
			"positions":   c.ChunkMeta.Positions,
			"entity_ids":  resolvedIDs,
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

	// ── 4. 构造 Node ──────────────────────────────────────────────────
	// allChunkIDs 作为 fallback（极少情况下实体未出现在任何 chunk 的 entity_ids 中）
	allChunkIDs := make([]string, len(chunks))
	for i, c := range chunks {
		allChunkIDs[i] = c.ID
	}

	nodes := make([]*core.Node, 0, len(data.Entities))
	for _, e := range data.Entities {
		if e.Name == "" {
			continue
		}
		nodeID := ordinalToNodeID[e.ID]
		srcChunks := entityToChunks[nodeID]
		if srcChunks == nil {
			srcChunks = allChunkIDs
		}

		// 保留 LLM 输出的全部结构化属性（如 code 模式下的 methods/fields/extends/generics 等），
		// 在顶层补充固定的 confidence 标注。
		props := make(map[string]any, len(e.Properties)+1)
		for k, v := range e.Properties {
			props[k] = v
		}
		props["confidence"] = 0.9

		nodes = append(nodes, &core.Node{
			ID:     nodeID,
			Labels: []string{e.Type},
			Name:   e.Name,
			Properties:    props,
			SourceChunkIDs: srcChunks,
			SourceDocIDs:   []string{docID},
		})
	}
	if len(nodes) > 0 {
		if err := idx.graphDB.UpsertNodes(ctx, nodes); err != nil {
			return chunks, fmt.Errorf("graphDB upsert nodes failed: %w", err)
		}
		idx.statsMu.Lock()
		idx.entitiesCreated += len(nodes)
		idx.statsMu.Unlock()
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

		predicate := r.Predicate
		if predicate == "" {
			predicate = r.Type
		}

		edgeSrcChunks := uniqueMerge(entityToChunks[sourceID], entityToChunks[targetID])

		// 保留 LLM 输出的全部关系属性，在顶层补充 confidence。
		eProps := make(map[string]any, len(r.Properties)+1)
		for k, v := range r.Properties {
			eProps[k] = v
		}
		eProps["confidence"] = 0.9

		edges = append(edges, &core.Edge{
			ID:        utils.GenerateID([]byte(sourceName + r.Type + targetName + docID)),
			Type:      r.Type,
			Source:    sourceID,
			Target:    targetID,
			Predicate: predicate,
			Properties:    eProps,
			SourceChunkIDs: edgeSrcChunks,
			SourceDocIDs:   []string{docID},
		})
	}
	if len(edges) > 0 {
		if err := idx.graphDB.UpsertEdges(ctx, edges); err != nil {
			return chunks, fmt.Errorf("graphDB upsert edges failed: %w", err)
		}
		idx.statsMu.Lock()
		idx.relsCreated += len(edges)
		idx.statsMu.Unlock()
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

// uniqueMerge 合并多个字符串切片并去重。
func uniqueMerge(slices ...[]string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, slice := range slices {
		for _, s := range slice {
			if _, ok := seen[s]; !ok {
				seen[s] = struct{}{}
				result = append(result, s)
			}
		}
	}
	return result
}

// scoreGraphResult 基于 nodes 和 edges 的质量计算相关性分数。
// 考虑因素：实体数量、关系数量、关系强度(score)、实体频率(frequency)。
func scoreGraphResult(nodes []*core.Node, edges []*core.Edge) float32 {
	if len(nodes) == 0 {
		return 0
	}

	// 基础分
	baseScore := float32(0.3)

	// 实体贡献：每个实体 +0.05
	entityBonus := float32(len(nodes)) * 0.05

	// 关系贡献：每条关系 +0.03
	relationBonus := float32(len(edges)) * 0.03

	// 关系强度加成：边的 score 属性（如果存在）
	edgeScoreSum := float32(0)
	edgeScoreCount := 0
	for _, edge := range edges {
		if edge.Properties != nil {
			if s, ok := edge.Properties["score"].(float64); ok {
				edgeScoreSum += float32(s)
				edgeScoreCount++
			}
		}
	}
	strengthBonus := float32(0)
	if edgeScoreCount > 0 {
		avgStrength := edgeScoreSum / float32(edgeScoreCount)
		strengthBonus = avgStrength * 0.1
	}

	// 实体频率加成：高频实体更相关
	freqBonus := float32(0)
	for _, node := range nodes {
		if node.Properties != nil {
			if f, ok := node.Properties["frequency"].(int); ok && f > 0 {
				freqBonus += float32(math.Min(float64(f), 10)) * 0.01
			}
		}
	}

	total := baseScore + entityBonus + relationBonus + strengthBonus + freqBonus
	if total > 1.0 {
		return 1.0
	}
	return total
}
