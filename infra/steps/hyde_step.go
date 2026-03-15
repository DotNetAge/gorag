package steps

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/infra/enhancer"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*HyDEStep)(nil)

// HyDEStep is a pipeline step that generates a hypothetical document
// to improve dense retrieval by bridging the query-document semantic gap.
type HyDEStep struct {
	generator *enhancer.HyDEGenerator
}

// NewHyDEStep creates a new HyDE step.
//
// Parameters:
// - generator: The HyDE generator instance
//
// Returns:
// - A new HyDEStep instance
func NewHyDEStep(generator *enhancer.HyDEGenerator) *HyDEStep {
	return &HyDEStep{generator: generator}
}

// Name returns the step name
func (s *HyDEStep) Name() string {
	return "HyDEStep"
}

// Execute generates a hypothetical document based on the query.
// The hypothetical document is stored in state for later embedding and retrieval.
func (s *HyDEStep) Execute(ctx context.Context, state *entity.PipelineState) error {
	if state.Query == nil {
		return fmt.Errorf("HyDEStep: 'query' not found in state")
	}

	// Generate hypothetical document
	hypotheticalDoc, err := s.generator.GenerateHypotheticalDocument(ctx, state.Query)
	if err != nil {
		return fmt.Errorf("HyDEStep failed to generate hypothetical document: %w", err)
	}

	// Store the hypothetical document for embedding
	state.GenerationPrompt = hypotheticalDoc.Content

	// Mark that HyDE was applied
	state.Query.Metadata["hyde_applied"] = true

	fmt.Printf("HyDEStep: generated hypothetical document (%d chars)\n", len(hypotheticalDoc.Content))
	return nil
}
