package service

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/stretchr/testify/assert"
)

// mockClient implements core.Client for testing
type mockClient struct {
	response *core.Response
	err      error
}

func (m *mockClient) Chat(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Response, error) {
	return m.response, m.err
}

func (m *mockClient) ChatStream(ctx context.Context, messages []core.Message, opts ...core.Option) (*core.Stream, error) {
	// For testing purposes, return nil stream
	// This method is not used by Generator.Generate()
	return nil, nil
}

func TestNewGenerator(t *testing.T) {
	t.Run("default configuration", func(t *testing.T) {
		mockLLM := &mockClient{}
		gen := NewGenerator(mockLLM)

		assert.NotNil(t, gen)
	})

	t.Run("with custom prompt template", func(t *testing.T) {
		mockLLM := &mockClient{}
		customPrompt := "Custom: %s\n%s"

		gen := NewGenerator(mockLLM, WithPromptTemplate(customPrompt))

		assert.NotNil(t, gen)
		assert.Equal(t, customPrompt, gen.promptTemplate)
	})

	t.Run("with logger and collector", func(t *testing.T) {
		mockLLM := &mockClient{}
		logger := logging.NewNoopLogger()
		collector := observability.NewNoopCollector()

		gen := NewGenerator(
			mockLLM,
			WithGeneratorLogger(logger),
			WithGeneratorCollector(collector),
		)

		assert.NotNil(t, gen)
		assert.Equal(t, logger, gen.logger)
		assert.Equal(t, collector, gen.collector)
	})
}

func TestGenerator_Generate_Success(t *testing.T) {
	expectedAnswer := "This is the generated answer."
	mockLLM := &mockClient{
		response: &core.Response{Content: expectedAnswer},
	}

	gen := NewGenerator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)
	chunks := []*entity.Chunk{
		{ID: "1", Content: "context 1"},
		{ID: "2", Content: "context 2"},
	}

	result, err := gen.Generate(ctx, query, chunks)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedAnswer, result.Answer)
}

func TestGenerator_Generate_EmptyChunks(t *testing.T) {
	expectedAnswer := "Answer from query only"
	mockLLM := &mockClient{
		response: &core.Response{Content: expectedAnswer},
	}

	gen := NewGenerator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)

	result, err := gen.Generate(ctx, query, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedAnswer, result.Answer)
}

func TestGenerator_Generate_NilQuery(t *testing.T) {
	mockLLM := &mockClient{}

	gen := NewGenerator(mockLLM)
	ctx := context.Background()

	result, err := gen.Generate(ctx, nil, []*entity.Chunk{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerator_Generate_EmptyQueryText(t *testing.T) {
	mockLLM := &mockClient{}

	gen := NewGenerator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "", nil)

	result, err := gen.Generate(ctx, query, []*entity.Chunk{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerator_Generate_LLMError(t *testing.T) {
	mockLLM := &mockClient{
		err: errors.New("LLM chat failed"),
	}

	gen := NewGenerator(mockLLM)
	ctx := context.Background()
	query := entity.NewQuery("", "test query", nil)
	chunks := []*entity.Chunk{{ID: "1", Content: "context"}}

	result, err := gen.Generate(ctx, query, chunks)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "Chat failed")
}

func TestGenerator_Generate_MetricsRecording(t *testing.T) {
	mockLLM := &mockClient{
		response: &core.Response{Content: "answer"},
	}
	collector := observability.NewNoopCollector()

	gen := NewGenerator(mockLLM, WithGeneratorCollector(collector))
	ctx := context.Background()
	query := entity.NewQuery("", "test", nil)

	_, err := gen.Generate(ctx, query, nil)

	assert.NoError(t, err)
	// Metrics should be recorded without panic
}
