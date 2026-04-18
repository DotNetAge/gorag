package indexer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	chatcore "github.com/DotNetAge/gochat/core"
	"github.com/DotNetAge/gochat/client/openai"
	"github.com/DotNetAge/gorag/core"
	bboltcache "github.com/DotNetAge/gorag/store/cache"
	"github.com/DotNetAge/gorag/store/graph/gograph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 集成测试：图索引器的分块索引与实体提取
// 环境变量:
//   GORAG_BASE_URL   = "https://dashscope.aliyuncs.com/compatible-mode/v1"
//   GORAG_API_KEY    = "你的 API Key"
//   GORAG_MODEL      = "qwen3.5-flash"
// 测试数据: .test/data 目录下的 markdown 文件
// =============================================================================

const (
	graphTestDBPath = "../.test/graph_test_db"
	graphTestDataDir = "../.test/data"
)

// envOrDefault 读取环境变量，若为空则返回默认值
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// newTestClient 从环境变量创建真实的 LLM 客户端
func newTestClient(t *testing.T) chatcore.Client {
	t.Helper()
	client, err := openai.NewOpenAI(chatcore.Config{
		BaseURL: envOrDefault("GORAG_BASE_URL", "https://dashscope.aliyuncs.com/compatible-mode/v1"),
		APIKey:  os.Getenv("GORAG_API_KEY"),
		Model:   envOrDefault("GORAG_MODEL", "qwen3.5-flash"),
		Timeout: 120 * time.Second,
	})
	require.NoError(t, err, "创建 LLM 客户端失败")
	return client
}

// pickTwoFiles 从 .test/data 目录中取前两个 .md 文件
func pickTwoFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(graphTestDataDir)
	require.NoError(t, err, "读取测试数据目录失败")
	files := make([]string, 0, 2)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			files = append(files, filepath.Join(graphTestDataDir, e.Name()))
			if len(files) == 2 {
				break
			}
		}
	}
	require.NotEmpty(t, files, "测试数据目录中应有至少 2 个 .md 文件")
	return files
}

// =============================================================================
// 测试1: 索引文件 → 实体提取并存储到图数据库
// 期待: 文件分块后，LLM 成功提取实体和关系，写入 gograph 数据库
// =============================================================================

func TestGraphIndexer_AddFile(t *testing.T) {
	if os.Getenv("GORAG_API_KEY") == "" {
		t.Skip("跳过: 未设置 GORAG_API_KEY 环境变量")
	}

	// 清理旧数据库
	_ = os.RemoveAll(graphTestDBPath)

	// 初始化图存储
	store, err := newGraphStoreForTest(t, graphTestDBPath)
	require.NoError(t, err, "创建 gograph 失败")
	defer store.Close(context.Background())

	// 创建图索引器（带 LLM client）
	client := newTestClient(t)
	gi := NewGraphIndexer(store, client)
	defer gi.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	// 取两个文件并索引
	files := pickTwoFiles(t)
	for _, f := range files {
		chunk, err := gi.AddFile(ctx, f)
		require.NoError(t, err, "AddFile(%s) 不应报错", f)
		require.NotNil(t, chunk, "AddFile(%s) 应返回非 nil chunk", f)
		t.Logf("已索引: %s -> chunkID=%s", filepath.Base(f), chunk.ID)
	}

	// 验证: 通过 Cypher 查询图中是否有节点
	rows, err := store.Query(ctx, "MATCH (n) RETURN n LIMIT 5", nil)
	require.NoError(t, err, "查询节点不应报错")
	require.NotEmpty(t, rows, "图中应有节点")
	t.Logf("查询返回 %d 行节点数据", len(rows))

	// 验证节点有 id 和 properties
	nodeCount := 0
	for _, row := range rows {
		if nodeMap, ok := row["n"].(map[string]any); ok {
			if _, hasID := nodeMap["id"]; hasID {
				nodeCount++
			}
		}
	}
	assert.Greater(t, nodeCount, 0, "图中应有至少 1 个带 id 的节点")

	// 验证: 通过 Cypher 查询图中是否有边
	edgeRows, err := store.Query(ctx, "MATCH ()-[r]->() RETURN r LIMIT 5", nil)
	require.NoError(t, err, "查询边不应报错")
	t.Logf("查询返回 %d 行边数据", len(edgeRows))
}

