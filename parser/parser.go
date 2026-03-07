package parser

import (
	"context"
	"io"
)

// Parser defines the interface for document parsers
//
// This interface is implemented by all document parsers
// (Text, PDF, DOCX, HTML, JSON, YAML, Excel, PPT, Image)
// and allows the RAG engine to parse different file formats.
//
// Example implementation:
//
//     type TextParser struct {
//         chunkSize    int
//         chunkOverlap int
//     }
//
//     func (p *TextParser) Parse(ctx context.Context, r io.Reader) ([]Chunk, error) {
//         // Parse text into chunks
//     }
//
//     func (p *TextParser) SupportedFormats() []string {
//         return []string{".txt", ".md", ".text"}
//     }
type Parser interface {
	// Parse parses the input reader into document chunks
	//
	// Parameters:
	// - ctx: Context for cancellation and timeout
	// - r: Reader containing the document content
	//
	// Returns:
	// - []Chunk: Slice of parsed document chunks
	// - error: Error if parsing fails
	Parse(ctx context.Context, r io.Reader) ([]Chunk, error)
	
	// SupportedFormats returns the file formats supported by this parser
	//
	// Returns:
	// - []string: Slice of supported file extensions (e.g., [".txt", ".md"])
	SupportedFormats() []string
}

// Chunk represents a parsed document chunk
//
// A Chunk is a piece of a document that has been parsed and
// prepared for embedding and storage in the vector store.
//
// Example:
//
//     chunk := Chunk{
//         ID:       "chunk-1",
//         Content:  "Go is an open source programming language...",
//         Metadata: map[string]string{
//             "source": "example.txt",
//             "page":   "1",
//         },
//         MediaType: "text/plain",
//     }
type Chunk struct {
	ID         string            // Unique identifier for the chunk
	Content    string            // Text content of the chunk
	Metadata   map[string]string // Metadata about the chunk (source, position, etc.)
	MediaType  string            // Media type (e.g., "text/plain", "image/jpeg")
	MediaData  []byte            // Binary data for non-text content (e.g., images)
}
