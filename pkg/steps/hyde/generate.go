// Package hyde provides HyDE (Hypothetical Document Embeddings) steps for RAG retrieval pipelines.
package stephyde

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// generate generates a hypothetical document to improve dense core.
type generate struct {
	generator core.HyDEGenerator
	logger    logging.Logger
	metrics   core.Metrics
}

// Generate creates a new HyDE generation step with logger and metrics.
//
// Parameters:
//   - generator: HyDE generator implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(hyde.Generate(generator, logger, metrics))
func Generate(
	generator core.HyDEGenerator,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.State] {
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	return &generate{
		generator: generator,
		logger:    logger,
		metrics:   metrics,
	}
}

// Name returns the step name
func (s *generate) Name() string {
	return "HyDEGeneration"
}

// Execute generates a hypothetical document based on the query.
func (s *generate) Execute(ctx context.Context, state *core.State) error {
	if state.Query == nil {
		return fmt.Errorf("HyDEGeneration: 'query' not found in state")
	}

	// Generate hypothetical document
	hypotheticalDoc, err := s.generator.GenerateHypotheticalDocument(ctx, state.Query)
	if err != nil {
		s.logger.Error("hyde generation failed", err, map[string]interface{}{
			"step":  "HyDEGeneration",
			"query": state.Query.Text,
		})
		return fmt.Errorf("HyDEGeneration failed: %w", err)
	}

	// Store the hypothetical document for embedding
	state.GenerationPrompt = hypotheticalDoc

	// Mark that HyDE was applied via AgenticMetadata
	if state.Agentic == nil {
		state.Agentic = core.NewAgenticState()
	}
	state.Agentic.HydeApplied = true
	state.Agentic.HypotheticalDocument = hypotheticalDoc

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("hyde", 1)
	}

	s.logger.Info("HyDEGeneration completed", map[string]interface{}{
		"step":       "HyDEGeneration",
		"doc_length": len(hypotheticalDoc),
	})

	return nil
}
