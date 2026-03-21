package selfrag

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
	args := m.Called(ctx, messages)
	return args.Get(0).(*chat.Response), args.Error(1)
}
func (m *mockLLM) ChatStream(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Stream, error) {
	args := m.Called(ctx, messages)
	return args.Get(0).(*chat.Stream), args.Error(1)
}

type mockRAGEvaluator struct{ mock.Mock }

func (m *mockRAGEvaluator) Evaluate(ctx context.Context, query string, answer string, contextStr string) (*core.RAGEvaluation, error) {
	args := m.Called(ctx, query, answer, contextStr)
	return args.Get(0).(*core.RAGEvaluation), args.Error(1)
}

func TestSelfRAGRetriever_Refine(t *testing.T) {
	mVS := new(mockVectorStore)
	mEmb := new(mockEmbedder)
	mEval := new(mockRAGEvaluator)
	mLLM := new(mockLLM)

	ctx := context.Background()
	queryText := "How to bake a cake?"

	// 1. Vector Search
	queryVec := []float32{0.1, 0.2}
	mEmb.On("Embed", mock.Anything, []string{queryText}).Return([][]float32{queryVec}, nil).Once()
	mVS.On("Search", mock.Anything, queryVec, 5, mock.Anything).Return(
		[]*core.Vector{{ID: "vec1", ChunkID: "chunk1"}},
		[]float32{0.9},
		nil,
	).Once()

	// 2. Initial Generation
	mLLM.On("Chat", ctx, mock.Anything).Return(&chat.Response{
		Content: "To bake a cake, you need sugar.",
	}, nil).Once()

	// 3. Evaluation 1 (Fail)
	mEval.On("Evaluate", ctx, queryText, "To bake a cake, you need sugar.", mock.Anything).Return(&core.RAGEvaluation{
		OverallScore: 0.3,
		Passed:       false,
		Feedback:     "Missing flour and eggs.",
	}, nil).Once()

	// 4. Refinement Generation
	mLLM.On("Chat", ctx, mock.Anything).Return(&chat.Response{
		Content: "To bake a cake, you need sugar, flour, and eggs.",
	}, nil).Once()

	// 5. Evaluation 2 (Pass)
	mEval.On("Evaluate", ctx, queryText, "To bake a cake, you need sugar, flour, and eggs.", mock.Anything).Return(&core.RAGEvaluation{
		OverallScore: 0.9,
		Passed:       true,
		Feedback:     "Perfect.",
	}, nil).Once()

	retriever := NewRetriever(mVS, mEmb, mEval, mLLM, WithThreshold(0.8), WithMaxRetries(2))
	results, err := retriever.Retrieve(ctx, []string{queryText}, 5)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "To bake a cake, you need sugar, flour, and eggs.", results[0].Answer)
	assert.Equal(t, float32(0.9), results[0].Metadata["self_rag_score"])

	mVS.AssertExpectations(t)
	mEmb.AssertExpectations(t)
	mEval.AssertExpectations(t)
	mLLM.AssertExpectations(t)
}
