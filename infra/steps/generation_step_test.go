package steps

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
	return &chat.Response{Content: "Test answer"}, nil
}

func (m *MockChatClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "Generated text", nil
}

func (m *MockChatClient) ChatStream(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Stream, error) {
	return nil, nil
}

func TestGenerationStep_New(t *testing.T) {
	// Create a mock chat client
	mockClient := &MockChatClient{}

	// Test creating a new GenerationStep
	step := NewGenerationStep(mockClient)
	assert.NotNil(t, step)
	assert.NotNil(t, step.llm)
}

func TestGenerationStep_Name(t *testing.T) {
	// Create a mock chat client
	mockClient := &MockChatClient{}

	// Create a GenerationStep
	step := NewGenerationStep(mockClient)

	// Test Name method
	name := step.Name()
	assert.Equal(t, "GenerationStep", name)
}

func TestGenerationStep_Execute_NoQuery(t *testing.T) {
	// Create a mock chat client
	mockClient := &MockChatClient{}

	// Create a GenerationStep
	step := NewGenerationStep(mockClient)

	// Create a pipeline state without query
	state := &entity.PipelineState{}

	// Test Execute method without query
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "GenerationStep: 'query' not found in state")
}

func TestGenerationStep_Execute_WithQuery(t *testing.T) {
	// Create a mock chat client that returns a specific response
	mockClient := &MockChatClient{
		chatFn: func(ctx context.Context, messages []chat.Message, options ...chat.Option) (*chat.Response, error) {
			// Verify that the message contains the expected content
			assert.Len(t, messages, 1)
			assert.Len(t, messages[0].Content, 1)
			assert.Contains(t, messages[0].Content[0].Text, "User Question")
			assert.Contains(t, messages[0].Content[0].Text, "What is the capital of France?")
			assert.Contains(t, messages[0].Content[0].Text, "Paris is the capital of France")
			return &chat.Response{Content: "The capital of France is Paris."}, nil
		},
	}

	// Create a GenerationStep
	step := NewGenerationStep(mockClient)

	// Create a pipeline state with query and retrieved chunks
	state := &entity.PipelineState{
		Query: &entity.Query{
			Text: "What is the capital of France?",
		},
		RetrievedChunks: [][]*entity.Chunk{
			{{ID: "chunk1", Content: "Paris is the capital of France."}},
		},
	}

	// Test Execute method with query and retrieved chunks
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.NoError(t, err)

	// Check that the answer was set
	assert.Equal(t, "The capital of France is Paris.", state.Answer)

	// Check that the generation prompt was set
	assert.Contains(t, state.GenerationPrompt, "User Question")
	assert.Contains(t, state.GenerationPrompt, "What is the capital of France?")
	assert.Contains(t, state.GenerationPrompt, "Paris is the capital of France")
}
