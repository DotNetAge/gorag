package parser

import (
	"context"
	"io"

	"github.com/DotNetAge/gorag/core"
)

// StreamingParser defines the interface for streaming document parsers
// This interface extends the Parser interface to support streaming processing
// for large files without loading the entire file into memory

type StreamingParser interface {
	Parser
	// ParseWithCallback parses the document and calls the callback for each chunk
	// This allows processing chunks as they are parsed, without storing all chunks in memory
	ParseWithCallback(ctx context.Context, r io.Reader, callback func(core.Chunk) error) error
}
