package store

import (
	"context"
)

// IndexStruct defines an index topology.
type IndexStruct struct {
	IndexID string         `json:"index_id"`
	Type    string         `json:"type"` // e.g., "vector", "tree", "keyword", "graph"
	Nodes   []string       `json:"nodes"`
	Summary string         `json:"summary,omitempty"`
	Config  map[string]any `json:"config,omitempty"`
}

// IndexStore tracks what indices have been built, managing state for incremental updates and routing.
// LlamaIndex uses IndexStore to compose a multi-index architecture.
type IndexStore interface {
	// AddIndexStruct saves the structure of a specific index
	AddIndexStruct(ctx context.Context, idxStruct *IndexStruct) error
	// GetIndexStruct retrieves a specific index layout
	GetIndexStruct(ctx context.Context, structID string) (*IndexStruct, error)
	// DeleteIndexStruct removes index tracking metadata
	DeleteIndexStruct(ctx context.Context, structID string) error
	// IndexStructs retrieves all registered indices
	IndexStructs(ctx context.Context) ([]*IndexStruct, error)
}
