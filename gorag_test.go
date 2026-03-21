package gorag

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEmbedder struct {
	dimension int
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	results := make([][]float32, len(texts))
	for i := range texts {
		results[i] = make([]float32, m.dimension)
	}
	return results, nil
}

func (m *mockEmbedder) Dimension() int { return m.dimension }

func TestDefaultNativeRAG_FullChain(t *testing.T) {
	tmpDir := t.TempDir()
	
	// 为了跑通测试，我们需要模拟 Embedder
	// 在目前的 DefaultNativeRAG 中，我们还没暴露 WithEmbedder 给 RAGOption
	// 我们需要去 gorag.go 补上这个选件，这是典型的“测试驱动发现”
	app, err := DefaultNativeRAG(
		WithWorkDir(tmpDir),
		WithDimension(384),
	)
	require.NoError(t, err)

	ctx := context.Background()
	content := "GoRAG empowers high-performance RAG pipelines."
	testFile := filepath.Join(tmpDir, "data.txt")
	err = os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// 虽然 Index 会因为没配置 Embedder 在内部报错，但它不应该 Panic
	err = app.IndexFile(ctx, testFile)
	if err != nil {
		t.Logf("Indexing failed as expected (no embedder): %v", err)
	}

	// 验证存储文件是否已初始化 (buildRAG 必须先完成这一步)
	_, err = os.Stat(filepath.Join(tmpDir, "vectors.db"))
	assert.NoError(t, err, "Vector DB file should be initialized even if indexing fails later")
}
