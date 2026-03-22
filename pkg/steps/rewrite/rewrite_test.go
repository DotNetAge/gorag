package rewrite

import (
	"context"
	"errors"
	"testing"

	gchat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockLLMForRewrite struct {
	response *gchat.Response
	err      error
}

func (m *mockLLMForRewrite) Chat(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockLLMForRewrite) ChatStream(ctx context.Context, messages []gchat.Message, opts ...gchat.Option) (*gchat.Stream, error) {
	return nil, nil
}

type mockLoggerForRewrite struct{}

func (m *mockLoggerForRewrite) Debug(msg string, fields ...map[string]any)            {}
func (m *mockLoggerForRewrite) Info(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForRewrite) Warn(msg string, fields ...map[string]any)             {}
func (m *mockLoggerForRewrite) Error(msg string, err error, fields ...map[string]any) {}

type mockMetricsForRewrite struct{}

func (m *mockMetricsForRewrite) RecordSearchDuration(engine string, duration any)   {}
func (m *mockMetricsForRewrite) RecordSearchResult(engine string, resultCount int)  {}
func (m *mockMetricsForRewrite) RecordSearchError(engine string, err error)         {}
func (m *mockMetricsForRewrite) RecordIndexingDuration(parser string, duration any) {}
func (m *mockMetricsForRewrite) RecordIndexingResult(parser string, count int)      {}
func (m *mockMetricsForRewrite) RecordEmbeddingCount(count int)                     {}
func (m *mockMetricsForRewrite) RecordVectorStoreOperations(op string, count int)   {}

func TestRewrite_Name(t *testing.T) {
	step := Rewrite(nil, nil, nil)
	assert.Equal(t, "QueryRewrite", step.Name())
}

func TestRewrite_Execute_Success(t *testing.T) {
	llm := &mockLLMForRewrite{
		response: &gchat.Response{Content: "clarified query"},
	}
	step := Rewrite(llm, &mockLoggerForRewrite{}, &mockMetricsForRewrite{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "original query", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Equal(t, "clarified query", state.Query.Text)
	assert.Equal(t, "original query", state.OriginalQuery)
}

func TestRewrite_Execute_NilQuery(t *testing.T) {
	step := Rewrite(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestRewrite_Execute_EmptyQuery(t *testing.T) {
	step := Rewrite(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
}

func TestRewrite_Execute_LLMError(t *testing.T) {
	llm := &mockLLMForRewrite{err: errors.New("LLM error")}
	step := Rewrite(llm, &mockLoggerForRewrite{}, &mockMetricsForRewrite{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Equal(t, "test query", state.Query.Text)
}

func TestRewrite_Execute_EmptyResponse(t *testing.T) {
	llm := &mockLLMForRewrite{
		response: &gchat.Response{Content: ""},
	}
	step := Rewrite(llm, &mockLoggerForRewrite{}, &mockMetricsForRewrite{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Equal(t, "test query", state.Query.Text)
}

func TestRewrite_Execute_PreservesOriginalQuery(t *testing.T) {
	llm := &mockLLMForRewrite{
		response: &gchat.Response{Content: "rewritten"},
	}
	step := Rewrite(llm, &mockLoggerForRewrite{}, &mockMetricsForRewrite{})
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:         core.NewQuery("1", "original", nil),
		OriginalQuery: "already_set",
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Equal(t, "rewritten", state.Query.Text)
	assert.Equal(t, "already_set", state.OriginalQuery)
}

func TestRewrite_Execute_WithNilLogger(t *testing.T) {
	llm := &mockLLMForRewrite{
		response: &gchat.Response{Content: "rewritten"},
	}
	step := Rewrite(llm, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "original", nil),
	}

	err := step.Execute(ctx, state)

	assert.NoError(t, err)
	assert.Equal(t, "rewritten", state.Query.Text)
}

func TestBuildRewritePrompt(t *testing.T) {
	step := Rewrite(nil, nil, nil).(*rewrite)
	prompt := step.buildRewritePrompt("test query")

	assert.Contains(t, prompt, "test query")
	assert.Contains(t, prompt, "expert")
	assert.Contains(t, prompt, "clarifying")
}