// =============================================================================
// 测试2: Search —— 独立 GraphRAG 模式，通过查询实体遍历图
// 期待: Search 返回包含实体和关系的 Hit
// =============================================================================

func TestGraphIndexer_Search(t *testing.T) {
	if os.Getenv("GORAG_API_KEY") == "" {
		t.Skip("跳过: 未设置 GORAG_API_KEY 环境变量")
	}

	// 打开已有的测试数据库（依赖测试1已运行）
	store, err := newGraphStoreForTest(t, graphTestDBPath)
	require.NoError(t, err, "打开 gograph 失败")
	defer store.Close(context.Background())

	client := newTestClient(t)
	gi := NewGraphIndexer(store, client)
	defer gi.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 先确认图中有数据
	rows, err := store.Query(ctx, "MATCH (n) RETURN count(n) AS cnt", nil)
	require.NoError(t, err)
	if len(rows) == 0 {
		t.Skip("跳过: 图数据库中无数据，请先运行 TestGraphIndexer_AddFile")
	}

	// 读取其中一个测试文件内容，取关键句子作为查询
	files := pickTwoFiles(t)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	content := string(data)

	// 提取一个有意义的查询（取前 200 字符）
	queryText := content
	if len([]rune(queryText)) > 200 {
		queryText = string([]rune(queryText)[:200])
	}

	q := gi.NewQuery(queryText)
	hits, err := gi.Search(ctx, q)
	require.NoError(t, err, "Search 不应报错")
	require.NotEmpty(t, hits, "Search 应返回至少 1 个 Hit")

	t.Logf("Search 返回 %d 个 Hit", len(hits))
	for i, hit := range hits {
		t.Logf("  Hit #%d: ID=%s Score=%.4f DocID=%s", i+1, hit.ID, hit.Score, hit.DocID)
		assert.NotEmpty(t, hit.ID, "Hit.ID 不应为空")
		assert.Greater(t, hit.Score, float32(0), "Hit.Score 应 > 0")
		assert.NotEmpty(t, hit.Content, "Hit.Content 不应为空")

		// 验证 Content 可解析为 GraphSearchResult，且包含实体
		var gsr GraphSearchResult
		err := json.Unmarshal([]byte(hit.Content), &gsr)
		require.NoError(t, err, "Hit.Content 应为有效 JSON (GraphSearchResult)")

		t.Logf("    实体数=%d 关系数=%d", len(gsr.Entities), len(gsr.Relations))

		// 验证实体内容质量
		if len(gsr.Entities) > 0 {
			for j, e := range gsr.Entities {
				assert.NotEmpty(t, e.ID, "实体 ID 不应为空")
				assert.NotEmpty(t, e.Name, "实体 Name 不应为空")
				if j == 0 {
					t.Logf("    首个实体: ID=%s Name=%s Type=%s", e.ID, e.Name, e.Type)
				}
			}
		}
	}
}

// =============================================================================
// 测试3: SearchByChunkIDs —— 混合模式，通过 Chunk IDs 查询关联图结构
// 期待: 输入 chunkID → 返回关联的节点和边
// =============================================================================

