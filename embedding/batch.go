package embedding

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/DotNetAge/gorag/internal/retry"
)

// BatchProcessor handles batch processing of embeddings with optimization
type BatchProcessor struct {
	provider      Provider
	batchSize     int
	maxWorkers    int
	rateLimiter   <-chan time.Time
	ticker        *time.Ticker
	retryConfig   RetryConfig
}

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryConfig returns default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}
}

// BatchOptions configures batch processing
type BatchOptions struct {
	BatchSize   int
	MaxWorkers  int
	RateLimit   time.Duration // Minimum time between requests
	RetryConfig RetryConfig
}

// DefaultBatchOptions returns default batch options
func DefaultBatchOptions() BatchOptions {
	return BatchOptions{
		BatchSize:   100,
		MaxWorkers:  runtime.NumCPU(),
		RateLimit:   100 * time.Millisecond,
		RetryConfig: DefaultRetryConfig(),
	}
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(provider Provider, opts BatchOptions) *BatchProcessor {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100
	}
	if opts.MaxWorkers <= 0 {
		opts.MaxWorkers = runtime.NumCPU()
	}
	if opts.RateLimit <= 0 {
		opts.RateLimit = 100 * time.Millisecond
	}

	ticker := time.NewTicker(opts.RateLimit)
	bp := &BatchProcessor{
		provider:    provider,
		batchSize:   opts.BatchSize,
		maxWorkers:  opts.MaxWorkers,
		ticker:      ticker,
		rateLimiter: ticker.C,
		retryConfig: opts.RetryConfig,
	}

	return bp
}

// Close stops the rate limiter ticker and releases resources
func (bp *BatchProcessor) Close() {
	if bp.ticker != nil {
		bp.ticker.Stop()
	}
}

// Process processes texts in optimized batches
func (bp *BatchProcessor) Process(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Split into batches
	batches := bp.splitBatches(texts)

	// Process batches concurrently
	results := make([][]float32, len(texts))
	errChan := make(chan error, len(batches))

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, bp.maxWorkers)

	for i, batch := range batches {
		wg.Add(1)
		semaphore <- struct{}{} // Acquire worker

		go func(batchIndex int, batchTexts []string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release worker

			// Rate limiting
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case <-bp.rateLimiter:
			}

			// Process batch with retry
			embeddings, err := bp.processBatchWithRetry(ctx, batchTexts)
			if err != nil {
				errChan <- fmt.Errorf("batch %d failed: %w", batchIndex, err)
				return
			}

			// Store results
			startIdx := batchIndex * bp.batchSize
			for j, embedding := range embeddings {
				results[startIdx+j] = embedding
			}
		}(i, batch)
	}

	// Wait for all batches to complete
	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// splitBatches splits texts into batches
func (bp *BatchProcessor) splitBatches(texts []string) [][]string {
	var batches [][]string
	for i := 0; i < len(texts); i += bp.batchSize {
		end := i + bp.batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batches = append(batches, texts[i:end])
	}
	return batches
}

// processBatchWithRetry processes a batch with retry logic
func (bp *BatchProcessor) processBatchWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error
	delay := bp.retryConfig.BaseDelay

	for attempt := 0; attempt <= bp.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}

			// Exponential backoff with jitter
			delay = time.Duration(float64(delay) * bp.retryConfig.Multiplier)
			if delay > bp.retryConfig.MaxDelay {
				delay = bp.retryConfig.MaxDelay
			}
		}

		embeddings, err := bp.provider.Embed(ctx, texts)
		if err == nil {
			return embeddings, nil
		}

		lastErr = err
		if !retry.IsRetryableError(err) {
			break
		}
	}

	return nil, fmt.Errorf("failed after %d retries: %w", bp.retryConfig.MaxRetries, lastErr)
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Stats returns batch processor statistics
type Stats struct {
	TotalBatches    int
	ProcessedBatches int
	FailedBatches   int
	TotalTexts      int
	ProcessingTime  time.Duration
}

// ProgressCallback is called during batch processing
type ProgressCallback func(processed, total int)

// ProcessWithProgress processes with progress tracking
func (bp *BatchProcessor) ProcessWithProgress(
	ctx context.Context,
	texts []string,
	callback ProgressCallback,
) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	batches := bp.splitBatches(texts)
	results := make([][]float32, len(texts))
	var processed int
	var mu sync.Mutex

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, bp.maxWorkers)
	errChan := make(chan error, len(batches))

	for i, batch := range batches {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(batchIndex int, batchTexts []string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case <-bp.rateLimiter:
			}

			embeddings, err := bp.processBatchWithRetry(ctx, batchTexts)
			if err != nil {
				errChan <- fmt.Errorf("batch %d failed: %w", batchIndex, err)
				return
			}

			startIdx := batchIndex * bp.batchSize
			for j, embedding := range embeddings {
				results[startIdx+j] = embedding
			}

			mu.Lock()
			processed += len(batchTexts)
			if callback != nil {
				callback(processed, len(texts))
			}
			mu.Unlock()
		}(i, batch)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
