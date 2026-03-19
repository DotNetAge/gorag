// Package answer provides answer generation utilities for RAG systems.
package answer

import (
	"github.com/DotNetAge/gorag/pkg/core"
	"context"
	"fmt"
	"strings"
	"time"
	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

const defaultGenerationPrompt = `You are a helpful and professional AI assistant.
Please answer the user's question based on the provided reference documents.
If the documents do not contain the answer, say "I don't know based on the provided context."

[Reference Documents]
%s

[User Question]
%s

Answer:`

// generator implements Generator interface.
type Generator struct {
	llm            chat.Client
	promptTemplate string
	logger         logging.Logger
	collector      observability.Collector
}

// Option configures a Generator instance.
type Option func(*Generator)

// WithPromptTemplate overrides the default generation prompt.
func WithPromptTemplate(tmpl string) Option {
	return func(g *Generator) {
		if tmpl != "" {
			g.promptTemplate = tmpl
		}
	}
}

// WithGeneratorLogger sets a structured logger.
func WithLogger(logger logging.Logger) Option {
	return func(g *Generator) {
		if logger != nil {
			g.logger = logger
		}
	}
}

// WithGeneratorCollector sets an observability collector.
func WithCollector(collector observability.Collector) Option {
	return func(g *Generator) {
		if collector != nil {
			g.collector = collector
		}
	}
}

// New creates a new generator.
//
// Required: llm.
// Optional (via options): WithPromptTemplate, WithLogger, WithCollector.
func New(llm chat.Client, opts ...Option) *Generator {
	g := &Generator{
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
func (g *Generator) Generate(ctx context.Context, query *core.Query, chunks []*core.Chunk) (*core.Result, error) {
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
	messages := []chat.Message{
		chat.NewUserMessage(prompt),
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

	return &core.Result{
		Answer: response.Content,
	}, nil
}
