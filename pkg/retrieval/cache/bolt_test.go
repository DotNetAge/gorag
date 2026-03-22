package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/pkg/core"
)

type mockEmbedderForBolt struct {
	embeddings map[string][]float32
	dimension  int
	err        error
}

func newMockEmbedderForBolt() *mockEmbedderForBolt {
	return &mockEmbedderForBolt{
		embeddings: make(map[string][]float32),
		dimension:  4,
	}
}

func (m *mockEmbedderForBolt) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	if vec, ok := m.embeddings[text]; ok {
		return vec, nil
	}
	vec := make([]float32, m.dimension)
	h := fnvHash(text)
	for i := 0; i < m.dimension; i++ {
		vec[i] = float32((h>>uint(i*3))&0xFF) / 255.0
	}
	m.embeddings[text] = vec
	return vec, nil
}

func (m *mockEmbedderForBolt) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, 0, len(texts))
	for _, text := range texts {
		vec, err := m.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		result = append(result, vec)
	}
	return result, nil
}

func (m *mockEmbedderForBolt) Dimension() int {
	return m.dimension
}

func TestBoltSemanticCache_CheckCache_Miss(t *testing.T) {
	emb := newMockEmbedderForBolt()
	tmpFile := t.TempDir() + "/test_cache.db"
	cache, err := NewBoltSemanticCache(emb, WithBoltDBPath(tmpFile))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	query := core.NewQuery("1", "test query", nil)

	result, err := cache.CheckCache(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hit {
		t.Fatal("expected cache miss")
	}
}

func TestBoltSemanticCache_CacheResponse_ThenCheckCache_Hit(t *testing.T) {
	emb := newMockEmbedderForBolt()
	tmpFile := t.TempDir() + "/test_cache.db"
	cache, err := NewBoltSemanticCache(emb, WithBoltDBPath(tmpFile))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	query := core.NewQuery("1", "test query", nil)
	answer := &core.Result{Answer: "test answer"}

	err = cache.CacheResponse(ctx, query, answer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := cache.CheckCache(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Hit {
		t.Fatal("expected cache hit")
	}
	if result.Answer != "test answer" {
		t.Fatalf("unexpected answer: %s", result.Answer)
	}
}

func TestBoltSemanticCache_TTLExpiry(t *testing.T) {
	emb := newMockEmbedderForBolt()
	tmpFile := t.TempDir() + "/test_cache.db"
	cache, err := NewBoltSemanticCache(emb,
		WithBoltDBPath(tmpFile),
	)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.config.TTL = 50 * time.Millisecond

	ctx := context.Background()
	query := core.NewQuery("1", "test query", nil)
	answer := &core.Result{Answer: "test answer"}

	err = cache.CacheResponse(ctx, query, answer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	result, err := cache.CheckCache(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hit {
		t.Fatal("expected cache miss after TTL expiry")
	}
}

func TestBoltSemanticCache_CleanExpired(t *testing.T) {
	emb := newMockEmbedderForBolt()
	tmpFile := t.TempDir() + "/test_cache.db"
	cache, err := NewBoltSemanticCache(emb,
		WithBoltDBPath(tmpFile),
	)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	cache.config.TTL = 100 * time.Millisecond

	ctx := context.Background()

	cache.CacheResponse(ctx, core.NewQuery("1", "query1", nil), &core.Result{Answer: "answer1"})

	time.Sleep(150 * time.Millisecond)

	cache.CacheResponse(ctx, core.NewQuery("2", "query2", nil), &core.Result{Answer: "answer2"})

	removed, err := cache.CleanExpired(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed < 1 {
		t.Fatalf("expected at least 1 expired entry removed, got %d", removed)
	}
}

func TestBoltSemanticCache_Size(t *testing.T) {
	emb := newMockEmbedderForBolt()
	tmpFile := t.TempDir() + "/test_cache.db"
	cache, err := NewBoltSemanticCache(emb, WithBoltDBPath(tmpFile))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	cache.CacheResponse(ctx, core.NewQuery("1", "q1_unique", nil), &core.Result{Answer: "a1"})
	cache.CacheResponse(ctx, core.NewQuery("2", "q2_unique", nil), &core.Result{Answer: "a2"})

	size, err := cache.Size()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size < 2 {
		t.Fatalf("expected at least 2, got %d", size)
	}
}

func TestBoltSemanticCache_Close(t *testing.T) {
	emb := newMockEmbedderForBolt()
	tmpFile := t.TempDir() + "/test_cache.db"
	cache, err := NewBoltSemanticCache(emb, WithBoltDBPath(tmpFile))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	err = cache.Close()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Log("db file removed after close (expected behavior)")
	}
}

func TestBoltSemanticCache_NilQuery(t *testing.T) {
	emb := newMockEmbedderForBolt()
	tmpFile := t.TempDir() + "/test_cache.db"
	cache, err := NewBoltSemanticCache(emb, WithBoltDBPath(tmpFile))
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	result, err := cache.CheckCache(ctx, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hit {
		t.Fatal("expected cache miss for nil query")
	}

	err = cache.CacheResponse(ctx, nil, &core.Result{Answer: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
