package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/stretchr/testify/assert"
)

type mockSemanticCache struct {
	result *CacheResult
	err    error
}

func (m *mockSemanticCache) CheckCache(ctx context.Context, query *core.Query) (*CacheResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *mockSemanticCache) CacheResponse(ctx context.Context, query *core.Query, answer *core.Result) error {
	return m.err
}

func TestCheck_Name(t *testing.T) {
	step := Check(nil, nil, nil)
	assert.Equal(t, "SemanticCacheCheck", step.Name())
}

func TestCheck_Execute_NilQuery(t *testing.T) {
	step := Check(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.Nil(t, state.Agentic)
}

func TestCheck_Execute_EmptyQuery(t *testing.T) {
	step := Check(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("q1", "", nil),
		Agentic: core.NewAgenticState(),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.False(t, state.Agentic.CacheHit)
}

func TestCheck_Execute_CacheHit(t *testing.T) {
	cache := &mockSemanticCache{
		result: &CacheResult{Hit: true, Answer: "Cached answer"},
	}
	step := Check(cache, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("q1", "test query", nil),
		Agentic: core.NewAgenticState(),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.True(t, state.Agentic.CacheHit)
	assert.NotNil(t, state.Answer)
	assert.Equal(t, "Cached answer", state.Answer.Answer)
}

func TestCheck_Execute_CacheMiss(t *testing.T) {
	cache := &mockSemanticCache{
		result: &CacheResult{Hit: false, Answer: ""},
	}
	step := Check(cache, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("q1", "test query", nil),
		Agentic: core.NewAgenticState(),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.False(t, state.Agentic.CacheHit)
	assert.Nil(t, state.Answer)
}

func TestCheck_Execute_CacheError(t *testing.T) {
	cache := &mockSemanticCache{
		err: errors.New("cache error"),
	}
	step := Check(cache, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("q1", "test query", nil),
		Agentic: core.NewAgenticState(),
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
	assert.False(t, state.Agentic.CacheHit)
}

func TestStore_Name(t *testing.T) {
	step := Store(nil, nil, nil)
	assert.Equal(t, "SemanticCacheStore", step.Name())
}

func TestStore_Execute_NilQuery(t *testing.T) {
	step := Store(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{Query: nil}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestStore_Execute_NilAnswer(t *testing.T) {
	step := Store(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:  core.NewQuery("q1", "test", nil),
		Answer: nil,
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestStore_Execute_EmptyAnswer(t *testing.T) {
	step := Store(nil, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:  core.NewQuery("q1", "test", nil),
		Answer: &core.Result{Answer: ""},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestStore_Execute_Success(t *testing.T) {
	cache := &mockSemanticCache{}
	step := Store(cache, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:  core.NewQuery("q1", "test", nil),
		Answer: &core.Result{Answer: "Generated answer"},
	}

	err := step.Execute(ctx, state)
	assert.NoError(t, err)
}

func TestStore_Execute_CacheError(t *testing.T) {
	cache := &mockSemanticCache{
		err: errors.New("cache store error"),
	}
	step := Store(cache, nil, nil)
	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:  core.NewQuery("q1", "test", nil),
		Answer: &core.Result{Answer: "Generated answer"},
	}

	err := step.Execute(ctx, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cache store error")
}
