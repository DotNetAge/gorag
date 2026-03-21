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

// MultimodalEmbedder extends Embedder to support multi-modal inputs (text and images).
// It encodes both text and image inputs into a shared vector space for cross-modal retrieval.
type MultimodalEmbedder interface {
	Embedder
	// EmbedImage encodes raw image bytes (JPEG/PNG) into a vector in the shared embedding space.
	EmbedImage(ctx context.Context, imageData []byte) ([]float32, error)
}
