package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/DotNetAge/gorag/pkg/core"
)

type mockRetriever struct {
	results []*core.RetrievalResult
	err    error
}

func (m *mockRetriever) Retrieve(ctx context.Context, queries []string, topK int) ([]*core.RetrievalResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	results := make([]*core.RetrievalResult, 0, len(queries))
	for _, q := range queries {
		for _, r := range m.results {
			if r.Query == q {
				results = append(results, r)
				break
			}
		}
	}
	return results, nil
}

func TestRetrieverWithCache_Retrieve_Miss(t *testing.T) {
	emb := newMockEmbedder()
	mockCache := NewInMemorySemanticCache(emb)

	retriever := &mockRetriever{
		results: []*core.RetrievalResult{
			{Query: "test", Answer: "retrieved answer"},
		},
	}

	wrapped := NewRetrieverWithCache(retriever, mockCache, nil)

	ctx := context.Background()
	results, err := wrapped.Retrieve(ctx, []string{"test"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Answer != "retrieved answer" {
		t.Fatalf("unexpected answer: %s", results[0].Answer)
	}
}

func TestRetrieverWithCache_Retrieve_Hit(t *testing.T) {
	emb := newMockEmbedder()
	mockCache := NewInMemorySemanticCache(emb)

	ctx := context.Background()
	mockCache.CacheResponse(ctx, core.NewQuery("1", "cached query", nil), &core.Result{Answer: "cached answer"})

	retriever := &mockRetriever{
		results: []*core.RetrievalResult{
			{Query: "cached query", Answer: "retrieved answer"},
		},
	}

	wrapped := NewRetrieverWithCache(retriever, mockCache, nil)

	results, err := wrapped.Retrieve(ctx, []string{"cached query"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Answer != "cached answer" {
		t.Fatalf("expected cached answer, got: %s", results[0].Answer)
	}
	if results[0].Metadata == nil {
		t.Fatal("expected metadata with cache_hit=true")
	}
	if results[0].Metadata["cache_hit"] != true {
		t.Fatal("expected cache_hit=true in metadata")
	}
}

func TestRetrieverWithCache_Retrieve_Error(t *testing.T) {
	emb := newMockEmbedder()
	mockCache := NewInMemorySemanticCache(emb)

	retriever := &mockRetriever{
		err: errors.New("retriever error"),
	}

	wrapped := NewRetrieverWithCache(retriever, mockCache, nil)

	ctx := context.Background()
	_, err := wrapped.Retrieve(ctx, []string{"test"}, 5)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRetrieverWithCache_Retrieve_MultipleQueries(t *testing.T) {
	emb := newMockEmbedder()
	mockCache := NewInMemorySemanticCache(emb)

	ctx := context.Background()
	mockCache.CacheResponse(ctx, core.NewQuery("1", "query1", nil), &core.Result{Answer: "cached1"})

	retriever := &mockRetriever{
		results: []*core.RetrievalResult{
			{Query: "query1", Answer: "retrieved1"},
			{Query: "query2", Answer: "retrieved2"},
		},
	}

	wrapped := NewRetrieverWithCache(retriever, mockCache, nil)

	results, err := wrapped.Retrieve(ctx, []string{"query1", "query2"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}
