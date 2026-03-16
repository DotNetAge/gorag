package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
	"github.com/stretchr/testify/assert"
)

// mockClientForDecomposer implements core.Client for query decomposer testing
type mockClientForDecomposer struct {
	response *core.Response
	err      error
}

func (m *mockClientForDecomposer) Chat(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
	return m.response, m.err
}

func (m *mockClientForDecomposer) ChatStream(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Stream, error) {
	return nil, nil
}

func TestNewQueryDecomposer(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		mockLLM := &mockClientForDecomposer{}
		decomposer := NewQueryDecomposer(mockLLM)

		assert.NotNil(t, decomposer)
		assert.Equal(t, 5, decomposer.maxSubQueries)
	})

	t.Run("with custom max sub-queries", func(t *testing.T) {
		mockLLM := &mockClientForDecomposer{}
		decomposer := NewQueryDecomposer(mockLLM, WithMaxSubQueries(10))

		assert.NotNil(t, decomposer)
		assert.Equal(t, 10, decomposer.maxSubQueries)
	})

	t.Run("with logger and collector", func(t *testing.T) {
		mockLLM := &mockClientForDecomposer{}
		logger := logging.NewNoopLogger()
		collector := observability.NewNoopCollector()

		decomposer := NewQueryDecomposer(
			mockLLM,
			WithQueryDecomposerLogger(logger),
			WithQueryDecomposerCollector(collector),
		)

		assert.NotNil(t, decomposer)
		assert.Equal(t, logger, decomposer.logger)
		assert.Equal(t, collector, decomposer.collector)
	})
}

func TestQueryDecomposer_Decompose_Success(t *testing.T) {
	expectedResponse := &retrieval.DecompositionResult{
		SubQueries: []string{"What is RAG?", "How does retrieval work?"},
		Reasoning:  "Breaking down into definition and mechanism",
		IsComplex:  true,
	}

	responseJSON, _ := json.Marshal(expectedResponse)
	mockLLM := &mockClientForDecomposer{
		response: &core.Response{Content: string(responseJSON)},
	}

	decomposer := NewQueryDecomposer(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "Explain how RAG systems work with retrieval and generation", nil)

	result, err := decomposer.Decompose(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SubQueries, 2)
	assert.Equal(t, "What is RAG?", result.SubQueries[0])
	assert.Equal(t, "How does retrieval work?", result.SubQueries[1])
	assert.True(t, result.IsComplex)
}

func TestQueryDecomposer_Decompose_EmptyQuery(t *testing.T) {
	mockLLM := &mockClientForDecomposer{}

	decomposer := NewQueryDecomposer(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "", nil)

	result, err := decomposer.Decompose(ctx, query)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query required")
}

func TestQueryDecomposer_Decompose_LLMError(t *testing.T) {
	mockLLM := &mockClientForDecomposer{
		err: assert.AnError,
	}

	decomposer := NewQueryDecomposer(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)

	result, err := decomposer.Decompose(ctx, query)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM chat failed")
}

func TestQueryDecomposer_Decompose_InvalidJSONResponse(t *testing.T) {
	mockLLM := &mockClientForDecomposer{
		response: &core.Response{Content: "invalid json response"},
	}

	decomposer := NewQueryDecomposer(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)

	result, err := decomposer.Decompose(ctx, query)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "parse response")
}

func TestQueryDecomposer_Decompose_MaxSubQueriesLimit(t *testing.T) {
	response := &retrieval.DecompositionResult{
		SubQueries: []string{"q1", "q2", "q3", "q4", "q5", "q6", "q7"},
		Reasoning:  "Many aspects",
		IsComplex:  true,
	}

	responseJSON, _ := json.Marshal(response)
	mockLLM := &mockClientForDecomposer{
		response: &core.Response{Content: string(responseJSON)},
	}

	// Set max to 5
	decomposer := NewQueryDecomposer(mockLLM, WithMaxSubQueries(5))
	ctx := context.Background()
	query := entity.NewQuery("", "complex query", nil)

	result, err := decomposer.Decompose(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SubQueries, 5) // Should be limited to max
}

func TestQueryDecomposer_Decompose_MetricsRecording(t *testing.T) {
	response := &retrieval.DecompositionResult{
		SubQueries: []string{"q1"},
		Reasoning:  "simple",
		IsComplex:  false,
	}

	responseJSON, _ := json.Marshal(response)
	mockLLM := &mockClientForDecomposer{
		response: &core.Response{Content: string(responseJSON)},
	}

	collector := observability.NewNoopCollector()
	decomposer := NewQueryDecomposer(mockLLM, WithQueryDecomposerCollector(collector))
	ctx := context.Background()
	query := entity.NewQuery("", "test", nil)

	_, err := decomposer.Decompose(ctx, query)

	assert.NoError(t, err)
	// Metrics should be recorded without panic
}
