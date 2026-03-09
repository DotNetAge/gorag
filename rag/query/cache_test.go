package query

import (
	"context"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
)

func TestMemoryCache(t *testing.T) {
	// Create cache with 1 second TTL
	cache := NewMemoryCache(1 * time.Second)

	ctx := context.Background()
	key := "test-key"
	value := &Response{
		Answer: "test answer",
		Sources: []core.Result{
			{
				Chunk: core.Chunk{
					ID:      "1",
					Content: "test content",
				},
				Score: 0.9,
			},
		},
	}

	// Test Set
	cache.Set(ctx, key, value, 1*time.Second)

	// Test Get
	result, found := cache.Get(ctx, key)
	if !found {
		t.Error("Expected to find key in cache")
	}
	if result.Answer != "test answer" {
		t.Errorf("Expected answer 'test answer', got '%s'", result.Answer)
	}

	// Test Stats
	stats := cache.Stats()
	if stats.Size != 1 {
		t.Errorf("Expected size 1, got %d", stats.Size)
	}

	// Test Delete
	cache.Delete(ctx, key)
	_, found = cache.Get(ctx, key)
	if found {
		t.Error("Expected key to be deleted")
	}

	// Test Clear
	cache.Set(ctx, key, value, 1*time.Second)
	cache.Clear(ctx)
	_, found = cache.Get(ctx, key)
	if found {
		t.Error("Expected cache to be cleared")
	}
}

func TestMemoryCacheTTL(t *testing.T) {
	// Create cache with 50ms TTL
	cache := NewMemoryCache(50 * time.Millisecond)

	ctx := context.Background()
	key := "test-key"
	value := &Response{
		Answer: "test answer",
	}

	// Test Set
	cache.Set(ctx, key, value, 50*time.Millisecond)

	// Test Get before TTL
	_, found := cache.Get(ctx, key)
	if !found {
		t.Error("Expected to find key in cache before TTL")
	}

	// Wait for TTL
	time.Sleep(100 * time.Millisecond)

	// Test Get after TTL
	_, found = cache.Get(ctx, key)
	if found {
		t.Error("Expected key to be expired")
	}
}

func TestMemoryCacheSizeLimit(t *testing.T) {
	// Create cache with size limit 2
	cache := NewMemoryCacheWithSize(1*time.Second, 2)

	ctx := context.Background()

	// Add 3 items
	cache.Set(ctx, "key1", &Response{Answer: "answer1"}, 1*time.Second)
	cache.Set(ctx, "key2", &Response{Answer: "answer2"}, 1*time.Second)
	cache.Set(ctx, "key3", &Response{Answer: "answer3"}, 1*time.Second)

	// Check size
	stats := cache.Stats()
	if stats.Size != 2 {
		t.Errorf("Expected size 2, got %d", stats.Size)
	}

	// Check that key1 is evicted
	_, found := cache.Get(ctx, "key1")
	if found {
		t.Error("Expected key1 to be evicted")
	}

	// Check that key2 and key3 are present
	_, found = cache.Get(ctx, "key2")
	if !found {
		t.Error("Expected key2 to be present")
	}
	_, found = cache.Get(ctx, "key3")
	if !found {
		t.Error("Expected key3 to be present")
	}
}
