package core

import "context"

// Embedder defines the interface for generating vector embeddings.
type Embedder interface {
	// Embed encodes a text string into a vector.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch encodes multiple text strings into vectors in batch.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the dimension of the embedding vectors.
	Dimension() int
}

// MultimodalEmbedder encodes text and image inputs into a shared vector space,
type MultimodalEmbedder interface {
	Embedder
	// EmbedImage encodes raw image bytes (JPEG/PNG) into a vector in the shared embedding space.
	EmbedImage(ctx context.Context, imageData []byte) ([]float32, error)
}
