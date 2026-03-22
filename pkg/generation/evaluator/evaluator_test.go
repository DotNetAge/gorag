package evaluation

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
	chat "github.com/DotNetAge/gochat/pkg/core"
)

type mockChatClient struct {
	chatFn func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error)
}

func (m *mockChatClient) Chat(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, messages, options...)
	}
	return &chat.Response{Content: "mock response"}, nil
}

func (m *mockChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "Generated text", nil
}

func (m *mockChatClient) ChatStream(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Stream, error) {
	return nil, nil
}

func TestRAGEvaluator_New(t *testing.T) {
	mockLLM := &mockChatClient{}
	evaluator := NewRAGEvaluator(mockLLM)
	assert.NotNil(t, evaluator)
}

func TestRAGEvaluator_Evaluate(t *testing.T) {
	mockLLM := &mockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			return &chat.Response{Content: "Evaluation complete"}, nil
		},
	}
	evaluator := NewRAGEvaluator(mockLLM)

	result, err := evaluator.Evaluate(context.Background(), "test query", "test answer", "test context")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, float32(0.8), result.Faithfulness)
	assert.Equal(t, float32(0.8), result.Relevance)
	assert.Equal(t, float32(0.8), result.OverallScore)
	assert.True(t, result.Passed)
}

func TestRAGEvaluator_Evaluate_LLMError(t *testing.T) {
	mockLLM := &mockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			return nil, errors.New("LLM error")
		},
	}
	evaluator := NewRAGEvaluator(mockLLM)

	result, err := evaluator.Evaluate(context.Background(), "test query", "test answer", "test context")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestCRAGEvaluator_New(t *testing.T) {
	mockLLM := &mockChatClient{}
	evaluator := NewCRAGEvaluator(mockLLM)
	assert.NotNil(t, evaluator)
}

func TestCRAGEvaluator_Evaluate(t *testing.T) {
	mockLLM := &mockChatClient{}
	evaluator := NewCRAGEvaluator(mockLLM)

	chunks := []*core.Chunk{
		{ID: "chunk1", Content: "test content"},
	}
	query := core.NewQuery("1", "test query", nil)

	result, err := evaluator.Evaluate(context.Background(), query, chunks)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, core.CRAGRelevant, result.Label)
	assert.Equal(t, float32(0.9), result.Score)
}

func TestCRAGEvaluator_Evaluate_LLMClientNotUsed(t *testing.T) {
	mockLLM := &mockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			t.Error("LLM should not be called for CRAG evaluation")
			return nil, errors.New("should not be called")
		},
	}
	evaluator := NewCRAGEvaluator(mockLLM)

	chunks := []*core.Chunk{
		{ID: "chunk1", Content: "test content"},
	}
	query := core.NewQuery("1", "test query", nil)

	result, err := evaluator.Evaluate(context.Background(), query, chunks)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestBenchmarkResult_Summary(t *testing.T) {
	result := &BenchmarkResult{
		TotalCases:      10,
		AvgFaithfulness: 0.85,
		AvgRelevance:    0.90,
		AvgPrecision:    0.88,
		TotalDuration:   1000000000,
	}

	summary := result.Summary()

	assert.Contains(t, summary, "Cases: 10")
	assert.Contains(t, summary, "Avg Faithfulness: 0.85")
	assert.Contains(t, summary, "Avg Relevance: 0.90")
	assert.Contains(t, summary, "Avg Precision: 0.88")
}

type mockRetrieverForBenchmark struct {
	results []*core.RetrievalResult
	err     error
}

func (m *mockRetrieverForBenchmark) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.results, nil
}

func TestRunBenchmark(t *testing.T) {
	mockRetriever := &mockRetrieverForBenchmark{
		results: []*core.RetrievalResult{
			{
				Query:  "test query",
				Answer: "test answer",
				Chunks: []*core.Chunk{
					{ID: "chunk1", Content: "test content"},
				},
			},
		},
	}
	mockLLM := &mockChatClient{}
	judge := NewRagasLLMJudge(mockLLM)

	cases := []TestCase{
		{Query: "test query", GroundTruth: "test answer"},
	}

	result, err := RunBenchmark(context.Background(), mockRetriever, judge, cases, 5)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalCases)
}
