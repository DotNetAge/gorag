package embedding

import (
	"context"
)

// BatchProvider wraps a Provider with batch processing optimization
type BatchProvider struct {
	provider  Provider
	processor *BatchProcessor
}

// NewBatchProvider creates a new batch provider
func NewBatchProvider(provider Provider, opts BatchOptions) *BatchProvider {
	return &BatchProvider{
		provider:  provider,
		processor: NewBatchProcessor(provider, opts),
	}
}

// Embed generates embeddings with batch processing optimization
// Implements the Provider interface
func (bp *BatchProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return bp.processor.Process(ctx, texts)
}

// Dimension returns the embedding dimension
// Implements the Provider interface
func (bp *BatchProvider) Dimension() int {
	return bp.provider.Dimension()
}

// ProcessWithProgress processes embeddings with progress tracking
func (bp *BatchProvider) ProcessWithProgress(
	ctx context.Context, 
	texts []string, 
	callback ProgressCallback,
) ([][]float32, error) {
	return bp.processor.ProcessWithProgress(ctx, texts, callback)
}

// GetProcessor returns the underlying batch processor
func (bp *BatchProvider) GetProcessor() *BatchProcessor {
	return bp.processor
}
