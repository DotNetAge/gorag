package indexer

import (
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/indexing/parser/config/types"
	"github.com/stretchr/testify/require"
)

// mockStatefulParser simulates a parser that checks for thread safety.
type mockStatefulParser struct {
	core.Parser
}

func (m *mockStatefulParser) GetSupportedTypes() []string {
	return []string{".mock"}
}

func (m *mockStatefulParser) ParseStream(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *core.Document, error) {
	docChan := make(chan *core.Document, 1)
	docChan <- &core.Document{Content: "mock content"}
	close(docChan)
	return docChan, nil
}

// AuditStandard_Indexer_Concurrency 审计标准：Indexer 必须能安全地处理极高并发
func TestAuditStandard_Indexer_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a custom registry to enforce factory pattern
	registry := types.NewParserRegistry()
	registry.Register(func() core.Parser { return &mockStatefulParser{} })

	idxIface, err := DefaultIndexer(
		WithConcurrency(true),
		WithWorkers(100),
		WithGoVector("test", filepath.Join(tmpDir, "vectors.db"), 1536),
		WithSQLDoc(filepath.Join(tmpDir, "docs.db")),
	)
	require.NoError(t, err)
	idx := idxIface.(*defaultIndexer)
	idx.registry = registry // override for test
	require.NoError(t, idx.Init())

	// Start 100 concurrent indexing requests to the same indexer instance
	var wg sync.WaitGroup
	const numRequests = 100
	wg.Add(numRequests)

	for i := 0; i < numRequests; i++ {
		go func(reqID int) {
			defer wg.Done()
			// We mock a file path to hit our mock parser
			file := filepath.Join(tmpDir, "test.mock")
			_, _ = idx.IndexFile(context.Background(), file) // Ignore errors, we just want to catch panics/race conditions
		}(i)
	}

	wg.Wait()
	// If it didn't panic with concurrent map writes or deadlock, it passes.
}
