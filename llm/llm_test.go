package llm

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/v2/chunker"
	"github.com/DotNetAge/gorag/v2/core"
	"github.com/DotNetAge/gorag/v2/logging"
)

// validConfig 返回用于测试的合法 LLM 配置（不会发起真实请求）。
func validConfig() Config {
	return Config{
		APIKey:  "test-key",
		BaseURL: "http://localhost:8080/v1",
		Model:   "test-model",
	}
}

// TestNewSummarizer_ValidConfig 验证合法配置能成功创建 Summarizer。
func TestNewSummarizer_ValidConfig(t *testing.T) {
	_, err := NewSummarizer(validConfig(), logging.DefaultNoopLogger())
	if err != nil {
		t.Fatalf("期望创建成功，实际错误: %v", err)
	}
}

// TestNewSummarizer_MissingLogger 验证 logger 为空时返回错误。
func TestNewSummarizer_MissingLogger(t *testing.T) {
	_, err := NewSummarizer(validConfig(), nil)
	if err == nil {
		t.Fatalf("期望 logger 为空时返回错误")
	}
}

// TestNewSummarizer_MissingConfig 验证配置不完整时返回错误。
func TestNewSummarizer_MissingConfig(t *testing.T) {
	_, err := NewSummarizer(Config{}, logging.DefaultNoopLogger())
	if err == nil {
		t.Fatalf("期望配置不完整时返回错误")
	}
}

// TestParseSummarizeResult 验证 Summarizer 的 LLM 响应解析。
func TestParseSummarizeResult(t *testing.T) {
	resp := "```json\n{\"title\":\"标题\",\"summary\":\"摘要。\"}\n```"
	res, err := parseSummarizeResult(resp)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Title != "标题" {
		t.Errorf("title 期望 %q，实际 %q", "标题", res.Title)
	}
	if res.Summary != "摘要。" {
		t.Errorf("summary 期望 %q，实际 %q", "摘要。", res.Summary)
	}
}

// TestNewRefiller_ValidConfig 验证合法配置能成功创建 Refiller。
func TestNewRefiller_ValidConfig(t *testing.T) {
	_, err := NewRefiller(validConfig(), logging.DefaultNoopLogger())
	if err != nil {
		t.Fatalf("期望创建成功，实际错误: %v", err)
	}
}

// TestNewRefiller_MissingLogger 验证 logger 为空时返回错误。
func TestNewRefiller_MissingLogger(t *testing.T) {
	_, err := NewRefiller(validConfig(), nil)
	if err == nil {
		t.Fatalf("期望 logger 为空时返回错误")
	}
}

// TestCollectDocIDs 验证 DocID 去重逻辑。
func TestCollectDocIDs(t *testing.T) {
	chunks := []core.Chunk{
		{ID: "c1", DocID: "doc-a"},
		{ID: "c2", DocID: "doc-b"},
		{ID: "c3", DocID: "doc-a"},
	}
	ids := collectDocIDs(chunks)
	if len(ids) != 2 {
		t.Fatalf("期望 2 个 DocID，实际 %d", len(ids))
	}
}

// TestSerializeChunks 验证 Chunk 序列化只保留关键字段。
func TestSerializeChunks(t *testing.T) {
	chunks := []core.Chunk{
		{
			ID:      "c1",
			Title:   "标题",
			Summary: "摘要",
			Content: "内容",
			Metadata: map[string]any{
				"lang": "go",
			},
		},
	}
	s, err := serializeChunks(chunks)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if s == "" {
		t.Fatalf("序列化结果不应为空")
	}
	if !contains(s, "c1") || !contains(s, "标题") || !contains(s, "内容") {
		t.Errorf("序列化结果缺少关键字段: %s", s)
	}
}

