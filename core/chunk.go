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
type Result struct {
	Chunk
	Score float32
}

// SearchOptions configures search behavior
type SearchOptions struct {
	TopK      int
	Filter    map[string]interface{}
	MinScore  float32
}

// Source represents a document source
type Source struct {
	Type    string
	Path    string
	Content string
	Reader  interface{}
}
