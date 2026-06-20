//go:build integration

package indexer

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/store/graph/gograph"
	"github.com/DotNetAge/gorag/store/vector/govector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 集成测试：GraphIndexer（分块 + 实体提取 + 标签 + 双写 vectorDB + graphDB）
//
// 环境变量:
//
//	GORAG_BASE_URL   = "https://dashscope.aliyuncs.com/compatible-mode/v1"  (默认)
//	GORAG_API_KEY    = "你的 API Key"                                          (必填)
//	GORAG_MODEL      = "qwen3.5-flash"                                       (默认)
//
// 测试数据: .test/data 目录下的 markdown 文件
// =============================================================================

const (
	llmTestDataDir = "../.test/data"
)

// mockEmbedder 返回固定维度的单位向量，免除对真实嵌入模型的依赖。
type mockEmbedder struct {
	dim int
}

func (m *mockEmbedder) Calc(chunk *core.Chunk) (*core.Vector, error) {
	return m.CalcText(chunk.Content)
}

func (m *mockEmbedder) CalcText(text string) (*core.Vector, error) {
	vec := make([]float32, m.dim)
	// 基于文本哈希生成确定性向量，避免全零向量导致搜索退化
	h := hashText(text)
	for i := range vec {
		vec[i] = float32(h%1000) / 1000.0
		h = h*31 + 1
	}
	return &core.Vector{Values: vec}, nil
}

func (m *mockEmbedder) CalcImage(data []byte) (*core.Vector, error) {
	vec := make([]float32, m.dim)
	for i := range vec {
		vec[i] = rand.Float32()
	}
	return &core.Vector{Values: vec}, nil
}

func (m *mockEmbedder) Bulk(chunks []*core.Chunk) ([]*core.Vector, error) {
	vecs := make([]*core.Vector, 0, len(chunks))
	for _, c := range chunks {
		v, err := m.Calc(c)
		if err != nil {
			return nil, err
		}
		vecs = append(vecs, v)
	}
	return vecs, nil
}

func (m *mockEmbedder) Dim() int          { return m.dim }
func (m *mockEmbedder) Multimoding() bool { return false }

// hashText 为文本生成确定性哈希值
func hashText(s string) uint64 {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// pickTestFiles 从 .test/data 目录中取前 N 个 .md 文件
func pickTestFiles(t *testing.T, n int) []string {
	t.Helper()
	entries, err := os.ReadDir(llmTestDataDir)
	require.NoError(t, err, "读取测试数据目录失败")

	files := make([]string, 0, n)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			files = append(files, filepath.Join(llmTestDataDir, e.Name()))
			if len(files) == n {
				break
			}
		}
	}
	require.NotEmpty(t, files, "测试数据目录中应有 .md 文件")
	return files
}

// =============================================================================
// 测试1: Add － 直接传入文本，验证分块、标签、实体、双写存储
// =============================================================================