// TestParseRefillExtraction 验证 Refiller 的 LLM 响应解析。
func TestParseRefillExtraction(t *testing.T) {
	resp := `{
		"entities": [{"name": "Alice", "entity_type": "person"}],
		"relations": [{"subject": "Alice", "predicate": "works_for", "object": "Acme"}]
	}`
	ext, err := parseRefillExtraction(resp)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(ext.Entities) != 1 {
		t.Errorf("期望 1 个 entity，实际 %d", len(ext.Entities))
	}
	if len(ext.Relations) != 1 {
		t.Errorf("期望 1 个 relation，实际 %d", len(ext.Relations))
	}
}

// TestBuildNodesAndEdges 验证提取结果转换为 Node/Edge 的语义。
func TestBuildNodesAndEdges(t *testing.T) {
	ext := refillExtraction{
		Entities: []refillEntity{
			{Name: "Alice", EntityType: "person"},
			{Name: "Acme", EntityType: "organization", Properties: map[string]any{"country": "CN"}},
		},
		Relations: []refillRelation{
			{Subject: "Alice", Predicate: "works_for", Object: "Acme"},
		},
	}
	nodes, edges := buildNodesAndEdges(ext, []string{"doc-1"})
	if len(nodes) != 2 {
		t.Fatalf("期望 2 个 node，实际 %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("期望 1 个 edge，实际 %d", len(edges))
	}

	// Refiller 产出的实体是纯图实体，SourceChunkIDs 应为空
	for _, n := range nodes {
		if len(n.SourceChunkIDs) != 0 {
			t.Errorf("Node %q 的 SourceChunkIDs 应为空", n.Name)
		}
		if len(n.SourceDocIDs) == 0 {
			t.Errorf("Node %q 的 SourceDocIDs 不应为空", n.Name)
		}
	}
	if edges[0].Source == "" || edges[0].Target == "" {
		t.Errorf("Edge 的 Source/Target 应已解析为 Node ID")
	}
}

// TestNormalizeLLMJSON 验证 JSON 清洗逻辑。
func TestNormalizeLLMJSON(t *testing.T) {
	input := "```json\n{\"a\":\"中文标点，测试\"}\n```"
	got := normalizeLLMJSON(input)
	want := `{"a":"中文标点,测试"}`
	if got != want {
		t.Errorf("期望 %q，实际 %q", want, got)
	}
}

// contains 判断字符串 s 是否包含子串 sub。
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// TestRefill_EmptyChunks 验证空 Chunk 列表直接返回原结果。
func TestRefill_EmptyChunks(t *testing.T) {
	r, err := NewRefiller(validConfig(), logging.DefaultNoopLogger())
	if err != nil {
		t.Fatalf("创建 Refiller 失败: %v", err)
	}
	input := chunker.ChunkResult{}
	result, err := r.Refill(nil, input, nil)
	if err != nil {
		t.Fatalf("空 chunks 不应返回错误: %v", err)
	}
	if len(result.Nodes) != 0 || len(result.Edges) != 0 {
		t.Errorf("空 chunks 不应产生 Nodes/Edges")
	}
}

// TestSummarize_EmptyChunks 验证空 Chunk 列表直接返回。
func TestSummarize_EmptyChunks(t *testing.T) {
	s, err := NewSummarizer(validConfig(), logging.DefaultNoopLogger())
	if err != nil {
		t.Fatalf("创建 Summarizer 失败: %v", err)
	}
	result, err := s.Summarize(nil, nil)
	if err != nil {
		t.Fatalf("空 chunks 不应返回错误: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("空 chunks 应返回空切片")
	}
}

// TestLoadEntitySchema 验证从外部 JSON Schema 文件创建 EntitySchema。
func TestLoadEntitySchema(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/Employee.json"
	content := `{
		"type": "object",
		"description": "组织内的员工",
		"properties": {
			"name": {"type": "string", "description": "员工全名"},
			"email": {"type": "string", "format": "email", "description": "员工邮箱"}
		},
		"required": ["name", "email"]
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("创建临时 schema 文件失败: %v", err)
	}

	schema, err := LoadEntitySchema(path)
	if err != nil {
		t.Fatalf("加载 schema 失败: %v", err)
	}
	if schema.Type != "Employee" {
		t.Errorf("Type 期望 Employee，实际 %q", schema.Type)
	}
	if !strings.Contains(schema.Prompt, "组织内的员工") {
		t.Errorf("Prompt 应包含 description: %q", schema.Prompt)
	}
	if schema.JSONSchema == "" {
		t.Errorf("JSONSchema 不应为空")
	}
}

// TestLoadEntitySchemasFromDir 验证扫描目录加载多个 schema。
func TestLoadEntitySchemasFromDir(t *testing.T) {
	dir := t.TempDir()
	writeSchemaFile(t, dir+"/Employee.json", `{"description":"员工"}`)
	writeSchemaFile(t, dir+"/Contract.json", `{"description":"合同"}`)

	schemas, err := LoadEntitySchemasFromDir(dir)
	if err != nil {
		t.Fatalf("加载目录失败: %v", err)
	}
	if len(schemas) != 2 {
		t.Fatalf("期望加载 2 个 schema，实际 %d", len(schemas))
	}
}

// TestLoadEntitySchema_InvalidJSON 验证非法 JSON 返回错误。
func TestLoadEntitySchema_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/Bad.json"
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}
	_, err := LoadEntitySchema(path)
	if err == nil {
		t.Fatalf("期望非法 JSON 返回错误")
	}
}

// TestLoadEntitySchema_MissingFile 验证文件不存在返回错误。
func TestLoadEntitySchema_MissingFile(t *testing.T) {
	_, err := LoadEntitySchema(t.TempDir() + "/Missing.json")
	if err == nil {
		t.Fatalf("期望文件不存在返回错误")
	}
}

// writeSchemaFile 是测试辅助函数，用于写入临时 schema 文件。
func writeSchemaFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("写入 schema 文件失败: %v", err)
	}
}

// ── buildNodesAndEdges 单元测试 ────────────────────────────────────

// TestBuildNodesAndEdges_MissingEntity 验证关系引用未提取的实体时，
// 该关系被跳过而不是产生空 Source/Target 的边。
func TestBuildNodesAndEdges_MissingEntity(t *testing.T) {
	ext := refillExtraction{
		Entities: []refillEntity{
			{Name: "多维度索引", EntityType: "concept"},
		},
		Relations: []refillRelation{
			// "语义搜索" 不在实体列表中，这条边应被跳过
			{Subject: "多维度索引", Predicate: "enhances", Object: "语义搜索"},
			// "检索效果" 也不在实体列表中，应被跳过
			{Subject: "检索效果", Predicate: "depends_on", Object: "多维度索引"},
		},
	}
	nodes, edges := buildNodesAndEdges(ext, []string{"doc-1"})
	if len(nodes) != 1 {
		t.Fatalf("期望 1 个 node，实际 %d", len(nodes))
	}
	if len(edges) != 0 {
		t.Fatalf("期望 0 个 edge（引用不存在的实体），实际 %d", len(edges))
	}
}

// TestBuildNodesAndEdges_PartialMissingObject 验证关系主体存在但客体
// 不在实体列表中时，该关系被跳过。
func TestBuildNodesAndEdges_PartialMissingObject(t *testing.T) {
	ext := refillExtraction{
		Entities: []refillEntity{
			{Name: "Alice", EntityType: "person"},
		},
		Relations: []refillRelation{
			// Object "Acme" 不在实体列表中
			{Subject: "Alice", Predicate: "works_for", Object: "Acme"},
		},
	}
	_, edges := buildNodesAndEdges(ext, []string{"doc-1"})
	if len(edges) != 0 {
		t.Fatalf("期望 0 个 edge（Object 不存在），实际 %d", len(edges))
	}
}

// TestBuildNodesAndEdges_PartialMissingSubject 验证关系客体存在但主体
// 不在实体列表中时，该关系被跳过。
func TestBuildNodesAndEdges_PartialMissingSubject(t *testing.T) {
	ext := refillExtraction{
		Entities: []refillEntity{
			{Name: "Acme", EntityType: "organization"},
		},
		Relations: []refillRelation{
			// Subject "Alice" 不在实体列表中
			{Subject: "Alice", Predicate: "works_for", Object: "Acme"},
		},
	}
	_, edges := buildNodesAndEdges(ext, []string{"doc-1"})
	if len(edges) != 0 {
		t.Fatalf("期望 0 个 edge（Subject 不存在），实际 %d", len(edges))
	}
}

// TestBuildNodesAndEdges_AllMatch 验证实体和关系都匹配时，所有边正常产生。
func TestBuildNodesAndEdges_AllMatch(t *testing.T) {
	ext := refillExtraction{
		Entities: []refillEntity{
			{Name: "Alice", EntityType: "person"},
			{Name: "Acme", EntityType: "organization"},
		},
		Relations: []refillRelation{
			{Subject: "Alice", Predicate: "works_for", Object: "Acme"},
		},
	}
	_, edges := buildNodesAndEdges(ext, []string{"doc-1"})
	if len(edges) != 1 {
		t.Fatalf("期望 1 个 edge，实际 %d", len(edges))
	}
	if edges[0].Source == "" || edges[0].Target == "" {
		t.Errorf("Source 和 Target 不应为空: Source=%q Target=%q",
			edges[0].Source, edges[0].Target)
	}
}

// ── 集成测试：使用环境变量配置的真实 LLM ──────────────────────────

// envLLMConfig 从环境变量读取 LLM 配置，仅用于集成测试。
// 环境变量未设置时返回 nil，测试应跳过。
func envLLMConfig() *Config {
	apiKey := os.Getenv("GORAG_API_KEY")
	baseURL := os.Getenv("GORAG_BASE_URL")
	model := os.Getenv("GORAG_MODEL")
	if apiKey == "" || baseURL == "" || model == "" {
		return nil
	}
	return &Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
	}
}

// TestRefill_WithRealLLM 使用真实 LLM 验证实体提取全流程，
// 确保 LLM 返回的关系不会引用不存在的实体。
//
// 运行条件：GORAG_API_KEY、GORAG_BASE_URL、GORAG_MODEL 三个环境变量必须设置。
func TestRefill_WithRealLLM(t *testing.T) {
	cfg := envLLMConfig()
	if cfg == nil {
		t.Skip("跳过集成测试：未设置 GORAG_API_KEY / GORAG_BASE_URL / GORAG_MODEL")
	}

	r, err := NewRefiller(*cfg, logging.DefaultNoopLogger())
	if err != nil {
		t.Fatalf("创建 Refiller 失败: %v", err)
	}

	// 构建带真实内容的 ChunkResult
	input := chunker.ChunkResult{
		Chunks: []core.Chunk{
			{
				ID:      "c1",
				Title:   "项目概述",
				Content: "这是一个基于 Go 语言开发的 RAG 检索增强生成系统。系统支持语义检索和图检索两种索引方式。",
			},
			{
				ID:      "c2",
				Title:   "架构设计",
				Content: "系统采用分层架构，包含文档解析层、分块层、向量化层和存储层。向量化使用 Embedder 将文本转为向量。",
			},
		},
	}

	schemas := []EntitySchema{
		{
			Type:   "concept",
			Prompt: "技术概念或术语",
		},
		{
			Type:   "technology",
			Prompt: "技术栈或框架",
		},
	}

	result, err := r.Refill(context.Background(), input, schemas)
	if err != nil {
		t.Fatalf("Refill 失败: %v", err)
	}

	// 验证：所有边的 Source 和 Target 都应能在节点列表中找到
	nodeIDMap := make(map[string]bool, len(result.Nodes))
	for _, n := range result.Nodes {
		nodeIDMap[n.ID] = true
	}
	for i, e := range result.Edges {
		if !nodeIDMap[e.Source] {
			t.Errorf("Edge[%d] Source=%q 在节点列表中不存在", i, e.Source)
		}
		if !nodeIDMap[e.Target] {
			t.Errorf("Edge[%d] Target=%q 在节点列表中不存在", i, e.Target)
		}
	}

	t.Logf("集成测试通过：实体=%d 关系=%d", len(result.Nodes), len(result.Edges))
}