func TestGraphIndexer_SearchByChunkIDs(t *testing.T) {
	// 此测试不需要 LLM（混合模式）

	// 打开已有的测试数据库
	store, err := newGraphStoreForTest(t, graphTestDBPath)
	require.NoError(t, err, "打开 gograph 失败")
	defer store.Close(context.Background())

	// 创建图索引器（不需要 client）
	gi := NewGraphIndexer(store)
	defer gi.Close(context.Background())

	ctx := context.Background()

	// 直接用 GetNodesByChunkIDs 不合适（它需要已知 chunkIDs），
	// 改为从 AddFile 阶段已知的 chunkID 来获取
	// 这里通过 Cypher 查询所有节点，取 source_chunk_ids 属性
	rows, err := store.Query(ctx, "MATCH (n) RETURN n LIMIT 10", nil)
	require.NoError(t, err, "查询节点不应报错")

	if len(rows) == 0 {
		t.Skip("跳过: 图中无数据，请先运行 TestGraphIndexer_AddFile")
	}

	// 收集有效的 chunkIDs
	// 注意：gograph Query 返回的 source_chunk_ids 属性可能是字符串 "[chunk_xxx,chunk_yyy]" 格式
	// 需要解析并清理方括号
	chunkIDs := make([]string, 0)
	seen := make(map[string]bool)
	for _, row := range rows {
		nodeMap, ok := row["n"].(map[string]any)
		if !ok {
			continue
		}
		props, ok := nodeMap["properties"].(map[string]any)
		if !ok {
			props = nodeMap
		}
		cids, ok := props["source_chunk_ids"]
		if !ok {
			continue
		}
		var parts []string
		switch v := cids.(type) {
		case []string:
			parts = v
		case string:
			if v != "" {
				parts = strings.Split(v, ",")
			}
		}
		for _, p := range parts {
			p = strings.TrimSpace(p)
			// 清理 gograph 序列化带来的方括号
			p = strings.TrimPrefix(p, "[")
			p = strings.TrimSuffix(p, "]")
			if p != "" && !seen[p] {
				seen[p] = true
				chunkIDs = append(chunkIDs, p)
			}
		}
	}
	require.NotEmpty(t, chunkIDs, "应有至少 1 个 chunk ID")

	// 只取前 2 个 chunkID 测试
	testChunkIDs := chunkIDs
	if len(testChunkIDs) > 2 {
		testChunkIDs = testChunkIDs[:2]
	}
	t.Logf("测试 chunkIDs: %v (len=%d)", testChunkIDs, len(testChunkIDs))

	// 执行 SearchByChunkIDs (depth=1, limit=10)
	hits, err := gi.SearchByChunkIDs(ctx, testChunkIDs, 1, 10)
	require.NoError(t, err, "SearchByChunkIDs 不应报错")
	require.NotEmpty(t, hits, "SearchByChunkIDs 应返回至少 1 个 Hit")

	t.Logf("SearchByChunkIDs 返回 %d 个 Hit", len(hits))
	for i, hit := range hits {
		t.Logf("  Hit #%d: ID=%s Score=%.4f", i+1, hit.ID, hit.Score)
		assert.NotEmpty(t, hit.ID, "Hit.ID 不应为空")
		assert.GreaterOrEqual(t, hit.Score, float32(0.3), "Score 应 >= 0.3 (base score)")
		assert.NotEmpty(t, hit.Content, "Hit.Content 不应为空")

		// 验证评分上限
		assert.LessOrEqual(t, hit.Score, float32(1.0), "Score 不应超过 1.0")
	}
}

// =============================================================================
// 测试4: SearchByChunkIDs depth=0 —— 仅返回直接关联的边，不做多跳
// 期待: depth=0 时也能正确返回直接关联节点
// =============================================================================

