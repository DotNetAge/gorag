package cache

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/pkg/core"
)

type mockEmbedder struct {
	embeddings map[string][]float32
	dimension  int
	err        error
}

func newMockEmbedder() *mockEmbedder {
	return &mockEmbedder{
		embeddings: make(map[string][]float32),
		dimension:  4,
	}
}

func (m *mockEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := make([][]float32, len(texts))
	for i, text := range texts {
		if vec, ok := m.embeddings[text]; ok {
			result[i] = vec
			continue
		}
		vec := make([]float32, m.dimension)
		h := fnvHash(text)
		for j := 0; j < m.dimension; j++ {
			vec[j] = float32((h>>uint(j*3))&0xFF) / 255.0
		}
		m.embeddings[text] = vec
		result[i] = vec
	}
	return result, nil
}

func fnvHash(s string) uint64 {
	h := uint64(14695981039346656037)
	for _, c := range s {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

func (m *mockEmbedder) Dimension() int {
	return m.dimension
}

func TestInMemorySemanticCache_CheckCache_Miss(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb)

	ctx := context.Background()
	query := core.NewQuery("1", "什么是 Go 语言？", nil)

	result, err := cache.CheckCache(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hit {
		t.Fatal("expected cache miss")
	}
}

func TestInMemorySemanticCache_CacheResponse_ThenCheckCache_Hit(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb)

	ctx := context.Background()
	query := core.NewQuery("1", "什么是 Go 语言？", nil)
	answer := &core.Result{Answer: "Go 是一种编译型语言"}

	err := cache.CacheResponse(ctx, query, answer)
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
	if result.Answer != "Go 是一种编译型语言" {
		t.Fatalf("unexpected answer: %s", result.Answer)
	}
}

func TestInMemorySemanticCache_SimilarQuery(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb)

	ctx := context.Background()
	query1 := core.NewQuery("1", "如何学习 Go", nil)
	answer1 := &core.Result{Answer: "学习 Go 多写代码"}

	err := cache.CacheResponse(ctx, query1, answer1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	query2 := core.NewQuery("2", "怎么学习 Golang", nil)
	result, err := cache.CheckCache(ctx, query2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Hit {
		t.Log("similar query hit cache (similarity above threshold)")
	} else {
		t.Log("similar query did not hit cache (similarity below threshold)")
	}
}

func TestInMemorySemanticCache_TTLExpiry(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb,
		WithTTL(50*time.Millisecond),
	)

	ctx := context.Background()
	query := core.NewQuery("1", "测试 TTL", nil)
	answer := &core.Result{Answer: "TTL 测试答案"}

	err := cache.CacheResponse(ctx, query, answer)
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

func TestInMemorySemanticCache_MaxSizeLRU(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb,
		WithMaxSize(3),
		WithEvictPolicy(EvictLRU),
		WithThreshold(0.1),
	)

	ctx := context.Background()

	cache.CacheResponse(ctx, core.NewQuery("1", "query1", nil), &core.Result{Answer: "答案1"})
	cache.CacheResponse(ctx, core.NewQuery("2", "query2", nil), &core.Result{Answer: "答案2"})
	cache.CacheResponse(ctx, core.NewQuery("3", "query3", nil), &core.Result{Answer: "答案3"})

	if cache.Size() != 3 {
		t.Fatalf("expected size 3, got %d", cache.Size())
	}

	cache.CacheResponse(ctx, core.NewQuery("4", "query4", nil), &core.Result{Answer: "答案4"})

	if cache.Size() != 3 {
		t.Fatalf("expected size 3 after eviction, got %d", cache.Size())
	}
}

func TestInMemorySemanticCache_FIFOEviction(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb,
		WithMaxSize(3),
		WithEvictPolicy(EvictFIFO),
	)

	ctx := context.Background()

	texts := []string{"查询1", "查询2", "查询3"}
	for i, text := range texts {
		query := core.NewQuery(string(rune('0'+i)), text, nil)
		cache.CacheResponse(ctx, query, &core.Result{Answer: "答案"})
	}

	cache.CacheResponse(ctx, core.NewQuery("4", "查询4", nil), &core.Result{Answer: "新答案"})

	if cache.Size() != 3 {
		t.Fatalf("expected size 3, got %d", cache.Size())
	}
}

