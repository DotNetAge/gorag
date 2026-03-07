package parser

import (
	"context"
	"io"

	"github.com/DotNetAge/gorag/core"
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
//     func (p *TextParser) Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error) {
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
	// - []core.Chunk: Slice of parsed document chunks
	// - error: Error if parsing fails
	Parse(ctx context.Context, r io.Reader) ([]core.Chunk, error)
	
	// SupportedFormats returns the file formats supported by this parser
	//
	// Returns:
	// - []string: Slice of supported file extensions (e.g., [".txt", ".md"])
	SupportedFormats() []string
}
