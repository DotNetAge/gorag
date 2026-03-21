package hyde

import (
	"context"
	"fmt"

	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

type hydeStep struct {
	generator core.Generator
	logger    logging.Logger
}

// Generate 创建一个 HyDE 生成步骤
func Generate(generator core.Generator, logger logging.Logger) pipeline.Step[*core.RetrievalContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &hydeStep{
		generator: generator,
		logger:    logger,
	}
}

func (s *hydeStep) Name() string {
	return "HyDE-Generate"
}

func (s *hydeStep) Execute(ctx context.Context, context *core.RetrievalContext) error {
	if context.Query == nil || context.Query.Text == "" {
		return fmt.Errorf("HyDE: query required in context")
	}

	s.logger.Debug("generating hypothetical document", map[string]any{
		"query": context.Query.Text,
	})

	doc, err := s.generator.GenerateHypotheticalDocument(ctx, context.Query)
	if err != nil {
		return fmt.Errorf("HyDE: failed to generate doc: %w", err)
	}

	if context.Agentic == nil {
		context.Agentic = &core.AgenticContext{}
	}
	context.Agentic.HypotheticalDocument = doc
	context.Agentic.HydeApplied = true

	s.logger.Info("hypothetical document generated", map[string]any{
		"doc_length": len(doc),
	})

	return nil
}
