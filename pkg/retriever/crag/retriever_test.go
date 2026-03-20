package crag

import (
	"context"
	"testing"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocks
type mockVectorStore struct{ mock.Mock }

func (m *mockVectorStore) Upsert(ctx context.Context, vectors []*core.Vector) error {
	return m.Called(ctx, vectors).Error(0)
}
func (m *mockVectorStore) Search(ctx context.Context, query []float32, topK int, filters map[string]any) ([]*core.Vector, []float32, error) {
	args := m.Called(ctx, query, topK, filters)
	return args.Get(0).([]*core.Vector), args.Get(1).([]float32), args.Error(2)
}
func (m *mockVectorStore) Delete(ctx context.Context, id string) error { return m.Called(ctx, id).Error(0) }
func (m *mockVectorStore) Close(ctx context.Context) error           { return m.Called(ctx).Error(0) }

type mockEmbedder struct{ mock.Mock }

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	return args.Get(0).([][]float32), args.Error(1)
}
func (m *mockEmbedder) EmbedQuery(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	return args.Get(0).([]float32), args.Error(1)
}
func (m *mockEmbedder) Dimension() int { return m.Called().Int(0) }

type mockLLM struct{ mock.Mock }

func (m *mockLLM) Chat(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Response, error) {
	args := m.Called(ctx, messages, opts)
	return args.Get(0).(*chat.Response), args.Error(1)
}
func (m *mockLLM) ChatStream(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Stream, error) {
	args := m.Called(ctx, messages, opts)
	return args.Get(0).(*chat.Stream), args.Error(1)
}

type mockCRAGEvaluator struct{ mock.Mock }

func (m *mockCRAGEvaluator) Evaluate(ctx context.Context, query *core.Query, chunks []*core.Chunk) (*core.CRAGEvaluation, error) {
	args := m.Called(ctx, query, chunks)
	return args.Get(0).(*core.CRAGEvaluation), args.Error(1)
}

type mockWebSearcher struct{ mock.Mock }

func (m *mockWebSearcher) Search(ctx context.Context, query string, topK int) ([]*core.Chunk, error) {
	args := m.Called(ctx, query, topK)
	return args.Get(0).([]*core.Chunk), args.Error(1)
}

func TestCRAGRetriever_Irrelevant(t *testing.T) {
	mVS := new(mockVectorStore)
	mEmb := new(mockEmbedder)
	mEval := new(mockCRAGEvaluator)
	mWeb := new(mockWebSearcher)
	mLLM := new(mockLLM)

	ctx := context.Background()
	queryText := "What is the capital of Mars?"

	// 1. Vector Search (Initial)
	queryVec := []float32{0.1, 0.2}
	mEmb.On("Embed", ctx, []string{queryText}).Return([][]float32{queryVec}, nil).Once()
	mVS.On("Search", ctx, queryVec, 5, mock.Anything).Return(
		[]*core.Vector{{ID: "vec1", ChunkID: "chunk1"}},
		[]float32{0.5},
		nil,
	).Once()

	// 2. Evaluation (Irrelevant)
	mEval.On("Evaluate", ctx, mock.Anything, mock.Anything).Return(&core.CRAGEvaluation{
		Label: core.CRAGIrrelevant,
		Score: 0.1,
	}, nil).Once()

	// 3. Fallback (Web Search)
	mWeb.On("Search", ctx, queryText, 5).Return([]*core.Chunk{
		{ID: "web1", Content: "Mars has no capital city."},
	}, nil).Once()

	// 4. Generation
	mLLM.On("Chat", ctx, mock.Anything, mock.Anything).Return(&chat.Response{
		Content: "Mars doesn't have a capital.",
	}, nil).Once()

	retriever := NewRetriever(mVS, mEmb, mEval, mLLM, WithWebSearcher(mWeb))
	results, err := retriever.Retrieve(ctx, []string{queryText}, 5)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Mars doesn't have a capital.", results[0].Answer)
	// Verify that chunks were replaced by web results
	assert.Len(t, results[0].Chunks, 1)
	assert.Equal(t, "web1", results[0].Chunks[0].ID)

	mVS.AssertExpectations(t)
	mEmb.AssertExpectations(t)
	mEval.AssertExpectations(t)
	mWeb.AssertExpectations(t)
	mLLM.AssertExpectations(t)
}
