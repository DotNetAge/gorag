package parser

import (
	"context"
	"io"
)

// Parser defines the interface for document parsers
type Parser interface {
	Parse(ctx context.Context, r io.Reader) ([]Chunk, error)
	SupportedFormats() []string
}

// Chunk represents a parsed document chunk
type Chunk struct {
	ID       string
	Content  string
	Metadata map[string]string
}
