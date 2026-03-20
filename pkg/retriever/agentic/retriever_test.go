package agentic

import (
	"context"
	"testing"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core/agent"
	"github.com/DotNetAge/gorag/pkg/core/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockAgent struct {
	mock.Mock
}

func (m *mockAgent) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockAgent) AddTool(tool agent.Tool) {
	m.Called(tool)
}

func (m *mockAgent) Chat(ctx context.Context, query string, history []chat.Message) (*agent.AgentResponse, error) {
	args := m.Called(ctx, query, history)
	return args.Get(0).(*agent.AgentResponse), args.Error(1)
}

func (m *mockAgent) Memory() store.ChatMemory {
	args := m.Called()
	return args.Get(0).(store.ChatMemory)
}

func TestAgenticRetriever_Retrieve(t *testing.T) {
	ctx := context.Background()
	queryText := "Explain the RAG process."
	
	mAgent := new(mockAgent)
	mAgent.On("Name").Return("TestAgent")
	mAgent.On("Chat", ctx, queryText, mock.Anything).Return(&agent.AgentResponse{
		Response: "RAG involves retrieval and generation.",
		Steps: []agent.AgentStep{
			{Thought: "I should explain RAG.", Action: "None", Observation: "Initial thought."},
		},
	}, nil).Once()

	retriever := NewRetriever(mAgent)
	results, err := retriever.Retrieve(ctx, []string{queryText}, 5)

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "RAG involves retrieval and generation.", results[0].Answer)
	assert.Equal(t, queryText, results[0].Query)
	
	// Check if agent steps are in metadata
	assert.NotNil(t, results[0].Metadata["agent_steps"])
	steps := results[0].Metadata["agent_steps"].([]agent.AgentStep)
	assert.Len(t, steps, 1)
	assert.Equal(t, "I should explain RAG.", steps[0].Thought)

	mAgent.AssertExpectations(t)
}
