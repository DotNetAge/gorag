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
func Generate(
	generator core.Generator,
	logger logging.Logger,
	metrics core.Metrics,
) pipeline.Step[*core.RetrievalContext] {
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
func (s *generate) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if context.Query == nil || context.Query.Text == "" {
		return fmt.Errorf("Generator: query required")
	}

	s.logger.Debug("generating response", map[string]interface{}{
		"step":  "Generator",
		"query": context.Query.Text,
	})

	// Flatten RetrievedChunks to []*core.Chunk
	var chunks []*core.Chunk
	for _, chunkGroup := range context.RetrievedChunks {
		chunks = append(chunks, chunkGroup...)
	}

	// Delegate to Generator interface
	result, err := s.generator.Generate(ctx, context.Query, chunks)
	if err != nil {
		s.logger.Error("generate failed", err, map[string]interface{}{
			"step":  "Generator",
			"query": context.Query.Text,
		})
		return fmt.Errorf("Generator: Generate failed: %w", err)
	}

	// Update context
	context.Answer = &core.Result{Answer: result.Answer}

	return nil
}
