package embedding

import (
	"context"
)

// Provider defines the interface for embedding providers
type Provider interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}
