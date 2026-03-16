package retrieval

import "github.com/DotNetAge/gorag/pkg/domain/entity"

// AgenticMetadata is an alias for entity.AgenticMetadata.
// All methods (Validate, MergeToQuery, LoadFromQuery, SetCacheHit, GetCacheHit) are
// defined on entity.AgenticMetadata. Use entity.AgenticMetadata directly.
type AgenticMetadata = entity.AgenticMetadata

// NewAgenticMetadata creates a new AgenticMetadata instance.
func NewAgenticMetadata() *AgenticMetadata {
	return entity.NewAgenticMetadata()
}
