package pre_retrieval

import (
	"context"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/stretchr/testify/assert"
)

// MockLLM is a mock implementation of core.Client
type MockLLM struct {
	chatFn func(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error)
}

func (m *MockLLM) Chat(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
	if m.chatFn != nil {
		return m.chatFn(ctx, messages, opts...)
	}
	return &core.Response{Content: "Rewritten query"}, nil
}

func (m *MockLLM) ChatStream(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Stream, error) {
	ch := make(chan core.StreamEvent, 1)
	go func() {
		ch <- core.StreamEvent{
			Type:    core.EventContent,
			Content: "Rewritten query",
		}
		close(ch)
	}()
	return core.NewStream(ch, nil), nil
}

func TestQueryRewriteStep_New(t *testing.T) {
	// Create a mock LLM
	mockLLM := &MockLLM{}

	// Test creating a new QueryRewriteStep
	step := NewQueryRewriteStep(mockLLM, logging.NewNoopLogger())
	assert.NotNil(t, step)
	assert.NotNil(t, step.llm)
}

func TestQueryRewriteStep_Name(t *testing.T) {
	// Create a mock LLM
	mockLLM := &MockLLM{}

	// Create a QueryRewriteStep
	step := NewQueryRewriteStep(mockLLM, logging.NewNoopLogger())

	// Test Name method
	name := step.Name()
	assert.Equal(t, "QueryRewriteStep", name)
}

func TestQueryRewriteStep_Execute_NoQuery(t *testing.T) {
	// Create a mock LLM
	mockLLM := &MockLLM{}

	// Create a QueryRewriteStep
	step := NewQueryRewriteStep(mockLLM, logging.NewNoopLogger())

	// Create a pipeline state without query
	state := &entity.PipelineState{}

	// Test Execute method without query
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err) // Should not error, just skip
}

func TestQueryRewriteStep_Execute_WithQuery(t *testing.T) {
	// Create a mock LLM that returns a specific rewritten query
	mockLLM := &MockLLM{
		chatFn: func(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
			// Verify the prompt contains the original query
			assert.Contains(t, messages[0].Content[0].Text, "What is the capital of France?")
			return &core.Response{Content: "What is the capital city of France?"}, nil
		},
	}

	// Create a QueryRewriteStep
	step := NewQueryRewriteStep(mockLLM, logging.NewNoopLogger())

	// Create a pipeline state with query
	state := &entity.PipelineState{
		Query: &entity.Query{
			Text: "What is the capital of France?",
		},
	}

	// Test Execute method with query
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)

	// Check that the query was rewritten
	assert.Equal(t, "What is the capital city of France?", state.Query.Text)

	// Check that the original query was preserved
	assert.NotNil(t, state.OriginalQuery)
	assert.Equal(t, "What is the capital of France?", state.OriginalQuery.Text)
}
