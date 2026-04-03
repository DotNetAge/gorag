package selfrag_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/retriever/selfrag"
	"github.com/DotNetAge/gorag/pkg/store/doc/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLLM locally defined to avoid dependency issues
type mockLLM struct{ mock.Mock }

func (m *mockLLM) Chat(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Response, error) {
	args := m.Called(ctx, messages)
	return args.Get(0).(*chat.Response), args.Error(1)
}
func (m *mockLLM) ChatStream(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Stream, error) {
	return nil, nil
}

// MockVectorStore locally defined
type mockVectorStore struct{ mock.Mock }

func (m *mockVectorStore) Upsert(ctx context.Context, vectors []*core.Vector) error {
	return nil
}
func (m *mockVectorStore) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	args := m.Called(ctx, query, topK, filters)
	return args.Get(0).([]*core.Vector), args.Get(1).([]float32), args.Error(2)
}
func (m *mockVectorStore) Delete(ctx context.Context, id string) error { return nil }
func (m *mockVectorStore) Close(ctx context.Context) error             { return nil }

// MockEmbedder locally defined
type mockEmbedder struct{ mock.Mock }

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	return args.Get(0).([][]float32), args.Error(1)
}
func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	return args.Get(0).([]float32), args.Error(1)
}
func (m *mockEmbedder) Dimension() int { return 1536 }

// Integration Evaluator
type integrationEvaluator struct {
	called int
}

func (e *integrationEvaluator) Evaluate(ctx context.Context, query string, answer string, contextStr string) (*core.RAGEvaluation, error) {
	e.called++
	if e.called == 1 {
		return &core.RAGEvaluation{
			OverallScore: 0.4,
			Passed:       false,
			Feedback:     "Too brief.",
		}, nil
	}
	return &core.RAGEvaluation{
		OverallScore: 0.9,
		Passed:       true,
		Feedback:     "Perfect.",
	}, nil
}

func TestSelfRAGWithSQLiteEnrichment(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_docstore.db")

	// 1. Setup SQLite DocStore
	docStore, err := sqlite.NewDocStore(dbPath)
	require.NoError(t, err)
	defer os.Remove(dbPath)

	// 2. Prepare Data
	docID := "doc_001"
	fullContent := "GoRAG is a high-performance RAG framework written in Go. Created in 2024."

	doc := &core.Document{
		ID:      docID,
		Content: fullContent,
	}
	require.NoError(t, docStore.SetDocument(ctx, doc))

	// 3. Setup Mocks
	mLLM := new(mockLLM)
	mVS := new(mockVectorStore)
	mEmb := new(mockEmbedder)

	queryText := "Tell me more about GoRAG."
	queryVec := []float32{0.1}

	// Use mock.Anything for context to avoid cancelCtx mismatches
	mEmb.On("Embed", mock.Anything, []string{queryText}).Return([][]float32{queryVec}, nil)
	mVS.On("Search", mock.Anything, queryVec, 5, mock.Anything).Return(
		[]*core.Vector{{ID: "v1", ChunkID: "chunk_001", Metadata: map[string]any{"document_id": docID, "text": "GoRAG is a framework."}}},
		[]float32{0.9},
		nil,
	)

	// First response
	mLLM.On("Chat", mock.Anything, mock.Anything).Return(&chat.Response{
		Content: "GoRAG is just a framework.",
	}, nil).Once()

	// Second response
	mLLM.On("Chat", mock.Anything, mock.Anything).Return(&chat.Response{
		Content: "GoRAG is a high-performance framework created in 2024.",
	}, nil).Once()

	// 4. Initialize Self-RAG
	evaluator := &integrationEvaluator{}
	retriever := selfrag.NewRetriever(
		mVS,
		mEmb,
		evaluator,
		mLLM,
		selfrag.WithDocStore(docStore),
		selfrag.WithThreshold(0.8),
		selfrag.WithMaxRetries(2),
	)

	// 5. Execute
	resp, err := retriever.Retrieve(ctx, []string{queryText}, 5)

	// 6. Assertions
	require.NoError(t, err)
	assert.Len(t, resp, 1)
	assert.Contains(t, resp[0].Answer, "2024")
	assert.Equal(t, 2, evaluator.called)
}
