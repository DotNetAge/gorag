package enhancer

import (
	"context"
	"testing"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
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
		Content: "Rewritten query",
	}
	return response, nil
}

func (m *MockChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "Generated text", nil
}

func (m *MockChatClient) ChatStream(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Stream, error) {
	// Return nil for now
	return nil, nil
}

func TestQueryRewriter_Rewrite(t *testing.T) {
	// Create a mock chat client
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "Rewritten test query",
			}
			return response, nil
		},
	}

	// Create a query rewriter
	rewriter := NewQueryRewriter(mockChat)

	// Create a test query
	query := entity.NewQuery("test-query", "test query", map[string]any{})

	// Test Rewrite method
	ctx := context.Background()
	rewrittenQuery, err := rewriter.Rewrite(ctx, query)

	// Check for errors
	assert.NoError(t, err)

	// Check that query was rewritten
	assert.NotEqual(t, query.Text, rewrittenQuery.Text)
	assert.Equal(t, "Rewritten test query", rewrittenQuery.Text)

	// Check that metadata was updated
	assert.Equal(t, "test query", rewrittenQuery.Metadata["original_query"])
	assert.True(t, rewrittenQuery.Metadata["is_rewritten"].(bool))
}

func TestQueryRewriter_Rewrite_EmptyResponse(t *testing.T) {
	// Create a mock chat client that returns empty response
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "",
			}
			return response, nil
		},
	}

	// Create a query rewriter
	rewriter := NewQueryRewriter(mockChat)

	// Create a test query
	query := entity.NewQuery("test-query", "test query", map[string]any{})

	// Test Rewrite method
	ctx := context.Background()
	rewrittenQuery, err := rewriter.Rewrite(ctx, query)

	// Check for errors
	assert.NoError(t, err)

	// Check that original query is used as fallback
	assert.Equal(t, query.Text, rewrittenQuery.Text)
}

func TestHyDEGenerator_GenerateHypotheticalDocument(t *testing.T) {
	// Create a mock chat client
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "This is a hypothetical document about the test query.",
			}
			return response, nil
		},
	}

	// Create a HyDE generator
	generator := NewHyDEGenerator(mockChat)

	// Create a test query
	query := entity.NewQuery("test-query", "test query", map[string]any{})

	// Test GenerateHypotheticalDocument method
	ctx := context.Background()
	doc, err := generator.GenerateHypotheticalDocument(ctx, query)

	// Check for errors
	assert.NoError(t, err)

	// Check that document was generated
	assert.NotEmpty(t, doc.ID)
	assert.Equal(t, "This is a hypothetical document about the test query.", doc.Content)
	assert.Equal(t, "hyde_generator", doc.Source)
	assert.Equal(t, "text/plain", doc.ContentType)
	assert.Equal(t, "test-query", doc.Metadata["generated_for"])
}

func TestStepBackGenerator_GenerateStepBackQuery(t *testing.T) {
	// Create a mock chat client
	mockChat := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			response := &chat.Response{
				Content: "What are the underlying principles of the test query?",
			}
			return response, nil
		},
	}

	// Create a step-back generator
	generator := NewStepBackGenerator(mockChat)

	// Create a test query
	query := entity.NewQuery("test-query", "test query", map[string]any{})

	// Test GenerateStepBackQuery method
	ctx := context.Background()
	stepBackQuery, err := generator.GenerateStepBackQuery(ctx, query)

	// Check for errors
	assert.NoError(t, err)

	// Check that step-back query was generated
	assert.NotEqual(t, query.Text, stepBackQuery.Text)
	assert.Equal(t, "What are the underlying principles of the test query?", stepBackQuery.Text)
	assert.Equal(t, "test query", stepBackQuery.Metadata["step_back_for"])
}
