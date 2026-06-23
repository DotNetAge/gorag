//go:build integration

package gorag

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/v2/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain clears LLM-related environment variables to prevent real API calls during tests
func TestMain(m *testing.M) {
	// Save and clear env vars that would trigger LLM client creation
	savedVars := map[string]string{}
	for _, key := range []string{"GORAG_MODEL", "GORAG_BASE_URL", "GORAG_API_KEY", "GORAG_AUTH_TOKEN"} {
		savedVars[key] = os.Getenv(key)
		os.Unsetenv(key)
	}

	code := m.Run()

	// Restore env vars
	for key, val := range savedVars {
		if val != "" {
			os.Setenv(key, val)
		}
	}
	os.Exit(code)
}

const (
	testModel   = "./models/chinese-clip-vit-base-patch16/onnx/model_q4.onnx"
	testDataDir = ".test/data"
)

// safeClose 安全关闭 Indexer
func safeClose(t *testing.T, idx core.Indexer) {
	t.Helper()
	if c, ok := idx.(interface{ Close(context.Context) error }); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		assert.NoError(t, c.Close(ctx))
	}
}

// listDataFiles 列出 .test/data 目录下所有文件
func listDataFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(testDataDir)
	if err != nil {
		t.Fatalf("读取测试数据目录 %s 失败: %v", testDataDir, err)
	}
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		files = append(files, filepath.Join(testDataDir, e.Name()))
	}
	return files
}

// readFirstLines 读取文件前 N 行，用于生成测试查询
func readFirstLines(t *testing.T, path string, n int) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件 %s 失败: %v", path, err)
	}
	lines := strings.SplitN(string(data), "\n", n+1)
	return strings.Join(lines[:min(n, len(lines))], "\n")
}

// extractSearchTerms 从文件内容中提取可用于搜索的关键短语
func extractSearchTerms(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取文件 %s 失败: %v", path, err)
	}
	content := string(data)

	// 按行分割，过滤短行和标题符号，取有意义的短语
	lines := strings.Split(content, "\n")
	var terms []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 去掉 Markdown 标题符号
		line = strings.TrimLeft(line, "# ")
		if len(line) < 4 || len(line) > 50 {
			continue
		}
		terms = append(terms, line)
		if len(terms) >= 5 {
			break
		}
	}
	if len(terms) == 0 {
		t.Fatalf("文件 %s 中未能提取搜索关键词", path)
	}
	return terms
}

// containsChunkID 检查搜索结果中是否包含指定 chunkID
func containsChunkID(hits []core.Hit, chunkID string) bool {
	for _, h := range hits {
		if h.ID == chunkID {
			return true
		}
	}
	return false
}

// =============================================================================
// 测试1: New 方法 —— 检查是否正确初始化
// =============================================================================

func TestNew_HybridIndexer(t *testing.T) {
	dataPath := t.TempDir()

	// 调用 New
	idx, err := New(dataPath,
		WithIndexType("hybrid"),
		WithEmbeddingModelFile(testModel),
		WithName("memory"),
	)
	require.NoError(t, err, "New 应成功创建索引器")
	require.NotNil(t, idx, "返回的 Indexer 不应为 nil")
	defer safeClose(t, idx)

	// 验证类型
	assert.Equal(t, "hybrid", idx.Name(), "Name 应返回 'hybrid'")
	assert.Equal(t, "hybrid", idx.Type(), "Type 应返回 'hybrid'")

	// 验证是否为 *HybridIndexer 实例
	hybrid, ok := idx.(*HybridIndexer)
	require.True(t, ok, "返回的 Indexer 应为 *HybridIndexer 类型")
	assert.NotNil(t, hybrid)

	// 验证数据目录结构已创建
	for _, sub := range []string{"vectors", "graphs", "fulltexts"} {
		dirPath := filepath.Join(dataPath, sub)
		info, err := os.Stat(dirPath)
		require.NoError(t, err, "子目录 %s 应存在", sub)
		assert.True(t, info.IsDir(), "%s 应为目录", sub)
	}

	// 验证配置文件已生成
	configPath := filepath.Join(dataPath, "config.yml")
	configData, err := os.ReadFile(configPath)
	require.NoError(t, err, "config.yml 应存在")
	assert.Contains(t, string(configData), "memory", "配置文件应包含 name")
	assert.Contains(t, string(configData), "hybrid", "配置文件应包含 type")

	// 验证内部索引器列表
	names := hybrid.ListIndexers()
	t.Logf("内部索引器列表: %v", names)
	assert.Contains(t, names, "semantic", "应包含 semantic 索引器")
	assert.Contains(t, names, "fulltext", "应包含 fulltext 索引器")

	// 验证权重
	weights := hybrid.GetWeights()
	assert.Greater(t, weights["semantic"], float32(0), "semantic 权重应 > 0")
	assert.Greater(t, weights["fulltext"], float32(0), "fulltext 权重应 > 0")
	t.Logf("索引器权重: semantic=%.2f, fulltext=%.2f", weights["semantic"], weights["fulltext"])
}

