package indexer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DotNetAge/gorag/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 集成测试：FulltextIndexer 建立全文索引 & 查询索引内文本内容
// 无需任何环境变量，bleve 为纯 Go 内嵌搜索引擎
// 测试数据: ../.test/data 目录下的 markdown 文件
// =============================================================================

const fulltextTestDataDir = "../.test/data"

// pickMdFiles 从测试数据目录中取所有 .md 文件
func pickMdFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(fulltextTestDataDir)
	require.NoError(t, err, "读取测试数据目录失败")
	files := make([]string, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			files = append(files, filepath.Join(fulltextTestDataDir, e.Name()))
		}
	}
	require.NotEmpty(t, files, "测试数据目录中应有至少 1 个 .md 文件")
	return files
}

// =============================================================================
// 测试1: Add — 索引文本内容并验证返回 chunk
// 期待: Add 成功返回非 nil chunk，chunk.ID 和 chunk.Content 非空
// =============================================================================
func TestFulltextIndexer_Add(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_add_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err, "创建 FulltextIndexer 失败")

	ctx := context.Background()
	content := "人工智能是计算机科学的一个分支，致力于创建能够模拟人类智能的系统。"
	chunks, err := idx.Add(ctx, content)
	require.NoError(t, err, "Add 不应报错")
	require.NotEmpty(t, chunks, "Add 应返回非空 chunks")
	assert.NotEmpty(t, chunks[0].ID, "chunk.ID 不应为空")
	assert.NotEmpty(t, chunks[0].Content, "chunk.Content 不应为空")
	assert.Contains(t, chunks[0].Content, "人工智能", "chunk.Content 应包含原文内容")
}

// =============================================================================
// 测试2: Add 空内容 — 应返回错误
// =============================================================================
func TestFulltextIndexer_Add_Empty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_empty_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	chunks, err := idx.Add(ctx, "")
	require.Error(t, err, "空内容应报错")
	assert.Nil(t, chunks)
}

// =============================================================================
// 测试3: AddFile — 索引文件并验证 chunk
// 期待: 使用 .test/data 中的文件，AddFile 成功返回非 nil chunk
// =============================================================================
func TestFulltextIndexer_AddFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_file_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	files := pickMdFiles(t)

	fileCount := 0
	for _, f := range files[:3] { // 取前 3 个文件测试
		absPath, err := filepath.Abs(f)
		require.NoError(t, err)

		chunks, err := idx.AddFile(ctx, absPath)
		require.NoError(t, err, "AddFile(%s) 不应报错", f)
		require.NotEmpty(t, chunks, "AddFile(%s) 应返回非 nil chunk", f)
		t.Logf("已索引: %s -> chunkCount=%d firstChunkID=%s", filepath.Base(f), len(chunks), chunks[0].ID)
		fileCount++
	}
	assert.Equal(t, 3, fileCount, "应成功索引 3 个文件")
}

// =============================================================================
// 测试4: AddFile 不存在的文件 — 应返回错误
// =============================================================================
func TestFulltextIndexer_AddFile_NotFound(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_notfound_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = idx.AddFile(ctx, "/nonexistent/path/to/file.md")
	require.Error(t, err, "不存在的文件应报错")
}

// =============================================================================
// 测试5: Search — 索引文件后全文搜索
// 期待: 搜索关键词能命中已索引的文档片段
// =============================================================================
func TestFulltextIndexer_Search(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_search_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	files := pickMdFiles(t)

	// 索引前 3 个文件，收集所有文件内容用于构造查询词
	var allContent strings.Builder
	for _, f := range files[:min(3, len(files))] {
		absPath, err := filepath.Abs(f)
		require.NoError(t, err)

		chunks, err := idx.AddFile(ctx, absPath)
		require.NoError(t, err, "AddFile(%s) 不应报错", f)
		if len(chunks) > 0 {
			allContent.WriteString(chunks[0].Content)
		}
	}

	// 确保有索引内容
	require.NotEmpty(t, allContent.String(), "索引后应有内容")

	// 构造查询: 取内容中的一个有意义的片段
	content := allContent.String()
	queryText := content
	if len([]rune(queryText)) > 50 {
		queryText = string([]rune(queryText)[:50])
	}

	q := idx.NewQuery(queryText)
	hits, err := idx.Search(ctx, q)
	require.NoError(t, err, "Search 不应报错")
	require.NotEmpty(t, hits, "Search 应返回至少 1 个 Hit")

	t.Logf("查询: %q", queryText)
	t.Logf("Search 返回 %d 个 Hit", len(hits))
	for i, hit := range hits {
		t.Logf("  Hit #%d: ID=%s Score=%.4f DocID=%s", i+1, hit.ID, hit.Score, hit.DocID)
		assert.NotEmpty(t, hit.ID, "Hit.ID 不应为空")
		assert.Greater(t, hit.Score, float32(0), "Hit.Score 应 > 0")
	}
}

// =============================================================================
// 测试6: Search 空查询 — 应返回 nil 而非报错
// =============================================================================
func TestFulltextIndexer_Search_Empty(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_search_empty_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	q := idx.NewQuery("")
	hits, err := idx.Search(ctx, q)
	require.NoError(t, err)
	assert.Nil(t, hits, "空查询应返回 nil")
}

