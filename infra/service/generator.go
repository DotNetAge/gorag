package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// ensure interface implementation
var _ retrieval.Generator = (*generator)(nil)

const defaultGenerationPrompt = `You are a helpful and professional AI assistant.
Please answer the user's question based on the provided reference documents.
If the documents do not contain the answer, say "I don't know based on the provided context."

[Reference Documents]
%s

[User Question]
%s

Answer:`

// generator is the infrastructure implementation of retrieval.Generator.
type generator struct {
	llm            core.Client
	promptTemplate string
	logger         logging.Logger
	collector      observability.Collector
}

// GeneratorOption configures a generator instance.
type GeneratorOption func(*generator)

// WithPromptTemplate overrides the default generation prompt.
func WithPromptTemplate(tmpl string) GeneratorOption {
	return func(g *generator) {
		if tmpl != "" {
			g.promptTemplate = tmpl
		}
	}
}

// WithGeneratorLogger sets a structured logger.
func WithGeneratorLogger(logger logging.Logger) GeneratorOption {
	return func(g *generator) {
		if logger != nil {
			g.logger = logger
		}
	}
}

// WithGeneratorCollector sets an observability collector.
func WithGeneratorCollector(collector observability.Collector) GeneratorOption {
	return func(g *generator) {
		if collector != nil {
			g.collector = collector
		}
	}
}

// NewGenerator creates a new generator.
//
// Required: llm.
// Optional (via options): WithPromptTemplate, WithGeneratorLogger, WithGeneratorCollector.
func NewGenerator(llm core.Client, opts ...GeneratorOption) *generator {
	g := &generator{
		llm:            llm,
		promptTemplate: defaultGenerationPrompt,
		logger:         logging.NewNoopLogger(),
		collector:      observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate generates an answer based on query and retrieved context.
func (g *generator) Generate(ctx context.Context, query *entity.Query, chunks []*entity.Chunk) (*retrieval.GenerationResult, error) {
	start := time.Now()
	defer func() {
		g.collector.RecordDuration("generation", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		g.logger.Error("generate failed", fmt.Errorf("query required"), map[string]interface{}{
			"operation": "generation",
		})
		g.collector.RecordCount("generation", "error", nil)
		return nil, fmt.Errorf("generator: query required")
	}

	g.logger.Debug("generating response", map[string]interface{}{
		"query":        query.Text,
		"chunks_count": len(chunks),
	})

	// Build context from chunks
	var contextBuilder strings.Builder
	for i, chunk := range chunks {
		if chunk.Content != "" {
			contextBuilder.WriteString(fmt.Sprintf("--- Document %d --\n%s\n\n", i+1, chunk.Content))
		}
	}
	contextStr := contextBuilder.String()

	// Build prompt
	prompt := fmt.Sprintf(g.promptTemplate, contextStr, query.Text)

	// Call LLM
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}
	response, err := g.llm.Chat(ctx, messages)
	if err != nil {
		g.logger.Error("LLM chat failed", err, map[string]interface{}{
			"operation": "generation",
			"query":     query.Text,
		})
		g.collector.RecordCount("generation", "error", nil)
		return nil, fmt.Errorf("generator: Chat failed: %w", err)
	}

	g.logger.Info("response generated", map[string]interface{}{
		"answer_length": len(response.Content),
		"query":         query.Text,
	})
	g.collector.RecordCount("generation", "success", nil)

	return &retrieval.GenerationResult{
		Answer: response.Content,
	}, nil
}