// =============================================================================
// 测试2: Open 方法 —— 自包含测试：先创建再打开
// =============================================================================

func TestOpen_HybridIndexer(t *testing.T) {
	dataPath := t.TempDir()

	// 先创建
	idx, err := New(dataPath,
		WithIndexType("hybrid"),
		WithEmbeddingModelFile(testModel),
		WithName("memory"),
	)
	require.NoError(t, err)
	safeClose(t, idx)

	// 延迟确保锁释放
	time.Sleep(200 * time.Millisecond)

	// 再打开
	idx2, err := Open(dataPath)
	require.NoError(t, err, "Open 应成功恢复索引器")
	require.NotNil(t, idx2, "返回的 Indexer 不应为 nil")
	defer safeClose(t, idx2)

	// 验证恢复后的属性
	assert.Equal(t, "hybrid", idx2.Name(), "Name 应与 New 创建的一致")
	assert.Equal(t, "hybrid", idx2.Type(), "Type 应与 New 创建的一致")

	// 验证恢复后仍为 HybridIndexer 实例
	hybrid, ok := idx2.(*HybridIndexer)
	require.True(t, ok, "恢复的 Indexer 应为 *HybridIndexer 类型")

	// 验证内部索引器列表一致
	names := hybrid.ListIndexers()
	t.Logf("恢复后索引器列表: %v", names)
	assert.Contains(t, names, "semantic")
	assert.Contains(t, names, "fulltext")
}

// =============================================================================
// 测试3: AddFile + Search —— 将 .test/data 目录内文件添加到索引器
// =============================================================================

func TestAddFile_And_Search_AllFiles(t *testing.T) {
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skipf("跳过: 测试数据目录 %s 不存在，请准备测试数据文件", testDataDir)
	}

	dataPath := t.TempDir()

	idx, err := New(dataPath,
		WithIndexType("hybrid"),
		WithEmbeddingModelFile(testModel),
		WithName("memory"),
	)
	require.NoError(t, err)
	defer safeClose(t, idx)

	ctx := context.Background()

	// 列出所有测试数据文件（仅取前 3 个以控制测试时间）
	files := listDataFiles(t)
	require.NotEmpty(t, files, "%s 目录下应有测试数据文件", testDataDir)
	if len(files) > 3 {
		files = files[:3]
	}
	t.Logf("使用 %d 个测试数据文件", len(files))

	// 添加所有文件到索引器
	addedChunks := make(map[string]string) // filePath -> chunkID
	for _, f := range files {
		chunks, err := idx.AddFile(ctx, f)
		require.NoError(t, err, "AddFile(%s) 不应报错", f)
		require.NotEmpty(t, chunks, "AddFile(%s) 应返回非空 chunks", f)
		addedChunks[f] = chunks[0].ID
		t.Logf("已索引: %s -> chunkCount=%d firstChunkID=%s", filepath.Base(f), len(chunks), chunks[0].ID)
	}

	// 对第一个已索引文件进行搜索验证
	for _, f := range files[:1] {
		terms := extractSearchTerms(t, f)
		expectedChunkID := addedChunks[f]

		for _, term := range terms[:min(2, len(terms))] {
			q := idx.NewQuery(term)
			hits, err := idx.Search(ctx, q)
			require.NoError(t, err, "搜索 '%s' 不应报错", term)

			if len(hits) > 0 {
				found := containsChunkID(hits, expectedChunkID)
				t.Logf("[文件=%s] 查询='%s': %d 结果, 命中原文件=%v",
					filepath.Base(f), term, len(hits), found)

				// 验证结果结构完整
				for _, hit := range hits {
					assert.NotEmpty(t, hit.ID, "Hit.ID 不应为空")
					assert.Greater(t, hit.Score, float32(0), "Hit.Score 应 > 0")
				}
			} else {
				t.Logf("[文件=%s] 查询='%s': 无结果 (可能关键词未被分块包含)", filepath.Base(f), term)
			}
		}
	}
}

