package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
	"github.com/stretchr/testify/assert"
)

// mockClientForCRAG implements core.Client for CRAG evaluator testing
type mockClientForCRAG struct {
	response *core.Response
	err      error
}

func (m *mockClientForCRAG) Chat(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
	return m.response, m.err
}

func (m *mockClientForCRAG) ChatStream(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Stream, error) {
	return nil, nil
}

func TestNewCRAGEvaluator(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		mockLLM := &mockClientForCRAG{}
		eval := NewCRAGEvaluator(mockLLM)

		assert.NotNil(t, eval)
	})

	t.Run("with custom prompt template", func(t *testing.T) {
		mockLLM := &mockClientForCRAG{}
		customPrompt := "Custom CRAG prompt: %s"

		eval := NewCRAGEvaluator(mockLLM, WithCRAGPromptTemplate(customPrompt))

		assert.NotNil(t, eval)
		assert.Equal(t, customPrompt, eval.promptTemplate)
	})

	t.Run("with logger and collector", func(t *testing.T) {
		mockLLM := &mockClientForCRAG{}
		logger := logging.NewNoopLogger()
		collector := observability.NewNoopCollector()

		eval := NewCRAGEvaluator(
			mockLLM,
			WithCRAGLogger(logger),
			WithCRAGCollector(collector),
		)

		assert.NotNil(t, eval)
		assert.Equal(t, logger, eval.logger)
		assert.Equal(t, collector, eval.collector)
	})
}

func TestCRAGEvaluator_Evaluate_Success(t *testing.T) {
	expectedResponse := &retrieval.CRAGEvaluation{
		Relevance: 0.85,
		Label:     retrieval.CRAGRelevant,
		Reason:    "Highly relevant content",
	}

	responseJSON, _ := json.Marshal(expectedResponse)
	mockLLM := &mockClientForCRAG{
		response: &core.Response{Content: string(responseJSON)},
	}

	eval := NewCRAGEvaluator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)
	chunks := []*entity.Chunk{
		{ID: "1", Content: "relevant content 1"},
		{ID: "2", Content: "relevant content 2"},
	}

	result, err := eval.Evaluate(ctx, query, chunks)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedResponse.Relevance, result.Relevance)
	assert.Equal(t, expectedResponse.Label, result.Label)
	assert.Equal(t, expectedResponse.Reason, result.Reason)
}

func TestCRAGEvaluator_Evaluate_EmptyChunks(t *testing.T) {
	responseJSON := `{"relevance": 0.3, "label": "irrelevant", "reason": "no content"}`
	mockLLM := &mockClientForCRAG{
		response: &core.Response{Content: responseJSON},
	}

	eval := NewCRAGEvaluator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)

	result, err := eval.Evaluate(ctx, query, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, retrieval.CRAGIrrelevant, result.Label)
}

func TestCRAGEvaluator_Evaluate_NilQuery(t *testing.T) {
	mockLLM := &mockClientForCRAG{}

	eval := NewCRAGEvaluator(mockLLM)
	ctx := context.Background()

	result, err := eval.Evaluate(ctx, nil, []*entity.Chunk{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query is nil or empty")
}

func TestCRAGEvaluator_Evaluate_LLMError(t *testing.T) {
	mockLLM := &mockClientForCRAG{
		err: errors.New("LLM chat failed"),
	}

	eval := NewCRAGEvaluator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)
	chunks := []*entity.Chunk{{ID: "1", Content: "context"}}

	result, err := eval.Evaluate(ctx, query, chunks)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM chat failed")
}

func TestCRAGEvaluator_Evaluate_InvalidJSONResponse(t *testing.T) {
	mockLLM := &mockClientForCRAG{
		response: &core.Response{Content: "invalid json response"},
	}

	eval := NewCRAGEvaluator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)
	chunks := []*entity.Chunk{{ID: "1", Content: "context"}}

	result, err := eval.Evaluate(ctx, query, chunks)

	// When JSON parsing fails, the evaluator returns a default evaluation with ambiguous label
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, retrieval.CRAGAmbiguous, result.Label)
	assert.Equal(t, float32(0.5), result.Relevance)
	assert.Contains(t, result.Reason, "Failed to parse LLM response")
}

func TestCRAGEvaluator_Evaluate_MetricsRecording(t *testing.T) {
	responseJSON := `{"relevance": 0.8, "label": "relevant", "reason": "test"}`
	mockLLM := &mockClientForCRAG{
		response: &core.Response{Content: responseJSON},
	}

	collector := observability.NewNoopCollector()
	eval := NewCRAGEvaluator(mockLLM, WithCRAGCollector(collector))
	ctx := context.Background()
	query := entity.NewQuery("", "test", nil)

	_, err := eval.Evaluate(ctx, query, nil)

	assert.NoError(t, err)
	// Metrics should be recorded without panic
}
