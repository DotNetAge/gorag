package evaluation

import (
	"github.com/DotNetAge/gorag/pkg/core"
)

// Re-export common types if needed, but prefer direct core usage
type Label = core.CRAGLabel
const (
	Relevant   = core.CRAGRelevant
	Irrelevant = core.CRAGIrrelevant
	Ambiguous  = core.CRAGAmbiguous
)
