package query

import (
	"context"
	"errors"
	"testing"

	gchat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMForStepBack struct {
	response *gchat.Response
	err      error
}

func (m *mockLLMForStepBack) Chat(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMForStepBack) ChatStream(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Stream, error) {
	return nil, nil
}

func TestGenerateStepBackQuery_Success(t *testing.T) {
	llm := &mockLLMForStepBack{
		response: &gchat.Response{Content: "What are the concurrency primitives in Go?"},
	}
	stepback := NewStepBack(llm)

	result, err := stepback.GenerateStepBackQuery(context.Background(), core.NewQuery("1", "How does Go channel work?", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "What are the concurrency primitives in Go?", result.Text)
}

func TestGenerateStepBackQuery_NilQuery(t *testing.T) {
	stepback := NewStepBack(nil)

	result, err := stepback.GenerateStepBackQuery(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestGenerateStepBackQuery_LLMError(t *testing.T) {
	llm := &mockLLMForStepBack{err: errors.New("LLM error")}
	stepback := NewStepBack(llm)

	result, err := stepback.GenerateStepBackQuery(context.Background(), core.NewQuery("1", "test query", nil))

	assert.Error(t, err)
	assert.Nil(t, result)
}