package core

import (
	"context"
	"io"
)

// Parser defines the interface for document parsing implementations.
// Parsers convert various file formats (PDF, DOCX, Markdown, etc.) into structured Document objects.
// They support both batch and streaming parsing modes for handling files of any size.
//
// Implementations should:
//   - Extract text content from various file formats
//   - Preserve metadata (title, author, creation date, etc.)
//   - Handle encoding and format-specific features
//   - Support streaming for large files to avoid memory issues
//
// Example usage:
//
//	parser := NewPDFParser()
//	if parser.Supports("application/pdf") {
//	    doc, err := parser.Parse(ctx, content, metadata)
//	    // Process the parsed document
//	}
type Parser interface {
	// Parse converts raw content into a Document object.
	// This is suitable for small to medium-sized files that fit in memory.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - content: Raw file content as bytes
	//   - metadata: Initial metadata to include in the document
	//
	// Returns:
	//   - *Document: The parsed document
	//   - error: Any error that occurred during parsing
	Parse(ctx context.Context, content []byte, metadata map[string]any) (*Document, error)

	// Supports checks if this parser can handle the given content type.
	// Content types should be MIME types (e.g., "application/pdf", "text/markdown").
	//
	// Parameters:
	//   - contentType: The MIME type to check
	//
	// Returns:
	//   - bool: True if this parser supports the content type
	Supports(contentType string) bool

	// ParseStream parses content from a reader in streaming fashion.
	// This is recommended for large files to avoid loading everything into memory.
	// It returns a channel that emits documents as they are parsed.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - reader: Source of the content to parse
	//   - metadata: Initial metadata to include in documents
	//
	// Returns:
	//   - <-chan *Document: Channel emitting parsed documents
	//   - error: Any error that occurred during stream setup
	ParseStream(ctx context.Context, reader io.Reader, metadata map[string]any) (<-chan *Document, error)

	// GetSupportedTypes returns all content types this parser can handle.
	// This is useful for registration and routing logic.
	//
	// Returns:
	//   - []string: List of supported MIME types
	GetSupportedTypes() []string
}
