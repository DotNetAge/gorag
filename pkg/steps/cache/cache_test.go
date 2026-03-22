package cache

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
)

type mockSemanticCache struct {
	result *core.CacheResult
	err    error
}

func (m *mockSemanticCache) CheckCache(ctx context.Context, query *core.Query) (*core.CacheResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result == nil {
		return &core.CacheResult{Hit: false}, nil
	}
	return m.result, nil
}

func (m *mockSemanticCache) CacheResponse(ctx context.Context, query *core.Query, answer *core.Result) error {
	if m.err != nil {
		return m.err
	}
	return nil
}

func TestCheckStep_Name(t *testing.T) {
	mockCache := &mockSemanticCache{}
	step := Check(mockCache, nil, nil)
	if step.Name() != "SemanticCacheCheck" {
		t.Fatalf("expected SemanticCacheCheck, got %s", step.Name())
	}
}

func TestCheckStep_Execute_CacheMiss(t *testing.T) {
	mockCache := &mockSemanticCache{
		result: &core.CacheResult{Hit: false},
	}
	step := Check(mockCache, nil, nil)

	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Answer != nil {
		t.Fatal("expected nil answer on cache miss")
	}
}

func TestCheckStep_Execute_CacheHit(t *testing.T) {
	mockCache := &mockSemanticCache{
		result: &core.CacheResult{Hit: true, Answer: "cached answer"},
	}
	step := Check(mockCache, nil, nil)

	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if state.Answer == nil {
		t.Fatal("expected answer on cache hit")
	}
	if state.Answer.Answer != "cached answer" {
		t.Fatalf("expected cached answer, got %s", state.Answer.Answer)
	}
	if state.Agentic == nil {
		t.Fatal("expected Agentic state to be set")
	}
	if !state.Agentic.CacheHit {
		t.Fatal("expected CacheHit=true")
	}
}

func TestCheckStep_Execute_NilQuery(t *testing.T) {
	mockCache := &mockSemanticCache{}
	step := Check(mockCache, nil, nil)

	ctx := context.Background()
	state := &core.RetrievalContext{}

	err := step.Execute(ctx, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckStep_Execute_EmptyQuery(t *testing.T) {
	mockCache := &mockSemanticCache{}
	step := Check(mockCache, nil, nil)

	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "", nil),
	}

	err := step.Execute(ctx, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreStep_Name(t *testing.T) {
	mockCache := &mockSemanticCache{}
	step := Store(mockCache, nil, nil)
	if step.Name() != "SemanticCacheStore" {
		t.Fatalf("expected SemanticCacheStore, got %s", step.Name())
	}
}

func TestStoreStep_Execute(t *testing.T) {
	mockCache := &mockSemanticCache{}
	step := Store(mockCache, nil, nil)

	ctx := context.Background()
	state := &core.RetrievalContext{
		Query:   core.NewQuery("1", "test query", nil),
		Answer:  &core.Result{Answer: "test answer"},
	}

	err := step.Execute(ctx, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreStep_Execute_NilQuery(t *testing.T) {
	mockCache := &mockSemanticCache{}
	step := Store(mockCache, nil, nil)

	ctx := context.Background()
	state := &core.RetrievalContext{
		Answer: &core.Result{Answer: "test answer"},
	}

	err := step.Execute(ctx, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStoreStep_Execute_NilAnswer(t *testing.T) {
	mockCache := &mockSemanticCache{}
	step := Store(mockCache, nil, nil)

	ctx := context.Background()
	state := &core.RetrievalContext{
		Query: core.NewQuery("1", "test query", nil),
	}

	err := step.Execute(ctx, state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