func TestInMemorySemanticCache_LFUEviction(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb,
		WithMaxSize(2),
		WithEvictPolicy(EvictLFU),
	)

	ctx := context.Background()

	q1 := core.NewQuery("1", "查询1", nil)
	q2 := core.NewQuery("2", "查询2", nil)

	cache.CacheResponse(ctx, q1, &core.Result{Answer: "答案1"})
	cache.CacheResponse(ctx, q2, &core.Result{Answer: "答案2"})

	cache.CheckCache(ctx, q1)
	cache.CheckCache(ctx, q1)

	cache.CacheResponse(ctx, core.NewQuery("3", "查询3", nil), &core.Result{Answer: "答案3"})

	if cache.Size() != 2 {
		t.Fatalf("expected size 2, got %d", cache.Size())
	}
}

func TestInMemorySemanticCache_CleanExpired(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb,
		WithTTL(100*time.Millisecond),
	)

	ctx := context.Background()

	q1 := core.NewQuery("1", "查询1", nil)
	q2 := core.NewQuery("2", "查询2", nil)

	cache.CacheResponse(ctx, q1, &core.Result{Answer: "答案1"})
	time.Sleep(150 * time.Millisecond)
	cache.CacheResponse(ctx, q2, &core.Result{Answer: "答案2"})

	removed, err := cache.CleanExpired(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 expired entry removed, got %d", removed)
	}
}

func TestInMemorySemanticCache_UpdateExisting(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb)

	ctx := context.Background()
	query := core.NewQuery("1", "测试查询", nil)

	cache.CacheResponse(ctx, query, &core.Result{Answer: "旧答案"})

	result1, _ := cache.CheckCache(ctx, query)
	if result1.Answer != "旧答案" {
		t.Fatalf("expected old answer, got %s", result1.Answer)
	}

	cache.CacheResponse(ctx, query, &core.Result{Answer: "新答案"})

	result2, _ := cache.CheckCache(ctx, query)
	if result2.Answer != "新答案" {
		t.Fatalf("expected new answer, got %s", result2.Answer)
	}

	if cache.Size() != 1 {
		t.Fatalf("expected size 1, got %d", cache.Size())
	}
}

func TestInMemorySemanticCache_NilQuery(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb)

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

func TestInMemorySemanticCache_EmptyQuery(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb)

	ctx := context.Background()

	result, err := cache.CheckCache(ctx, core.NewQuery("1", "", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Hit {
		t.Fatal("expected cache miss for empty query")
	}
}

func TestInMemorySemanticCache_EmbedError(t *testing.T) {
	emb := &mockEmbedder{err: errors.New("embed error")}
	cache := NewInMemorySemanticCache(emb)

	ctx := context.Background()
	query := core.NewQuery("1", "test", nil)

	_, err := cache.CheckCache(ctx, query)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestInMemorySemanticCache_ConcurrentAccess(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb, WithMaxSize(100))

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func(idx int) {
			defer wg.Done()
			query := core.NewQuery(string(rune('A'+idx)), "查询", nil)
			cache.CacheResponse(ctx, query, &core.Result{Answer: "答案"})
		}(i)

		go func(idx int) {
			defer wg.Done()
			query := core.NewQuery(string(rune('A'+idx)), "查询", nil)
			cache.CheckCache(ctx, query)
		}(i)
	}

	wg.Wait()

	if cache.Size() > 100 {
		t.Fatalf("size exceeded max: %d", cache.Size())
	}
}

func TestInMemorySemanticCache_Threshold(t *testing.T) {
	emb := newMockEmbedder()
	cache := NewInMemorySemanticCache(emb,
		WithThreshold(0.5),
	)

	ctx := context.Background()
	query1 := core.NewQuery("1", "AAA", nil)
	cache.CacheResponse(ctx, query1, &core.Result{Answer: "答案1"})

	query2 := core.NewQuery("2", "完全不同的查询 BBB", nil)
	result, _ := cache.CheckCache(ctx, query2)

	if result.Hit {
		t.Log("queries with different embeddings above threshold")
	} else {
		t.Log("queries with different embeddings below threshold")
	}
}