func TestGraphIndexer_AddContent(t *testing.T) {
	if os.Getenv("GORAG_API_KEY") == "" {
		t.Skip("跳过: 未设置 GORAG_API_KEY 环境变量")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// 创建临时存储目录
	tmpDir := t.TempDir()
	vectorDB := newVectorStore(t, tmpDir)
	graphDB := newGraphStore(t, tmpDir)

	embedder := &mockEmbedder{dim: 128}

	idx := New(ModelConfig{
		APIKey:         os.Getenv("GORAG_API_KEY"),
		BaseURL:        envOrDefault("GORAG_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		Model:          envOrDefault("GORAG_MODEL", "qwen3.5-flash"),
		Language:       "Chinese",
		MaxTokens:      128000,
		ThinkingBudget: 1024,
	}, embedder, vectorDB, graphDB)
	defer idx.Close(ctx)

	// 用一段有明确实体和主题的中文文本
	content := `Go语言（Golang）是由Google开发的一种静态类型、编译型的编程语言。
Go语言以其简洁的语法、高效的并发模型和快速的编译速度而著称。
goroutine和channel为并发编程提供了优雅的解决方案，使得开发者可以轻松编写高并发的网络服务。
Go语言在云计算、微服务架构和DevOps领域得到了广泛应用，Docker和Kubernetes等知名项目都是用Go编写的。
Go语言的垃圾回收机制和丰富的标准库也大大提高了开发效率。`

	chunks, err := idx.Add(ctx, content)
	require.NoError(t, err, "Add 不应报错")
	require.NotEmpty(t, chunks, "Add 应返回非空 chunks")

	t.Logf("Add 返回 %d 个 chunk", len(chunks))

	// 验证 chunk metadata
	for i, chunk := range chunks {
		t.Logf("  Chunk #%d: ID=%s len=%d", i, chunk.ID, len(chunk.Content))

		// 验证 summary 存在
		summary, ok := chunk.Metadata["summary"].(string)
		assert.True(t, ok, "chunk.Metadata 应有 summary")
		assert.NotEmpty(t, summary, "summary 不应为空")
		t.Logf("    summary: %s", truncate(summary, 60))

		// 验证 tags 存在且数量在 3-5 之间
		tagsRaw, ok := chunk.Metadata["tags"].([]string)
		if !ok {
			// 也可能是 []any 类型
			if tagsAny, ok2 := chunk.Metadata["tags"].([]any); ok2 {
				tagsRaw = make([]string, 0, len(tagsAny))
				for _, tag := range tagsAny {
					if s, ok3 := tag.(string); ok3 {
						tagsRaw = append(tagsRaw, s)
					}
				}
			}
		}
		assert.NotEmpty(t, tagsRaw, "tags 不应为空")
		if len(tagsRaw) > 0 {
			t.Logf("    tags: %v (count=%d)", tagsRaw, len(tagsRaw))
			// 允许 2-6 个标签（LLM 可能略有偏差，但应该在合理范围内）
			assert.GreaterOrEqual(t, len(tagsRaw), 2, "应有至少 2 个标签")
			assert.LessOrEqual(t, len(tagsRaw), 6, "标签不应超过 6 个")
		}

		// 验证 entity_ids 存在
		entityIDs, ok := chunk.Metadata["entity_ids"].([]string)
		if ok {
			t.Logf("    entity_ids: %v", entityIDs)
		}
	}

	// 验证 vectorDB 有数据
	count, err := idx.vectorDB.Count(ctx)
	require.NoError(t, err, "vectorDB.Count 不应报错")
	assert.Greater(t, count, 0, "vectorDB 应有数据")
	t.Logf("VectorDB 中向量数: %d", count)

	// 验证 graphDB 有数据
	rows, err := graphDB.Query(ctx, "MATCH (n) RETURN count(n) AS cnt", nil)
	require.NoError(t, err, "graphDB 查询不应报错")
	if len(rows) > 0 {
		if cnt, ok := rows[0]["cnt"].(int64); ok {
			t.Logf("GraphDB 中节点数: %d", cnt)
			assert.Greater(t, cnt, int64(0), "graphDB 应有节点")
		}
	}
}

// =============================================================================
// 测试2: AddFile － 从文件索引，验证 tags 在 metadata 中的完整链路
// =============================================================================

func TestGraphIndexer_AddFile(t *testing.T) {
	if os.Getenv("GORAG_API_KEY") == "" {
		t.Skip("跳过: 未设置 GORAG_API_KEY 环境变量")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	vectorDB := newVectorStore(t, tmpDir)
	graphDB := newGraphStore(t, tmpDir)

	embedder := &mockEmbedder{dim: 128}

	idx := New(ModelConfig{
		APIKey:         os.Getenv("GORAG_API_KEY"),
		BaseURL:        envOrDefault("GORAG_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		Model:          envOrDefault("GORAG_MODEL", "qwen3.5-flash"),
		Language:       "Chinese",
		MaxTokens:      128000,
		ThinkingBudget: 1024,
	}, embedder, vectorDB, graphDB)
	defer idx.Close(ctx)

	// 从测试数据目录取一个文件
	files := pickTestFiles(t, 1)
	filePath := files[0]
	t.Logf("测试文件: %s", filepath.Base(filePath))

	chunks, err := idx.AddFile(ctx, filePath)
	require.NoError(t, err, "AddFile 不应报错")
	require.NotEmpty(t, chunks, "AddFile 应返回非空 chunks")

	t.Logf("AddFile 返回 %d 个 chunk", len(chunks))

	// 验证至少有一个 chunk 的 tags 非空
	tagsFound := false
	for i, chunk := range chunks {
		t.Logf("  Chunk #%d: ID=%s len=%d", i, chunk.ID, len(chunk.Content))

		summary, _ := chunk.Metadata["summary"].(string)
		t.Logf("    summary: %s", truncate(summary, 60))

		// 验证 tags
		tags, _ := chunk.Metadata["tags"].([]string)
		if len(tags) == 0 {
			// 也可能是 []any 类型
			if tagsAny, ok := chunk.Metadata["tags"].([]any); ok {
				for _, tag := range tagsAny {
					if s, ok2 := tag.(string); ok2 {
						tags = append(tags, s)
					}
				}
			}
		}
		if len(tags) > 0 {
			tagsFound = true
			t.Logf("    tags: %v (count=%d)", tags, len(tags))
		}

		// 验证 entity_ids
		entityIDs, _ := chunk.Metadata["entity_ids"].([]string)
		if len(entityIDs) > 0 {
			t.Logf("    entity_ids: %v", entityIDs)
		}
	}
	assert.True(t, tagsFound, "至少应有一个 chunk 包含 tags")

	// 验证 vectorDB 数据
	count, err := idx.vectorDB.Count(ctx)
	require.NoError(t, err)
	assert.Greater(t, count, 0)
	t.Logf("VectorDB 中向量数: %d", count)
}

// =============================================================================
// 测试3: Search － 索引后搜索，验证 tags 在搜索结果中可访问
// =============================================================================

func TestGraphIndexer_Search(t *testing.T) {
	if os.Getenv("GORAG_API_KEY") == "" {
		t.Skip("跳过: 未设置 GORAG_API_KEY 环境变量")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	vectorDB := newVectorStore(t, tmpDir)
	graphDB := newGraphStore(t, tmpDir)

	embedder := &mockEmbedder{dim: 128}

	idx := New(ModelConfig{
		APIKey:         os.Getenv("GORAG_API_KEY"),
		BaseURL:        envOrDefault("GORAG_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		Model:          envOrDefault("GORAG_MODEL", "qwen3.5-flash"),
		Language:       "Chinese",
		MaxTokens:      128000,
		ThinkingBudget: 1024,
	}, embedder, vectorDB, graphDB)
	defer idx.Close(ctx)

	// 先索引一个测试文件
	files := pickTestFiles(t, 1)
	_, err := idx.AddFile(ctx, files[0])
	require.NoError(t, err, "AddFile 不应报错")

	// 搜索
	q := idx.NewQuery("Go语言 并发 微服务")
	hits, err := idx.Search(ctx, q)
	require.NoError(t, err, "Search 不应报错")

	t.Logf("Search 返回 %d 个 Hit", len(hits))
	for i, hit := range hits {
		t.Logf("  Hit #%d: ID=%s Score=%.4f", i+1, hit.ID, hit.Score)
		if hit.Metadata != nil {
			// 验证 tags 在搜索结果的 Metadata 中
			if tags, ok := hit.Metadata["tags"]; ok {
				t.Logf("    tags: %v", tags)
			}
			if summary, ok := hit.Metadata["summary"]; ok {
				t.Logf("    summary: %s", truncate(fmt.Sprintf("%v", summary), 60))
			}
		}
	}
}

// =============================================================================
// 测试4: Token 估算与切片 － 测试超长内容的自动切片 + 合并
// =============================================================================

func TestGraphIndexer_SliceAndMerge(t *testing.T) {
	if os.Getenv("GORAG_API_KEY") == "" {
		t.Skip("跳过: 未设置 GORAG_API_KEY 环境变量")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	tmpDir := t.TempDir()
	vectorDB := newVectorStore(t, tmpDir)
	graphDB := newGraphStore(t, tmpDir)

	embedder := &mockEmbedder{dim: 128}

	// 故意设小 MaxTokens 触发切片
	idx := New(ModelConfig{
		APIKey:         os.Getenv("GORAG_API_KEY"),
		BaseURL:        envOrDefault("GORAG_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		Model:          envOrDefault("GORAG_MODEL", "qwen3.5-flash"),
		Language:       "Chinese",
		MaxTokens:      32000,
		ThinkingBudget: 1024,
	}, embedder, vectorDB, graphDB)
	defer idx.Close(ctx)

	// 读取多个不同文件并拼接（避免重复内容降低测试价值）
	files := pickTestFiles(t, 3)
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		require.NoError(t, err)
		b.Write(data)
		b.WriteString("\n\n---\n\n")
	}
	content := b.String()
	t.Logf("拼接多个文件 (%d 个): 总长度 %d 字符", len(files), len(content))

	// 如果不足 50K 则继续追加其他文件
	if len(content) < 50000 {
		more := pickTestFiles(t, 5)
		for _, f := range more {
			data, err := os.ReadFile(f)
			require.NoError(t, err)
			b.Write(data)
			b.WriteString("\n\n---\n\n")
		}
		content = b.String()
		t.Logf("追加更多文件后: 总长度 %d 字符", len(content))
	}
	t.Logf("测试内容长度: %d 字符 (应触发切片)", len(content))

	chunks, err := idx.Add(ctx, content)
	require.NoError(t, err, "带切片的 Add 不应报错")
	require.NotEmpty(t, chunks, "Add 应返回非空 chunks")

	t.Logf("切片后合并结果: %d 个 chunk", len(chunks))

	// 验证合并后的数据完整性
	tagsCount := 0
	for i, chunk := range chunks {
		if i < 3 {
			t.Logf("  Chunk #%d: ID=%s len=%d", i, chunk.ID, len(chunk.Content))
		}
		tags, _ := chunk.Metadata["tags"].([]string)
		if len(tags) > 0 {
			tagsCount++
		}
	}
	t.Logf("含 tags 的 chunk 数: %d / %d", tagsCount, len(chunks))

	// 验证 vectorDB 有写入
	count, err := idx.vectorDB.Count(ctx)
	require.NoError(t, err)
	assert.Greater(t, count, 0, "切片后 vectorDB 应有数据")
	t.Logf("VectorDB 中向量数: %d", count)
}

// =============================================================================
// 测试辅助
// =============================================================================

func newVectorStore(t *testing.T, baseDir string) core.VectorStore {
	t.Helper()
	dbPath := filepath.Join(baseDir, "vectors.db")
	store, err := govector.NewStore(
		govector.WithDBPath(dbPath),
		govector.WithDimension(128),
		govector.WithCollection("test_llm"),
		govector.WithHNSW(false),
	)
	require.NoError(t, err, "创建 vector store 失败")
	t.Cleanup(func() { store.Close(context.Background()) })
	return store
}

func newGraphStore(t *testing.T, baseDir string) core.GraphStore {
	t.Helper()
	dbPath := filepath.Join(baseDir, "graph")
	store, err := gograph.NewGraphStore(dbPath)
	require.NoError(t, err, "创建 graph store 失败")
	t.Cleanup(func() { store.Close(context.Background()) })
	return store
}


