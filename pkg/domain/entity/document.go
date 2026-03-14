// Package entity defines the core entities for the goRAG framework.
package entity

import (
	"time"
)

// Document represents a document entity in the RAG system.
// It contains the original content, metadata, and source information.
//
// Related RAG concepts:
// - Document Processing: Part of the data pipeline for converting documents into vector representations
// - Data Freshness & Lifecycle: Tracks creation and update times for data lifecycle management
type Document struct {
	ID          string                 `json:"id"`          // Unique identifier for the document
	Content     string                 `json:"content"`     // The actual content of the document
	Metadata    map[string]any         `json:"metadata"`    // Additional metadata about the document
	CreatedAt   time.Time              `json:"created_at"`  // Creation timestamp
	UpdatedAt   time.Time              `json:"updated_at"`  // Last update timestamp
	Source      string                 `json:"source"`      // Source of the document
	ContentType string                 `json:"content_type"` // Type of content (e.g., text, pdf, markdown)
}

// NewDocument creates a new document entity.
func NewDocument(id, content, source, contentType string, metadata map[string]any) *Document {
	now := time.Now()
	return &Document{
		ID:          id,
		Content:     content,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
		Source:      source,
		ContentType: contentType,
	}
}

// Update updates the document content and metadata.
func (d *Document) Update(content string, metadata map[string]any) {
	d.Content = content
	d.Metadata = metadata
	d.UpdatedAt = time.Now()
}
