package evaluation

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"testing"
	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/stretchr/testify/assert"
)

// MockChatClient is a mock implementation of chat.Client
type MockChatClient struct {
	chatFn func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error)
}

func (m *MockChatClient) Chat(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, messages, options...)
	}
	// Return a dummy response
	response := &chat.Response{
		Content: "Score: 0.8\nReason: Test reason",
	}
	return response, nil
}

func (m *MockChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "Generated text", nil
}

func (m *MockChatClient) ChatStream(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Stream, error) {
	return nil, nil
}

func TestRagasLLMJudge_New(t *testing.T) {
	// Create a mock chat client
	mockChat := &MockChatClient{}

	// Test creating a new RagasLLMJudge
	judge := NewRagasLLMJudge(mockChat)
	assert.NotNil(t, judge)
	assert.NotNil(t, judge.judgeLLM)
}

func TestRagasLLMJudge_EvaluateFaithfulness(t *testing.T) {
	// Create a mock chat client that returns a specific response
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "Score: 0.9\nReason: The answer is fully faithful to the context",
			}
			return response, nil
		},
	}

	// Create a RagasLLMJudge
	judge := NewRagasLLMJudge(mockChat)

	// Create test data
	query := "What is the capital of France?"
	chunks := []*core.Chunk{
		{
			ID:      "chunk1",
			Content: "Paris is the capital of France and the largest city in the country.",
		},
	}
	answer := "Paris is the capital of France."

	// Test EvaluateFaithfulness method
	ctx := context.Background()
	score, reason, err := judge.EvaluateFaithfulness(ctx, query, chunks, answer)

	// Check for errors
	assert.NoError(t, err)

	// Check the result
	assert.InDelta(t, 0.9, score, 0.000001)
	assert.Equal(t, "The answer is fully faithful to the context", reason)
}

func TestRagasLLMJudge_EvaluateAnswerRelevance(t *testing.T) {
	// Create a mock chat client that returns a specific response
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "Score: 1.0\nReason: The answer directly addresses the query",
			}
			return response, nil
		},
	}

	// Create a RagasLLMJudge
	judge := NewRagasLLMJudge(mockChat)

	// Create test data
	query := "What is the capital of France?"
	answer := "Paris is the capital of France."

	// Test EvaluateAnswerRelevance method
	ctx := context.Background()
	score, reason, err := judge.EvaluateAnswerRelevance(ctx, query, answer)

	// Check for errors
	assert.NoError(t, err)

	// Check the result
	assert.InDelta(t, 1.0, score, 0.000001)
	assert.Equal(t, "The answer directly addresses the query", reason)
}

func TestRagasLLMJudge_EvaluateContextPrecision(t *testing.T) {
	// Create a mock chat client that returns a specific response
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "Score: 0.8\nReason: The context contains relevant information",
			}
			return response, nil
		},
	}

	// Create a RagasLLMJudge
	judge := NewRagasLLMJudge(mockChat)

	// Create test data
	query := "What is the capital of France?"
	chunks := []*core.Chunk{
		{
			ID:      "chunk1",
			Content: "Paris is the capital of France and the largest city in the country.",
		},
		{
			ID:      "chunk2",
			Content: "France is a country in Europe.",
		},
	}

	// Test EvaluateContextPrecision method
	ctx := context.Background()
	score, reason, err := judge.EvaluateContextPrecision(ctx, query, chunks)

	// Check for errors
	assert.NoError(t, err)

	// Check the result
	assert.InDelta(t, 0.8, score, 0.000001)
	assert.Equal(t, "The context contains relevant information", reason)
}

func TestRagasLLMJudge_ParseEvalResponse(t *testing.T) {
	// Create a mock chat client that returns a specific response
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "Score: 0.75\nReason: This is a test reason",
			}
			return response, nil
		},
	}

	// Create a RagasLLMJudge
	judge := NewRagasLLMJudge(mockChat)

	// Test parseEvalResponse method by calling it through one of the public methods
	ctx := context.Background()
	score, reason, err := judge.EvaluateAnswerRelevance(ctx, "Test query", "Test answer")

	// Check for errors
	assert.NoError(t, err)

	// Check the result
	assert.InDelta(t, 0.75, score, 0.000001)
	assert.Equal(t, "This is a test reason", reason)
}

func TestRagasLLMJudge_ParseEvalResponse_InvalidScore(t *testing.T) {
	// Create a mock chat client that returns a response with invalid score
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "Score: invalid\nReason: This is a test reason",
			}
			return response, nil
		},
	}

	// Create a RagasLLMJudge
	judge := NewRagasLLMJudge(mockChat)

	// Test parseEvalResponse method by calling it through one of the public methods
	ctx := context.Background()
	score, reason, err := judge.EvaluateAnswerRelevance(ctx, "Test query", "Test answer")

	// Check for errors
	assert.NoError(t, err)

	// Check the result (should default to 0.0 for invalid score)
	assert.InDelta(t, 0.0, score, 0.000001)
	assert.Equal(t, "This is a test reason", reason)
}

func TestRagasLLMJudge_ParseEvalResponse_NoReason(t *testing.T) {
	// Create a mock chat client that returns a response without reason
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "Score: 0.8",
			}
			return response, nil
		},
	}

	// Create a RagasLLMJudge
	judge := NewRagasLLMJudge(mockChat)

	// Test parseEvalResponse method by calling it through one of the public methods
	ctx := context.Background()
	score, reason, err := judge.EvaluateAnswerRelevance(ctx, "Test query", "Test answer")

	// Check for errors
	assert.NoError(t, err)

	// Check the result (should use default reason)
	assert.InDelta(t, 0.8, score, 0.000001)
	assert.Equal(t, "Could not parse reason", reason)
}

func TestBuildContextText(t *testing.T) {
	// Create test chunks
	chunks := []*core.Chunk{
		{
			ID:      "chunk1",
			Content: "First chunk content",
		},
		{
			ID:      "chunk2",
			Content: "Second chunk content",
		},
	}

	// Test buildContextText function
	contextText := buildContextText(chunks)

	// Check the result
	expected := "\n--- Chunk 1 ---\nFirst chunk content\n\n--- Chunk 2 ---\nSecond chunk content\n"
	assert.Equal(t, expected, contextText)
}
