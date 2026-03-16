package enhancer

import (
	"context"
	"testing"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// mockDocumentStore is a mock implementation of DocumentStore
type mockDocumentStore struct {
	docs map[string]*entity.Document
}

func newMockDocumentStore() *mockDocumentStore {
	return &mockDocumentStore{
		docs: make(map[string]*entity.Document),
	}
}

func (s *mockDocumentStore) GetByID(ctx context.Context, id string) (*entity.Document, error) {
	doc, ok := s.docs[id]
	if !ok {
		return nil, nil // Return nil, not error for not found
	}
	return doc, nil
}

func TestParentDocExpander_Enhance(t *testing.T) {
	tests := []struct {
		name          string
		chunks        []*entity.Chunk
		parentDocs    map[string]*entity.Document
		expectedCount int
		expectExpand  bool
	}{
		{
			name: "successful expansion to parent documents",
			chunks: []*entity.Chunk{
				{
					ID:         "c1",
					DocumentID: "doc1",
					ParentID:   "doc1",
					Content:    "function snippet 1",
					Metadata:   map[string]any{"type": "function"},
				},
				{
					ID:         "c2",
					DocumentID: "doc1",
					ParentID:   "doc1",
					Content:    "function snippet 2",
					Metadata:   map[string]any{"type": "function"},
				},
			},
			parentDocs: map[string]*entity.Document{
				"doc1": {
					ID:      "doc1",
					Content: "Complete source file content with all functions",
					Metadata: map[string]any{
						"type": "source_file",
						"path": "/path/to/file.go",
					},
				},
			},
			expectedCount: 1, // Should deduplicate to 1 parent
			expectExpand:  true,
		},
		{
			name: "no parent ID - return original",
			chunks: []*entity.Chunk{
				{
					ID:         "c1",
					DocumentID: "doc1",
					ParentID:   "", // No parent
					Content:    "Already at root level",
				},
			},
			parentDocs:    map[string]*entity.Document{},
			expectedCount: 1,
			expectExpand:  false,
		},
		{
			name: "parent not found - use original chunk",
			chunks: []*entity.Chunk{
				{
					ID:         "c1",
					DocumentID: "doc1",
					ParentID:   "nonexistent",
					Content:    "Original chunk content",
				},
			},
			parentDocs:    map[string]*entity.Document{},
			expectedCount: 1,
			expectExpand:  false,
		},
		{
			name: "duplicate parent deduplication",
			chunks: []*entity.Chunk{
				{ID: "c1", ParentID: "doc1", Content: "snippet 1"},
				{ID: "c2", ParentID: "doc1", Content: "snippet 2"},
				{ID: "c3", ParentID: "doc1", Content: "snippet 3"},
			},
			parentDocs: map[string]*entity.Document{
				"doc1": {ID: "doc1", Content: "Full document"},
			},
			expectedCount: 1, // All expand to same parent, deduplicated
			expectExpand:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			store := newMockDocumentStore()
			for id, doc := range tt.parentDocs {
				store.docs[id] = doc
			}

			expander := NewParentDocExpander(store)

			results := entity.NewRetrievalResult(
				uuid.New().String(),
				uuid.New().String(),
				tt.chunks,
				make([]float32, len(tt.chunks)),
				nil,
			)

			// Execute
			ctx := context.Background()
			expandedResults, err := expander.Enhance(ctx, results)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, expandedResults)
			assert.Len(t, expandedResults.Chunks, tt.expectedCount)

			if tt.expectExpand {
				// Verify chunks are expanded to parent content
				for _, chunk := range expandedResults.Chunks {
					assert.Equal(t, tt.parentDocs["doc1"].Content, chunk.Content)
					assert.Empty(t, chunk.ParentID) // Should be at root level now
				}
			}
		})
	}
}

func TestMergeMetadata(t *testing.T) {
	tests := []struct {
		name     string
		child    map[string]any
		parent   map[string]any
		expected map[string]any
	}{
		{
			name:   "child overrides parent",
			child:  map[string]any{"key1": "child_value", "key2": "child_only"},
			parent: map[string]any{"key1": "parent_value", "key3": "parent_only"},
			expected: map[string]any{
				"key1": "child_value", // Child wins
				"key2": "child_only",
				"key3": "parent_only",
			},
		},
		{
			name:     "empty child",
			child:    map[string]any{},
			parent:   map[string]any{"key": "value"},
			expected: map[string]any{"key": "value"},
		},
		{
			name:     "empty parent",
			child:    map[string]any{"key": "value"},
			parent:   map[string]any{},
			expected: map[string]any{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeMetadata(tt.child, tt.parent)
			assert.Equal(t, tt.expected, result)
		})
	}
}