// =============================================================================
// 测试7: Remove — 删除已索引的 chunk 后搜索不再命中
// =============================================================================
func TestFulltextIndexer_Remove(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_remove_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	chunks, err := idx.Add(ctx, "这是一段独特的测试文本，用于验证删除功能。雪花算法生成唯一标识符。")
	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	// 先搜索确认已索引
	q := idx.NewQuery("雪花算法")
	hits, err := idx.Search(ctx, q)
	require.NoError(t, err)
	assert.NotEmpty(t, hits, "删除前应能搜索到结果")

	// 删除
	err = idx.Remove(ctx, chunks[0].ID)
	require.NoError(t, err, "Remove 不应报错")

	// 搜索同一关键词，该 chunk 不应再命中
	hitsAfter, err := idx.Search(ctx, q)
	require.NoError(t, err)
	for _, hit := range hitsAfter {
		assert.NotEqual(t, chunks[0].ID, hit.ID, "删除后该 chunk 不应再出现")
	}
}

// =============================================================================
// 测试8: Name / Type — 验证元信息
// =============================================================================
func TestFulltextIndexer_MetaData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_meta_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	assert.Equal(t, "fulltext", idx.Name())
	assert.Equal(t, "fulltext", idx.Type())
}

// =============================================================================
// 测试9: NewQuery — 验证查询构造
// =============================================================================
func TestFulltextIndexer_NewQuery(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_query_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	q := idx.NewQuery("知识图谱")
	require.NotNil(t, q)
	assert.Equal(t, "知识图谱", q.Raw())
}

// =============================================================================
// 测试10: 索引全部测试数据后搜索
// 期待: 全量索引 .test/data 所有文件后，中文关键词搜索能命中
// =============================================================================
func TestFulltextIndexer_FullDataset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full dataset test in short mode")
	}
	dbPath := filepath.Join(t.TempDir(), "fulltext_full_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()
	files := pickMdFiles(t)
	t.Logf("共发现 %d 个测试文件", len(files))

	// 限制文件数以控制测试时间
	maxFiles := 5
	if len(files) > maxFiles {
		files = files[:maxFiles]
	}
	fileCount := 0
	for _, f := range files {
		absPath, err := filepath.Abs(f)
		require.NoError(t, err)

		chunks, err := idx.AddFile(ctx, absPath)
		if err != nil {
			t.Logf("跳过文件 %s: %v", filepath.Base(f), err)
			continue
		}
		if len(chunks) > 0 {
			fileCount++
		}
	}
	t.Logf("成功索引 %d 个文件的首个 chunk", fileCount)
	require.Greater(t, fileCount, 0, "应至少索引 1 个文件")

	// 使用通用关键词搜索
	keywords := []string{"RAG", "向量", "检索", "模型"}
	foundAny := false
	for _, kw := range keywords {
		q := idx.NewQuery(kw)
		hits, err := idx.Search(ctx, q)
		require.NoError(t, err, "Search(%q) 不应报错", kw)
		if len(hits) > 0 {
			foundAny = true
			t.Logf("关键词 %q: 命中 %d 条", kw, len(hits))
			for i, hit := range hits[:min(3, len(hits))] {
				t.Logf("  #%d: Score=%.4f DocID=%s Content=%q...", i+1, hit.Score, hit.DocID, truncate(hit.Content, 80))
			}
		} else {
			t.Logf("关键词 %q: 无命中", kw)
		}
	}
	assert.True(t, foundAny, "至少一个关键词应命中结果")
}

// =============================================================================
// 测试11: SafeFulltextIndexer — 线程安全包装器
// =============================================================================
func TestSafeFulltextIndexer(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fulltext_safe_test")
	idx, err := NewSafeFulltextIndexer(dbPath)
	require.NoError(t, err)

	assert.Equal(t, "fulltext", idx.Name())
	assert.Equal(t, "fulltext", idx.Type())

	ctx := context.Background()
	chunks, err := idx.Add(ctx, "线程安全的全文索引器测试。")
	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	q := idx.NewQuery("线程安全")
	hits, err := idx.Search(ctx, q)
	require.NoError(t, err)
	assert.NotEmpty(t, hits, "搜索应命中结果")
}

// =============================================================================
// 测试12: 接口合规性 — 确保 FulltextStore 接口被正确实现
// =============================================================================
func TestFulltextIndexer_Interfaces(t *testing.T) {
	// 编译期检查已在 fulltext.go 中通过 var _ 断言
	// 这里验证运行时行为: Indexer 和 ChunkIndexer 接口
	var _ core.Indexer = (*fulltextIndexer)(nil)
	var _ core.ChunkIndexer = (*fulltextIndexer)(nil)

	dbPath := filepath.Join(t.TempDir(), "fulltext_iface_test")
	idx, err := NewFulltextIndexerWithFile(dbPath)
	require.NoError(t, err)

	ctx := context.Background()

	// 测试 IndexChunk
	chunks, err := GetChunks("接口合规性验证文本内容。")
	require.NoError(t, err)
	require.NotEmpty(t, chunks)

	err = idx.IndexChunk(ctx, chunks[0])
	require.NoError(t, err, "IndexChunk 不应报错")

	// 测试 IndexChunks (批量) — 需要通过 ChunkIndexer 接口调用
	ci, ok := idx.(core.ChunkIndexer)
	require.True(t, ok, "应实现 ChunkIndexer 接口")
	moreChunks, err := GetChunks("批量索引测试内容，包含多个段落。")
	require.NoError(t, err)
	err = ci.IndexChunks(ctx, moreChunks)
	require.NoError(t, err, "IndexChunks 不应报错")
}

// truncate 截断字符串用于日志输出
func truncate(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
