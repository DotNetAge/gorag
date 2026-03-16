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

// mockClientForExtractor implements core.Client for entity extractor testing
type mockClientForExtractor struct {
	response *core.Response
	err      error
}

func (m *mockClientForExtractor) Chat(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
	return m.response, m.err
}

func (m *mockClientForExtractor) ChatStream(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Stream, error) {
	return nil, nil
}

func TestNewEntityExtractor(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		mockLLM := &mockClientForExtractor{}
		extractor := NewEntityExtractor(mockLLM)

		assert.NotNil(t, extractor)
	})

	t.Run("with custom prompt template", func(t *testing.T) {
		mockLLM := &mockClientForExtractor{}
		customPrompt := "Custom extraction prompt: %s"

		extractor := NewEntityExtractor(mockLLM, WithEntityExtractionPromptTemplate(customPrompt))

		assert.NotNil(t, extractor)
		assert.Equal(t, customPrompt, extractor.promptTemplate)
	})

	t.Run("with logger and collector", func(t *testing.T) {
		mockLLM := &mockClientForExtractor{}
		logger := logging.NewNoopLogger()
		collector := observability.NewNoopCollector()

		extractor := NewEntityExtractor(
			mockLLM,
			WithEntityExtractorLogger(logger),
			WithEntityExtractorCollector(collector),
		)

		assert.NotNil(t, extractor)
		assert.Equal(t, logger, extractor.logger)
		assert.Equal(t, collector, extractor.collector)
	})
}

func TestEntityExtractor_Extract_Success(t *testing.T) {
	expectedResponse := &retrieval.EntityExtractionResult{
		Entities: []string{"Microsoft", "Bill Gates"},
	}

	responseJSON, _ := json.Marshal(expectedResponse)
	mockLLM := &mockClientForExtractor{
		response: &core.Response{Content: string(responseJSON)},
	}

	extractor := NewEntityExtractor(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "What is the relationship between Microsoft and Bill Gates?", nil)

	result, err := extractor.Extract(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Entities, 2)
	assert.Equal(t, "Microsoft", result.Entities[0])
	assert.Equal(t, "Bill Gates", result.Entities[1])
}

func TestEntityExtractor_Extract_EmptyQuery(t *testing.T) {
	mockLLM := &mockClientForExtractor{}

	extractor := NewEntityExtractor(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "", nil)

	result, err := extractor.Extract(ctx, query)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query required")
}

func TestEntityExtractor_Extract_LLMError(t *testing.T) {
	mockLLM := &mockClientForExtractor{
		err: assert.AnError,
	}

	extractor := NewEntityExtractor(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)

	result, err := extractor.Extract(ctx, query)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM chat failed")
}

func TestEntityExtractor_Extract_InvalidJSONResponse(t *testing.T) {
	mockLLM := &mockClientForExtractor{
		response: &core.Response{Content: "invalid json response"},
	}

	extractor := NewEntityExtractor(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)

	result, err := extractor.Extract(ctx, query)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "parse response")
}

func TestEntityExtractor_Extract_EmptyEntities(t *testing.T) {
	response := &retrieval.EntityExtractionResult{
		Entities: []string{},
	}

	responseJSON, _ := json.Marshal(response)
	mockLLM := &mockClientForExtractor{
		response: &core.Response{Content: string(responseJSON)},
	}

	extractor := NewEntityExtractor(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "simple question", nil)

	result, err := extractor.Extract(ctx, query)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Entities)
}

func TestEntityExtractor_Extract_MetricsRecording(t *testing.T) {
	response := &retrieval.EntityExtractionResult{
		Entities: []string{"entity1"},
	}

	responseJSON, _ := json.Marshal(response)
	mockLLM := &mockClientForExtractor{
		response: &core.Response{Content: string(responseJSON)},
	}

	collector := observability.NewNoopCollector()
	extractor := NewEntityExtractor(mockLLM, WithEntityExtractorCollector(collector))
	ctx := context.Background()
	query := entity.NewQuery("", "test", nil)

	_, err := extractor.Extract(ctx, query)

	assert.NoError(t, err)
	// Metrics should be recorded without panic
}
