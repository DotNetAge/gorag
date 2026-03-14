package dataprep

import (
	"context"
	"io"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// Parser defines the streaming document parser for Next-Gen RAG.
type Parser interface {
	// ParseStream reads from an io.Reader and streams parsed Document objects.
	// This ensures O(1) memory complexity for handling massive files (e.g., 2GB logs).
	ParseStream(ctx context.Context, r io.Reader, metadata map[string]any) (<-chan *entity.Document, error)
	
	// GetSupportedTypes returns the file extensions or MIME types this parser supports.
	GetSupportedTypes() []string
}
