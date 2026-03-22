package rerank

import (
	"context"
	"errors"
	"testing"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMClient struct {
	responseContent string
	responseError   error
}

func (m *mockLLMClient) Chat(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Response, error) {
	if m.responseError != nil {
		return nil, m.responseError
	}
	return &chat.Response{Content: m.responseContent}, nil
}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	if m.responseError != nil {
		return "", m.responseError
	}
	return m.responseContent, nil
}

func (m *mockLLMClient) ChatStream(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Stream, error) {
	return nil, nil
}

func TestEnhance_Success(t *testing.T) {
	llm := &mockLLMClient{responseContent: `[0.8, 0.3, 0.9, 0.1]`}
	reranker := NewCrossEncoder(llm, WithRerankTopK(4))

	chunks := []*core.Chunk{
		{ID: "c1", Content: "Go is a programming language"},
		{ID: "c2", Content: "Python is a programming language"},
		{ID: "c3", Content: "Go has goroutines"},
		{ID: "c4", Content: "Java is enterprise software"},
	}

	query := &core.Query{Text: "Tell me about Go programming"}

	enhanced, err := reranker.Enhance(context.Background(), query, chunks)

	assert.NoError(t, err)
	assert.Len(t, enhanced, 4)
	assert.Equal(t, "c3", enhanced[0].ID)
	assert.Equal(t, "c1", enhanced[1].ID)
	assert.Equal(t, "c2", enhanced[2].ID)
	assert.Equal(t, "c4", enhanced[3].ID)
}

func TestEnhance_EmptyChunks(t *testing.T) {
	llm := &mockLLMClient{responseContent: `[0.8, 0.3, 0.9, 0.1]`}
	reranker := NewCrossEncoder(llm, WithRerankTopK(4))

	query := &core.Query{Text: "Tell me about Go programming"}

	enhanced, err := reranker.Enhance(context.Background(), query, []*core.Chunk{})

	assert.NoError(t, err)
	assert.Len(t, enhanced, 0)
}

func TestEnhance_NilQuery(t *testing.T) {
	llm := &mockLLMClient{responseContent: `[0.8, 0.3]`}
	reranker := NewCrossEncoder(llm, WithRerankTopK(2))

	chunks := []*core.Chunk{
		{ID: "c1", Content: "Go is a programming language"},
		{ID: "c2", Content: "Python is a programming language"},
	}

	enhanced, err := reranker.Enhance(context.Background(), nil, chunks)

	assert.Error(t, err)
	assert.Nil(t, enhanced)
}

func TestEnhance_LLMError(t *testing.T) {
	llm := &mockLLMClient{responseError: errors.New("LLM error")}
	reranker := NewCrossEncoder(llm, WithRerankTopK(2))

	chunks := []*core.Chunk{
		{ID: "c1", Content: "Go is a programming language"},
		{ID: "c2", Content: "Python is a programming language"},
	}

	query := &core.Query{Text: "Tell me about Go programming"}

	enhanced, err := reranker.Enhance(context.Background(), query, chunks)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM call failed")
	assert.Nil(t, enhanced)
}

func TestEnhance_TopKLimit(t *testing.T) {
	llm := &mockLLMClient{responseContent: `[0.8, 0.3, 0.9, 0.1, 0.5, 0.7]`}
	reranker := NewCrossEncoder(llm, WithRerankTopK(3))

	chunks := []*core.Chunk{
		{ID: "c1", Content: "Go is a programming language"},
		{ID: "c2", Content: "Python is a programming language"},
		{ID: "c3", Content: "Go has goroutines"},
		{ID: "c4", Content: "Java is enterprise software"},
		{ID: "c5", Content: "Go concurrency model"},
		{ID: "c6", Content: "Rust is systems programming"},
	}

	query := &core.Query{Text: "Tell me about Go programming"}

	enhanced, err := reranker.Enhance(context.Background(), query, chunks)

	assert.NoError(t, err)
	assert.Len(t, enhanced, 3)
	assert.Equal(t, "c3", enhanced[0].ID)
	assert.Equal(t, "c1", enhanced[1].ID)
	assert.Equal(t, "c6", enhanced[2].ID)
}

func TestEnhance_SortOrder(t *testing.T) {
	llm := &mockLLMClient{responseContent: `[0.1, 0.2, 0.3, 0.4]`}
	reranker := NewCrossEncoder(llm, WithRerankTopK(4))

	chunks := []*core.Chunk{
		{ID: "c1", Content: "Least relevant"},
		{ID: "c2", Content: "Second least relevant"},
		{ID: "c3", Content: "Third relevant"},
		{ID: "c4", Content: "Most relevant"},
	}

	query := &core.Query{Text: "Test query"}

	enhanced, err := reranker.Enhance(context.Background(), query, chunks)

	assert.NoError(t, err)
	assert.Len(t, enhanced, 4)
	assert.Equal(t, "c4", enhanced[0].ID)
	assert.Equal(t, "c3", enhanced[1].ID)
	assert.Equal(t, "c2", enhanced[2].ID)
	assert.Equal(t, "c1", enhanced[3].ID)
}