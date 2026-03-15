package service

import (
	"context"
	"encoding/json"
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
var _ retrieval.EntityExtractor = (*entityExtractor)(nil)

// entityExtractorConfig holds configuration for entity extraction.
type entityExtractorConfig struct {
	PromptTemplate string
}

// DefaultEntityExtractorConfig returns a default configuration.
func DefaultEntityExtractorConfig() entityExtractorConfig {
	return entityExtractorConfig{
		PromptTemplate: defaultEntityExtractionPrompt,
	}
}

const defaultEntityExtractionPrompt = `You are an expert entity extraction system.
Your task is to extract key entities (people, places, organizations, concepts, etc.) from the user's question.
Return ONLY a JSON object with an "entities" array containing the entity names as strings.

Example Input: "What is the relationship between Microsoft and Bill Gates?"
Example Output: {"entities": ["Microsoft", "Bill Gates"]}

[Question]
%s

[Entities]`

// entityExtractor is the infrastructure implementation of retrieval.EntityExtractor.
type entityExtractor struct {
	llm       core.Client
	config    entityExtractorConfig
	logger    logging.Logger
	collector observability.Collector
}

// NewEntityExtractor creates a new entity extractor with logger and metrics.
func NewEntityExtractor(llm core.Client, config entityExtractorConfig, logger logging.Logger, collector observability.Collector) *entityExtractor {
	if config.PromptTemplate == "" {
		config = DefaultEntityExtractorConfig()
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	if collector == nil {
		collector = observability.NewNoopCollector()
	}
	return &entityExtractor{
		llm:       llm,
		config:    config,
		logger:    logger,
		collector: collector,
	}
}

// Extract extracts entities from the query.
func (e *entityExtractor) Extract(ctx context.Context, query *entity.Query) (*retrieval.EntityExtractionResult, error) {
	start := time.Now()
	defer func() {
		e.collector.RecordDuration("entity_extraction", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		e.logger.Error("extract failed", fmt.Errorf("query required"), map[string]interface{}{
			"operation": "entity_extraction",
		})
		e.collector.RecordCount("entity_extraction", "error", nil)
		return nil, fmt.Errorf("entityExtractor: query required")
	}

	e.logger.Debug("extracting entities", map[string]interface{}{
		"query": query.Text,
	})

	// Build prompt
	prompt := fmt.Sprintf(e.config.PromptTemplate, query.Text)

	// Call LLM
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}
	response, err := e.llm.Chat(ctx, messages)
	if err != nil {
		e.logger.Error("LLM chat failed", err, map[string]interface{}{
			"operation": "entity_extraction",
			"query":     query.Text,
		})
		e.collector.RecordCount("entity_extraction", "error", nil)
		return nil, fmt.Errorf("entityExtractor: Chat failed: %w", err)
	}

	// Parse JSON response
	var result retrieval.EntityExtractionResult
	content := strings.TrimSpace(response.Content)

	// Extract JSON from response
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		// If parsing fails, return empty result
		e.logger.Warn("failed to parse entities", map[string]interface{}{
			"error": err,
		})
		return &retrieval.EntityExtractionResult{}, nil
	}

	e.logger.Info("entities extracted successfully", map[string]interface{}{
		"entities_count": len(result.Entities),
		"query":          query.Text,
	})
	e.collector.RecordCount("entity_extraction", "success", nil)

	return &result, nil
}
