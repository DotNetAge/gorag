package embedding

import (
	"context"
	"testing"
	"time"
)

// MockProvider is a mock embedding provider for testing
type MockProvider struct {
	dimension int
	delay     time.Duration
}

func NewMockProvider(dimension int, delay time.Duration) *MockProvider {
	return &MockProvider{
		dimension: dimension,
		delay:     delay,
	}
}

func (m *MockProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// Simulate API delay
	time.Sleep(m.delay)

	// Generate mock embeddings
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embedding := make([]float32, m.dimension)
		for j := range embedding {
			embedding[j] = float32(i*10 + j)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

func (m *MockProvider) Dimension() int {
	return m.dimension
}

func TestBatchProcessor(t *testing.T) {
	// Create mock provider
	provider := NewMockProvider(3, 10*time.Millisecond)

	// Create batch processor
	processor := NewBatchProcessor(provider, BatchOptions{
		BatchSize:  2,
		MaxWorkers: 2,
		RateLimit:  5 * time.Millisecond,
		RetryConfig: RetryConfig{
			MaxRetries: 3,
			BaseDelay:  10 * time.Millisecond,
			MaxDelay:   100 * time.Millisecond,
			Multiplier: 2.0,
		},
	})

	// Test texts
	texts := []string{
		"text 1",
		"text 2",
		"text 3",
		"text 4",
	}

	ctx := context.Background()

	// Test Process
	start := time.Now()
	embeddings, err := processor.Process(ctx, texts)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	duration := time.Since(start)

	if len(embeddings) != 4 {
		t.Errorf("Expected 4 embeddings, got %d", len(embeddings))
	}

	// Check that processing was faster than sequential
	expectedSequentialTime := 4 * 10 * time.Millisecond
	if duration >= expectedSequentialTime {
		t.Errorf("Expected parallel processing to be faster than %v, got %v", expectedSequentialTime, duration)
	}

	// Test ProcessWithProgress
	var progressCalls int
	embeddings, err = processor.ProcessWithProgress(ctx, texts, func(processed, total int) {
		progressCalls++
		if processed < 0 || processed > total {
			t.Errorf("Invalid progress: processed=%d, total=%d", processed, total)
		}
	})
	if err != nil {
		t.Fatalf("ProcessWithProgress failed: %v", err)
	}

	if progressCalls == 0 {
		t.Error("Expected progress callback to be called")
	}

	if len(embeddings) != 4 {
		t.Errorf("Expected 4 embeddings, got %d", len(embeddings))
	}
}

func TestBatchProvider(t *testing.T) {
	// Create mock provider
	provider := NewMockProvider(3, 5*time.Millisecond)

	// Create batch provider
	batchProvider := NewBatchProvider(provider, BatchOptions{
		BatchSize:  2,
		MaxWorkers: 2,
		RateLimit:  2 * time.Millisecond,
	})

	// Test texts
	texts := []string{
		"text 1",
		"text 2",
	}

	ctx := context.Background()

	// Test Embed
	embeddings, err := batchProvider.Embed(ctx, texts)
	if err != nil {
		t.Fatalf("Embed failed: %v", err)
	}

	if len(embeddings) != 2 {
		t.Errorf("Expected 2 embeddings, got %d", len(embeddings))
	}

	// Test Dimension
	if batchProvider.Dimension() != 3 {
		t.Errorf("Expected dimension 3, got %d", batchProvider.Dimension())
	}

	// Test GetProcessor
	processor := batchProvider.GetProcessor()
	if processor == nil {
		t.Error("Expected to get processor")
	}
}
