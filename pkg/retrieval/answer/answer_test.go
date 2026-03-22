package answer

import (
	"context"
	"errors"
	"testing"
	"time"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMForAnswer struct {
	response string
	err      error
}

func (m *mockLLMForAnswer) Chat(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &chat.Response{Content: m.response}, nil
}

func (m *mockLLMForAnswer) ChatStream(ctx context.Context, messages []chat.Message, opts ...chat.Option) (*chat.Stream, error) {
	return nil, nil
}

type mockLoggerForAnswer struct{}

func (m *mockLoggerForAnswer) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForAnswer) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForAnswer) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForAnswer) Error(msg string, err error, fields ...map[string]any) {}

type mockCollectorForAnswer struct{}

func (m *mockCollectorForAnswer) RecordCount(name, value string, labels map[string]string) {}
func (m *mockCollectorForAnswer) RecordDuration(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockCollectorForAnswer) RecordValue(name string, value float64, labels map[string]string) {}

func TestNew_DefaultValues(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "test response"}
	gen := New(mockLLM)

	assert.NotNil(t, gen)
	assert.NotNil(t, gen.logger)
	assert.NotNil(t, gen.collector)
}

func TestNew_WithOptions(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "test response"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM,
		WithLogger(logger),
		WithCollector(collector),
		WithPromptTemplate("custom prompt"),
	)

	assert.NotNil(t, gen)
}

func TestGenerator_Generate_Success(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "Generated answer content"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	chunks := []*core.Chunk{
		{ID: "1", Content: "Document 1 content"},
		{ID: "2", Content: "Document 2 content"},
	}

	result, err := gen.Generate(context.Background(), core.NewQuery("1", "test query", nil), chunks)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Generated answer content", result.Answer)
}

func TestGenerator_Generate_NilQuery(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "response"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	chunks := []*core.Chunk{{ID: "1", Content: "test"}}

	result, err := gen.Generate(context.Background(), nil, chunks)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerator_Generate_EmptyQuery(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "response"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	chunks := []*core.Chunk{{ID: "1", Content: "test"}}

	result, err := gen.Generate(context.Background(), core.NewQuery("1", "", nil), chunks)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerator_Generate_LLMError(t *testing.T) {
	mockLLM := &mockLLMForAnswer{err: errors.New("LLM error")}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	chunks := []*core.Chunk{{ID: "1", Content: "test"}}

	result, err := gen.Generate(context.Background(), core.NewQuery("1", "test query", nil), chunks)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "LLM error")
}

func TestGenerator_Generate_EmptyChunks(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "Generated answer"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	result, err := gen.Generate(context.Background(), core.NewQuery("1", "test query", nil), []*core.Chunk{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGenerator_Generate_ChunksWithEmptyContent(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "Generated answer"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	chunks := []*core.Chunk{
		{ID: "1", Content: ""},
		{ID: "2", Content: "Valid content"},
	}

	result, err := gen.Generate(context.Background(), core.NewQuery("1", "test query", nil), chunks)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestGenerator_GenerateHypotheticalDocument_Success(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "Hypothetical document content"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	doc, err := gen.GenerateHypotheticalDocument(context.Background(), core.NewQuery("1", "test query", nil))

	assert.NoError(t, err)
	assert.Equal(t, "Hypothetical document content", doc)
}

func TestGenerator_GenerateHypotheticalDocument_NilQuery(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "response"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	doc, err := gen.GenerateHypotheticalDocument(context.Background(), nil)

	assert.Error(t, err)
	assert.Empty(t, doc)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerator_GenerateHypotheticalDocument_EmptyQuery(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "response"}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	doc, err := gen.GenerateHypotheticalDocument(context.Background(), core.NewQuery("1", "", nil))

	assert.Error(t, err)
	assert.Empty(t, doc)
	assert.Contains(t, err.Error(), "query required")
}

func TestGenerator_GenerateHypotheticalDocument_LLMError(t *testing.T) {
	mockLLM := &mockLLMForAnswer{err: errors.New("HyDE error")}
	logger := &mockLoggerForAnswer{}
	collector := &mockCollectorForAnswer{}

	gen := New(mockLLM, WithLogger(logger), WithCollector(collector))

	doc, err := gen.GenerateHypotheticalDocument(context.Background(), core.NewQuery("1", "test query", nil))

	assert.Error(t, err)
	assert.Empty(t, doc)
	assert.Contains(t, err.Error(), "HyDE error")
}

func TestWithPromptTemplate_EmptyString(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "response"}
	gen := New(mockLLM, WithPromptTemplate(""))

	assert.NotNil(t, gen)
	assert.NotEmpty(t, gen.promptTemplate)
}

func TestWithLogger_NilLogger(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "response"}
	gen := New(mockLLM, WithLogger(nil))

	assert.NotNil(t, gen)
}

func TestWithCollector_NilCollector(t *testing.T) {
	mockLLM := &mockLLMForAnswer{response: "response"}
	gen := New(mockLLM, WithCollector(nil))

	assert.NotNil(t, gen)
}
