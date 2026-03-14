package dataprep

import (
	"context"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// GraphExtractor is responsible for LLM-based Entity and Relationship extraction.
type GraphExtractor interface {
	// Extract parses a chunk and returns a list of Nodes (Entities) and Edges (Relationships).
	// This forms the foundation for GraphRAG indexing.
	Extract(ctx context.Context, chunk *entity.Chunk) ([]abstraction.Node, []abstraction.Edge, error)
}
