package pre_retrieval

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ pipeline.Step[*entity.PipelineState] = (*HyDEStep)(nil)

// HyDEStep is a pipeline step that generates a hypothetical document
// to improve dense retrieval by bridging the query-document semantic gap.
type HyDEStep struct {
	generator retrieval.HyDEGenerator
	logger    logging.Logger
}

// NewHyDEStep creates a new HyDE step.
//
// Parameters:
// - generator: The HyDE generator instance (any retrieval.HyDEGenerator implementation)
// - logger: optional structured logger; pass nil to use noop
//
// Returns:
// - A new HyDEStep instance
func NewHyDEStep(generator retrieval.HyDEGenerator, logger logging.Logger) *HyDEStep {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &HyDEStep{
		generator: generator,
		logger:    logger,
	}
}

// Name returns the step name
func (s *HyDEStep) Name() string {
	return "HyDEStep"
}

// Execute generates a hypothetical document based on the query.
// The hypothetical document is stored in state.Agentic.HypotheticalDocument.
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

	// Mark that HyDE was applied via AgenticMetadata (not blackboard)
	if state.Agentic == nil {
		state.Agentic = entity.NewAgenticMetadata()
	}
	state.Agentic.HydeApplied = true
	state.Agentic.HypotheticalDocument = hypotheticalDoc.Content

	s.logger.Info("HyDEStep completed", map[string]interface{}{
		"step":       "HyDEStep",
		"doc_length": len(hypotheticalDoc.Content),
	})
	return nil
}
