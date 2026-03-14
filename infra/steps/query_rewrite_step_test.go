package steps

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/stretchr/testify/assert"
)

// MockQueryRewriter is a mock implementation of retrieval.QueryRewriter
type MockQueryRewriter struct {
	rewriteFn func(ctx context.Context, query *entity.Query) (*entity.Query, error)
}

func (m *MockQueryRewriter) Rewrite(ctx context.Context, query *entity.Query) (*entity.Query, error) {
	if m.rewriteFn != nil {
		return m.rewriteFn(ctx, query)
	}
	return &entity.Query{Text: "Rewritten query"}, nil
}

func TestQueryRewriteStep_New(t *testing.T) {
	// Create a mock query rewriter
	mockRewriter := &MockQueryRewriter{}

	// Test creating a new QueryRewriteStep
	step := NewQueryRewriteStep(mockRewriter)
	assert.NotNil(t, step)
	assert.NotNil(t, step.rewriter)
}

func TestQueryRewriteStep_Name(t *testing.T) {
	// Create a mock query rewriter
	mockRewriter := &MockQueryRewriter{}

	// Create a QueryRewriteStep
	step := NewQueryRewriteStep(mockRewriter)

	// Test Name method
	name := step.Name()
	assert.Equal(t, "QueryRewriteStep", name)
}

func TestQueryRewriteStep_Execute_NoQuery(t *testing.T) {
	// Create a mock query rewriter
	mockRewriter := &MockQueryRewriter{}

	// Create a QueryRewriteStep
	step := NewQueryRewriteStep(mockRewriter)

	// Create a pipeline state without query
	state := &entity.PipelineState{}

	// Test Execute method without query
	ctx := context.Background()
	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "QueryRewriteStep: 'query' not found in state")
}

func TestQueryRewriteStep_Execute_WithQuery(t *testing.T) {
	// Create a mock query rewriter that returns a specific rewritten query
	mockRewriter := &MockQueryRewriter{
		rewriteFn: func(ctx context.Context, query *entity.Query) (*entity.Query, error) {
			assert.Equal(t, "What is the capital of France?", query.Text)
			return &entity.Query{Text: "What is the capital city of France?"}, nil
		},
	}

	// Create a QueryRewriteStep
	step := NewQueryRewriteStep(mockRewriter)

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
