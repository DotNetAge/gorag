// Package generate provides answer generation steps for RAG pipelines.
package stepgen

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// generate generates final answer based on retrieved chunks and query.
type generate struct {
	generator core.Generator
	logger    logging.Logger
	metrics   core.Metrics
}

// Generate creates a new generation step with logger and metrics.
//
// Parameters:
//   - generator: LLM answer generator implementation
//   - logger: structured logger (auto-defaults to NoopLogger if nil)
//   - metrics: metrics collector (optional, can be nil)
//
// Example:
//
//	p.AddStep(generate.Generate(generator, logger, metrics))
func Generate(
	generator core.Generator,
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
	return "Generator"
}

// Execute generates answer by delegating to the Generator interface.
func (s *generate) Execute(ctx context.Context, state *core.State) error {
	if state.Query == nil || state.Query.Text == "" {
		return fmt.Errorf("Generator: query required")
	}

	s.logger.Debug("generating response", map[string]interface{}{
		"step":  "Generator",
		"query": state.Query.Text,
	})

	// Flatten RetrievedChunks to []*core.Chunk
	var chunks []*core.Chunk
	for _, chunkGroup := range state.RetrievedChunks {
		chunks = append(chunks, chunkGroup...)
	}

	// Delegate to Generator interface (business logic abstraction)
	result, err := s.generator.Generate(ctx, state.Query, chunks)
	if err != nil {
		s.logger.Error("generate failed", err, map[string]interface{}{
			"step":  "Generator",
			"query": state.Query.Text,
		})
		return fmt.Errorf("Generator: Generate failed: %w", err)
	}

	// Update state
	state.Answer = &core.Result{Answer: result.Answer}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSearchResult("generation", 1)
	}

	s.logger.Info("response generated", map[string]interface{}{
		"step":          "Generator",
		"answer_length": len(result.Answer),
		"query":         state.Query.Text,
		"chunk_count":   len(chunks),
	})

	return nil
}
