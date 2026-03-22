package query

import (
	"context"
	"errors"
	"testing"

	gchat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMForRewriter struct {
	response *gchat.Response
	err      error
}

func (m *mockLLMForRewriter) Chat(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMForRewriter) ChatStream(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Stream, error) {
	return nil, nil
}

func TestRewrite_Success(t *testing.T) {
	llm := &mockLLMForRewriter{
		response: &gchat.Response{Content: "What is semantic cache in RAG systems?"},
	}
	rewriter := NewRewriter(llm)

	result, err := rewriter.Rewrite(context.Background(), core.NewQuery("1", "tell me about semantic cache", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "What is semantic cache in RAG systems?", result.Text)
}

func TestRewrite_NilQuery(t *testing.T) {
	rewriter := NewRewriter(nil)

	result, err := rewriter.Rewrite(context.Background(), nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRewrite_EmptyQuery(t *testing.T) {
	rewriter := NewRewriter(nil)

	result, err := rewriter.Rewrite(context.Background(), core.NewQuery("1", "", nil))

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRewrite_LLMError(t *testing.T) {
	llm := &mockLLMForRewriter{err: errors.New("LLM error")}
	rewriter := NewRewriter(llm)

	result, err := rewriter.Rewrite(context.Background(), core.NewQuery("1", "test query", nil))

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestRewrite_EmptyResponseFallsBack(t *testing.T) {
	llm := &mockLLMForRewriter{
		response: &gchat.Response{Content: ""},
	}
	rewriter := NewRewriter(llm)

	result, err := rewriter.Rewrite(context.Background(), core.NewQuery("1", "original query", nil))

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "original query", result.Text)
}