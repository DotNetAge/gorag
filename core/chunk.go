// Package core contains core data structures and interfaces for GoRAG
//
// This package defines the fundamental types used throughout the RAG engine,
// including chunks, search results, and document sources.
package core

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

// Result represents a search result with relevance score
//
// A Result is a Chunk with an associated relevance score from the vector search.
// The score indicates how relevant the chunk is to the query.
//
// Example:
//
//     result := Result{
//         Chunk: Chunk{
//             ID:      "chunk-1",
//             Content: "Go is an open source programming language...",
//         },
//         Score: 0.95, // High relevance
//     }
type Result struct {
	Chunk
	Score float32 // Relevance score (0.0-1.0)
}

// SearchOptions configures search behavior
//
// This struct defines options for vector search operations, including
// the number of results to return, filters, and minimum score threshold.
//
// Example:
//
//     options := SearchOptions{
//         TopK:     5,
//         MinScore: 0.7,
//         Filter: map[string]interface{}{
//             "source": "technical-docs",
//         },
//     }
type SearchOptions struct {
	TopK     int                    // Number of top results to return
	Filter   map[string]any // Metadata filters
	MinScore float32                // Minimum relevance score
}

// Source represents a document source for indexing
//
// A Source defines the input for the indexing process. It can represent:
// 1. A text string (Content field)
// 2. A file path (Path field)
// 3. A reader interface (Reader field)
//
// The Type field specifies the document format (e.g., ".txt", ".pdf", ".docx")
// and is used to select the appropriate parser.
//
// Example:
//
//     // Index a text string
//     source1 := Source{
//         Type:    "text",
//         Content: "Go is an open source programming language...",
//     }
//
//     // Index a file
//     source2 := Source{
//         Type: ".pdf",
//         Path: "/path/to/document.pdf",
//     }
type Source struct {
	Type    string      // Document type/format (e.g., ".txt", ".pdf")
	Path    string      // File path (if loading from file)
	Content string      // Text content (if loading from string)
	Reader  any // Reader interface (if loading from reader)
}
