package core

import (
	"time"
)

// Document represents a document entity in the RAG system.
type Document struct {
	ID          string         `json:"id"`           // Unique identifier for the document
	Content     string         `json:"content"`      // The actual content of the document
	Metadata    map[string]any `json:"metadata"`     // Additional metadata about the document
	CreatedAt   time.Time      `json:"created_at"`   // Creation timestamp
	UpdatedAt   time.Time      `json:"updated_at"`   // Last update timestamp
	Source      string         `json:"source"`       // Source of the document
	ContentType string         `json:"content_type"` // Type of content (e.g., text, pdf, markdown)
}

// NewDocument creates a new Document instance with the specified parameters.
//
// Parameters:
//   - id: unique identifier for the document
//   - content: text content of the document
//   - source: source location or origin of the document (file path, URL, etc.)
//   - contentType: MIME type or format indicator (e.g., "text/plain", "application/pdf")
//   - metadata: additional metadata (can be nil)
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
// It also refreshes the UpdatedAt timestamp to track modification time.
func (d *Document) Update(content string, metadata map[string]any) {
	d.Content = content
	d.Metadata = metadata
	d.UpdatedAt = time.Now()
}
