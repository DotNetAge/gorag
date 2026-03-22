package query

import (
	"context"
	"errors"
	"testing"
	"time"

	gchat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMForDecomposer struct {
	response *gchat.Response
	err      error
}

func (m *mockLLMForDecomposer) Chat(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMForDecomposer) ChatStream(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Stream, error) {
	return nil, nil
}

type mockLoggerForDecomposer struct{}

func (m *mockLoggerForDecomposer) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForDecomposer) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForDecomposer) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForDecomposer) Error(msg string, err error, fields ...map[string]any) {}

type mockCollectorForDecomposer struct{}

func (m *mockCollectorForDecomposer) RecordCount(name, value string, labels map[string]string) {}
func (m *mockCollectorForDecomposer) RecordDuration(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockCollectorForDecomposer) RecordValue(name string, value float64, labels map[string]string) {
}

func TestDecompose_Success(t *testing.T) {
	llm := &mockLLMForDecomposer{
		response: &gchat.Response{
			Content: `{"sub_queries":["What is semantic cache?","How does vector similarity work?","What are the benefits of caching?"],"reasoning":"Decomposed into three aspects","is_complex":true}`,
		},
	}
	decomposer := NewDecomposer(llm)

	result, err := decomposer.Decompose(context.Background(), core.NewQuery("1", "Explain semantic cache in RAG", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SubQueries, 3)
	assert.True(t, result.IsComplex)
}

func TestDecompose_SimpleQuery(t *testing.T) {
	llm := &mockLLMForDecomposer{
		response: &gchat.Response{
			Content: `{"sub_queries":["What is Go?"],"reasoning":"Simple query, no decomposition needed","is_complex":false}`,
		},
	}
	decomposer := NewDecomposer(llm)

	result, err := decomposer.Decompose(context.Background(), core.NewQuery("1", "What is Go?", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SubQueries, 1)
	assert.False(t, result.IsComplex)
}

func TestDecompose_NilQuery(t *testing.T) {
	decomposer := NewDecomposer(nil)

	result, err := decomposer.Decompose(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "nil or empty")
}

func TestDecompose_EmptyQuery(t *testing.T) {
	decomposer := NewDecomposer(nil)

	result, err := decomposer.Decompose(context.Background(), core.NewQuery("1", "", nil))

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestDecompose_LLMError(t *testing.T) {
	llm := &mockLLMForDecomposer{err: errors.New("LLM error")}
	decomposer := NewDecomposer(llm)

	result, err := decomposer.Decompose(context.Background(), core.NewQuery("1", "test query", nil))

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "query decomposition failed")
}

func TestDecompose_WithOptions(t *testing.T) {
	llm := &mockLLMForDecomposer{
		response: &gchat.Response{
			Content: `{"sub_queries":["q1","q2","q3","q4","q5","q6"],"reasoning":"test","is_complex":true}`,
		},
	}
	decomposer := NewDecomposer(llm,
		WithDecompositionPromptTemplate("custom prompt"),
		WithMaxSubQueries(3),
		WithDecomposerLogger(&mockLoggerForDecomposer{}),
		WithDecomposerCollector(&mockCollectorForDecomposer{}),
	)

	result, err := decomposer.Decompose(context.Background(), core.NewQuery("1", "test", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.SubQueries, 3)
}