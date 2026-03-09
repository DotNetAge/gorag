package vectorstore

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/DotNetAge/gorag/core"
)

// mockStore is a mock vector store for testing
type mockStore struct {
	addCalled        bool
	searchCalled     bool
	deleteCalled     bool
	searchMetaCalled bool
	closeCalled      bool
}

func (m *mockStore) Add(ctx context.Context, chunks []core.Chunk, embeddings [][]float32) error {
	m.addCalled = true
	return nil
}

func (m *mockStore) Search(ctx context.Context, query []float32, opts SearchOptions) ([]core.Result, error) {
	m.searchCalled = true
	return []core.Result{{
		Chunk: core.Chunk{
			ID:      "1",
			Content: "test",
		},
		Score: 0.9,
	}}, nil
}

func (m *mockStore) Delete(ctx context.Context, ids []string) error {
	m.deleteCalled = true
	return nil
}

func (m *mockStore) SearchByMetadata(ctx context.Context, metadata map[string]string) ([]core.Chunk, error) {
	m.searchMetaCalled = true
	return []core.Chunk{}, nil
}

func (m *mockStore) Close() error {
	m.closeCalled = true
	return nil
}

func TestPooledStore(t *testing.T) {
	// Create mock store
	mock := &mockStore{}

	// Create pooled store
	pooledStore := NewPooledStore(mock, PoolOptions{
		MaxConns:    5,
		IdleConns:   2,
		IdleTimeout: 1 * time.Second,
	})

	// Test Add
	chunks := []core.Chunk{
		{
			ID:      "1",
			Content: "test content",
			Metadata: map[string]string{
				"type": "text",
			},
		},
	}
	embeddings := [][]float32{{1.0, 2.0, 3.0}}

	ctx := context.Background()
	err := pooledStore.Add(ctx, chunks, embeddings)
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if !mock.addCalled {
		t.Error("Expected Add to be called")
	}

	// Test Search
	results, err := pooledStore.Search(ctx, []float32{1.0, 2.0, 3.0}, SearchOptions{
		TopK: 1,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if !mock.searchCalled {
		t.Error("Expected Search to be called")
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Test Delete
	err = pooledStore.Delete(ctx, []string{"1"})
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !mock.deleteCalled {
		t.Error("Expected Delete to be called")
	}

	// Test SearchByMetadata
	chunks2, err := pooledStore.SearchByMetadata(ctx, map[string]string{"type": "text"})
	if err != nil {
		t.Fatalf("SearchByMetadata failed: %v", err)
	}
	if !mock.searchMetaCalled {
		t.Error("Expected SearchByMetadata to be called")
	}
	if len(chunks2) != 0 {
		t.Errorf("Expected 0 chunks, got %d", len(chunks2))
	}

	// Test Stats
	stats := pooledStore.Stats()
	if stats.MaxConnections != 5 {
		t.Errorf("Expected max connections 5, got %d", stats.MaxConnections)
	}

	// Test Close
	err = pooledStore.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestPooledStoreConcurrent(t *testing.T) {
	// Create mock store
	mock := &mockStore{}

	// Create pooled store
	pooledStore := NewPooledStore(mock, PoolOptions{
		MaxConns:    3,
		IdleConns:   1,
		IdleTimeout: 1 * time.Second,
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	errorChan := make(chan error, 10)

	// Run concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			// Add chunk
			chunks := []core.Chunk{
				{
					ID:      fmt.Sprintf("chunk-%d", i),
					Content: fmt.Sprintf("content-%d", i),
					Metadata: map[string]string{
						"type": "text",
					},
				},
			}
			embeddings := [][]float32{{1.0, 2.0, 3.0}}

			err := pooledStore.Add(ctx, chunks, embeddings)
			if err != nil {
				errorChan <- err
				return
			}

			// Search
			_, err = pooledStore.Search(ctx, []float32{1.0, 2.0, 3.0}, SearchOptions{
				TopK: 1,
			})
			if err != nil {
				errorChan <- err
				return
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Test Close
	err := pooledStore.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}
