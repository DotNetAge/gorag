// Package entity defines the core entities for the goRAG framework.
package entity

import (
	"time"
)

// Query represents a query entity in the RAG system.
// It contains the user's query and related metadata.
//
// Related RAG concepts:
// - Query Understanding & Intent: Represents the user's query intent
// - HyDE: Used as input for Hypothetical Document Embeddings
// - RAG-Fusion: Used as input for multi-query generation
// - Query Rewriting: Can be rewritten to improve retrieval effectiveness
type Query struct {
	ID        string                 `json:"id"`        // Unique identifier for the query
	Text      string                 `json:"text"`      // The actual query text
	Metadata  map[string]any         `json:"metadata"`  // Additional metadata about the query
	CreatedAt time.Time              `json:"created_at"` // Creation timestamp
}

// NewQuery creates a new query entity.
func NewQuery(id, text string, metadata map[string]any) *Query {
	return &Query{
		ID:        id,
		Text:      text,
		Metadata:  metadata,
		CreatedAt: time.Now(),
	}
}
