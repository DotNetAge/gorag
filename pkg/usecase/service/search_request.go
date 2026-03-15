package service

import "github.com/DotNetAge/gorag/pkg/domain/entity"

// SearchRequest is the DTO for search use case input.
type SearchRequest struct {
	// Query is the user's search query
	Query *entity.Query

	// TopK is the number of chunks to retrieve (default: 5)
	TopK int

	// UserID identifies the user making the request
	UserID string

	// SessionID tracks the conversation session
	SessionID string
}

// ChatRequest is the DTO for chat use case input.
type ChatRequest struct {
	// Message is the user's chat message
	Message string

	// UserID identifies the user
	UserID string

	// SessionID tracks the conversation session
	SessionID string

	// History contains previous conversation messages
	History []string
}

// IndexRequest is the DTO for indexing use case input.
type IndexRequest struct {
	// Documents to index
	Documents []*entity.Document

	// Collection is the target collection name
	Collection string

	// BatchSize for bulk indexing (default: 100)
	BatchSize int
}
