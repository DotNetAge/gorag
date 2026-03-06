package core

import "time"

// Chunk represents a piece of text content with metadata
type Chunk struct {
	ID       string
	Content  string
	Metadata Metadata
}

// Metadata contains additional information about a chunk
type Metadata struct {
	Source   string
	Page     int
	Position int
	Type     string
	CreatedAt time.Time
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
