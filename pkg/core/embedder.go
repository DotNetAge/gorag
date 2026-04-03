package core

import "context"

// Embedder defines the interface for generating vector embeddings from text.
// It provides both single and batch embedding capabilities for document vectorization.
type Embedder interface {
	// Embed encodes a text string into a vector.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch encodes multiple text strings into vectors in batch.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the dimension of the embedding vectors.
	Dimension() int
}
