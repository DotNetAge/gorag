package entity

import (
	"time"
)

// Chunk represents a document chunk entity in the RAG system.
// It is a portion of a document that has been processed for vectorization.
type Chunk struct {
	ID          string                 `json:"id"`
	DocumentID  string                 `json:"document_id"`  // ID of the root document
	ParentID    string                 `json:"parent_id,omitempty"` // For Parent-Child Indexing (Hierarchical)
	Level       int                    `json:"level"`        // 0: Root, 1: Parent, 2: Child
	Content     string                 `json:"content"`
	Metadata    map[string]any         `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	StartIndex  int                    `json:"start_index"`
	EndIndex    int                    `json:"end_index"`
	VectorID    string                 `json:"vector_id,omitempty"`
}

func NewChunk(id, documentID, content string, startIndex, endIndex int, metadata map[string]any) *Chunk {
	return &Chunk{
		ID:          id,
		DocumentID:  documentID,
		Content:     content,
		Metadata:    metadata,
		CreatedAt:   time.Now(),
		StartIndex:  startIndex,
		EndIndex:    endIndex,
	}
}

func (c *Chunk) SetVectorID(vectorID string) {
	c.VectorID = vectorID
}
