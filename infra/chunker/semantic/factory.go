package semantic

import (
	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
)

// DefaultSemanticChunker 创建默认的语义分块器
// embedder: embedding provider
func DefaultSemanticChunker(embedder embedding.Provider) dataprep.SemanticChunker {
	return NewSemanticChunker(embedder, 100, 1000, 0.85)
}
