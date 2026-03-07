package rag

import "github.com/DotNetAge/gorag/core"

// StreamResponse represents a streaming RAG query response
type StreamResponse struct {
	Chunk   string
	Sources []core.Result
	Done    bool
	Error   error
}

// Response represents the RAG query response
type Response struct {
	Answer  string
	Sources []core.Result
}
