package query

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
	"github.com/DotNetAge/gorag/pkg/observability"
)

// ensure interface implementation
var _ core.EntityExtractor = (*entityExtractor)(nil)

const defaultEntityExtractionPrompt = `You are an expert entity extraction system.
Your task is to extract key entities (people, places, organizations, concepts, etc.) from the user's question.
Return ONLY a JSON object with an "entities" array containing the entity names as strings.

[Question]
%s

[Entities]`

// entityExtractor is the implementation of core.EntityExtractor.
type entityExtractor struct {
	llm            chat.Client
	promptTemplate string
	logger         logging.Logger
	collector      observability.Collector
}

// EntityExtractorOption configures an entityExtractor instance.
type EntityExtractorOption func(*entityExtractor)

// WithEntityExtractionPromptTemplate overrides the default extraction prompt.
func WithEntityExtractionPromptTemplate(tmpl string) EntityExtractorOption {
	return func(e *entityExtractor) {
		if tmpl != "" {
			e.promptTemplate = tmpl
		}
	}
}

// WithEntityExtractorLogger sets a structured logger.
func WithEntityExtractorLogger(logger logging.Logger) EntityExtractorOption {
	return func(e *entityExtractor) {
		if logger != nil {
			e.logger = logger
		}
	}
}

// WithEntityExtractorCollector sets an observability collector.
func WithEntityExtractorCollector(collector observability.Collector) EntityExtractorOption {
	return func(e *entityExtractor) {
		if collector != nil {
			e.collector = collector
		}
	}
}

// NewEntityExtractor creates a new entity extractor.
func NewEntityExtractor(llm chat.Client, opts ...EntityExtractorOption) *entityExtractor {
	e := &entityExtractor{
		llm:            llm,
		promptTemplate: defaultEntityExtractionPrompt,
		logger:         logging.NewNoopLogger(),
		collector:      observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Extract extracts entities from the query.
func (e *entityExtractor) Extract(ctx context.Context, query *core.Query) (*core.EntityExtractionResult, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("entity_extraction", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		return nil, fmt.Errorf("query required")
	}

	prompt := fmt.Sprintf(e.promptTemplate, query.Text)
	messages := []chat.Message{chat.NewUserMessage(prompt)}

	response, err := e.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("entity extraction failed: %w", err)
	}

	var result core.EntityExtractionResult
	content := strings.TrimSpace(response.Content)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		return &core.EntityExtractionResult{}, nil
	}

	return &result, nil
}
