// Package abstraction defines the storage abstraction interfaces for the goRAG framework.
package abstraction

import "context"

// MultimodalEmbedder encodes text and image inputs into a shared vector space,
// enabling cross-modal retrieval (e.g. text query → image results and vice-versa).
//
// Implementations should use models that project both modalities into the same
// embedding space, such as CLIP, BLIP-2, or similar vision-language models.
//
// ImageData is expected to be raw bytes of a JPEG or PNG image.
// When ImageData is nil, EmbedImage should return a zero/null vector or an error.
type MultimodalEmbedder interface {
	// EmbedText encodes a plain-text string into a vector in the shared embedding space.
	EmbedText(ctx context.Context, text string) ([]float32, error)

	// EmbedImage encodes raw image bytes (JPEG/PNG) into a vector in the shared embedding space.
	// Implementations may return an error if imageData is nil or malformed.
	EmbedImage(ctx context.Context, imageData []byte) ([]float32, error)
}