// =============================================================================
// 测试4: Add + Search —— 添加单个文件内容，验证索引和召回
// =============================================================================

func TestAdd_And_Search_SingleFile(t *testing.T) {
	if _, err := os.Stat(testDataDir); os.IsNotExist(err) {
		t.Skipf("跳过: 测试数据目录 %s 不存在", testDataDir)
	}

	dataPath := t.TempDir()

	idx, err := New(dataPath,
		WithIndexType("hybrid"),
		WithEmbeddingModelFile(testModel),
		WithName("memory"),
	)
	require.NoError(t, err)
	defer safeClose(t, idx)

	ctx := context.Background()

	// 找到第一个 .md 文件作为测试数据
	files := listDataFiles(t)
	var mdFile string
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".md") {
			mdFile = f
			break
		}
	}
	if mdFile == "" {
		if len(files) == 0 {
			t.Skipf("跳过: %s 目录下没有测试数据文件", testDataDir)
		}
		mdFile = files[0]
	}
	t.Logf("使用测试文件: %s", mdFile)

	// 读取文件内容
	content, err := os.ReadFile(mdFile)
	require.NoError(t, err, "读取测试文件不应报错")
	require.NotEmpty(t, content, "测试文件不应为空")

	fileContent := string(content)

	// 使用 Add 方法添加内容
	chunks, err := idx.Add(ctx, fileContent)
	require.NoError(t, err, "Add 不应报错")
	require.NotEmpty(t, chunks, "Add 应返回非空 chunks")
	chunkID := chunks[0].ID
	t.Logf("Add 成功: chunkID=%s, content长度=%d", chunkID, len(chunks[0].Content))

	// 提取搜索关键词
	terms := extractSearchTerms(t, mdFile)
	require.NotEmpty(t, terms, "应能从文件中提取搜索关键词")

	// 使用文件前几行内容作为查询（验证语义召回）
	prefix := readFirstLines(t, mdFile, 3)
	t.Logf("查询内容（文件前几行）: %q", prefix)

	q := idx.NewQuery(prefix)
	hits, err := idx.Search(ctx, q)
	require.NoError(t, err, "搜索不应报错")
	assert.NotEmpty(t, hits, "使用文件前几行内容搜索应有结果")

	t.Logf("搜索 '%s...': %d 个结果", prefix[:min(30, len(prefix))], len(hits))

	// 验证添加的文档被召回
	found := containsChunkID(hits, chunkID)
	assert.True(t, found, "搜索结果应包含刚添加的 chunk (chunkID=%s)", chunkID)

	// 验证搜索结果质量
	for i, hit := range hits[:min(3, len(hits))] {
		t.Logf("  #%d: score=%.4f id=%s", i+1, hit.Score, hit.ID)
		assert.NotEmpty(t, hit.ID)
		assert.Greater(t, hit.Score, float32(0))
	}

	// 使用提取的关键词搜索（最多 2 个）
	for _, term := range terms[:min(2, len(terms))] {
		q := idx.NewQuery(term)
		hits, err := idx.Search(ctx, q)
		require.NoError(t, err, "搜索 '%s' 不应报错", term)

		if len(hits) > 0 {
			found := containsChunkID(hits, chunkID)
			t.Logf("关键词查询 '%s': %d 结果, 命中=%v", term, len(hits), found)
		}
	}
}
