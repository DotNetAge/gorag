package rag

import "github.com/DotNetAge/gorag/vectorstore"

// StreamResponse represents a streaming RAG query response
type StreamResponse struct {
	Chunk   string
	Sources []vectorstore.Result
	Done    bool
	Error   error
}

// Response represents the RAG query response
type Response struct {
	Answer  string
	Sources []vectorstore.Result
}