func TestGraphIndexer_SearchByChunkIDs_ZeroDepth(t *testing.T) {
	store, err := newGraphStoreForTest(t, graphTestDBPath)
	require.NoError(t, err, "打开 gograph 失败")
	defer store.Close(context.Background())

	gi := NewGraphIndexer(store)
	defer gi.Close(context.Background())

	ctx := context.Background()

	// 获取节点上的 chunkID
	rows, err := store.Query(ctx, "MATCH (n) RETURN n LIMIT 5", nil)
	require.NoError(t, err)
	if len(rows) == 0 {
		t.Skip("跳过: 图中无数据")
	}

	var chunkIDs []string
	for _, row := range rows {
		nodeMap, ok := row["n"].(map[string]any)
		if !ok {
			continue
		}
		props, ok := nodeMap["properties"].(map[string]any)
		if !ok {
			props = nodeMap
		}
		cids, ok := props["source_chunk_ids"]
		if !ok {
			continue
		}
		switch v := cids.(type) {
		case []string:
			chunkIDs = v
		case string:
			if v != "" {
				for _, p := range strings.Split(v, ",") {
					p = strings.TrimSpace(p)
					p = strings.TrimPrefix(p, "[")
					p = strings.TrimSuffix(p, "]")
					if p != "" {
						chunkIDs = append(chunkIDs, p)
					}
				}
			}
		}
		if len(chunkIDs) > 0 {
			break
		}
	}
	require.NotEmpty(t, chunkIDs, "应有 chunk ID")

	// depth=0 应该仍然返回结果（直接关联节点 + 直接边）
	hits, err := gi.SearchByChunkIDs(ctx, chunkIDs[:1], 0, 10)
	require.NoError(t, err, "depth=0 时不应报错")

	t.Logf("depth=0: 返回 %d 个 Hit", len(hits))
	if len(hits) > 0 {
		for _, hit := range hits {
			t.Logf("  ID=%s Score=%.4f", hit.ID, hit.Score)
			assert.NotEmpty(t, hit.Content)
		}
	}
}

// TestGraphIndexer_Cache 验证实体提取缓存功能
// 第一次索引调用 LLM，第二次相同内容直接命中缓存，不调用 LLM
func TestGraphIndexer_Cache(t *testing.T) {
	ctx := context.Background()
	client := newTestClient(t)

	// 使用临时缓存目录
	cacheDir := t.TempDir()
	cachePath := filepath.Join(cacheDir, "entity_cache.db")

	// 第一次索引：无缓存，所有 chunk 都需要调用 LLM
	store1, err := gograph.NewGraphStore(filepath.Join(t.TempDir(), "graph1"))
	require.NoError(t, err)
	defer store1.Close(ctx)

	cacheStore1, err := bboltcache.NewBoltCache(cachePath)
	require.NoError(t, err)
	gi1 := NewGraphIndexer(store1, client, WithCache(cacheStore1))

	start1 := time.Now()
	files := pickTwoFiles(t)
	for _, f := range files {
		_, err := gi1.AddFile(ctx, f)
		require.NoError(t, err, "第一次索引失败")
	}
	duration1 := time.Since(start1)
	t.Logf("第一次索引（无缓存）: %v", duration1)

	cacheSize1 := gi1.cache.Len()
	t.Logf("第一次索引后缓存条目数: %d", cacheSize1)
	assert.Greater(t, cacheSize1, 0, "缓存应有条目")

	gi1.Close(ctx)

	// 第二次索引：使用缓存，相同内容应命中缓存
	store2, err := gograph.NewGraphStore(filepath.Join(t.TempDir(), "graph2"))
	require.NoError(t, err)
	defer store2.Close(ctx)

	cacheStore2, err := bboltcache.NewBoltCache(cachePath)
	require.NoError(t, err)
	gi2 := NewGraphIndexer(store2, client, WithCache(cacheStore2))

	start2 := time.Now()
	for _, f := range files {
		_, err := gi2.AddFile(ctx, f)
		require.NoError(t, err, "第二次索引失败")
	}
	duration2 := time.Since(start2)
	t.Logf("第二次索引（有缓存）: %v", duration2)

	// 缓存命中后应显著更快（至少快 3 倍）
	assert.Less(t, duration2, duration1/3,
		"缓存命中后应显著更快: 第一次=%v, 第二次=%v", duration1, duration2)

	t.Logf("缓存加速比: %.1fx", float64(duration1)/float64(duration2))

	gi2.Close(ctx)
}

// newGraphStoreForTest 创建测试用的 gograph 存储实例
func newGraphStoreForTest(t *testing.T, path string) (core.GraphStore, error) {
	t.Helper()
	return gograph.NewGraphStore(path)
}
