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
var _ retrieval.IntentClassifier = (*intentRouter)(nil)

// intentRouterConfig holds configuration for intent classification.
type intentRouterConfig struct {
	PromptTemplate string
	DefaultIntent  retrieval.IntentType
	MinConfidence  float32
}

// DefaultIntentRouterConfig returns a default configuration.
func DefaultIntentRouterConfig() intentRouterConfig {
	return intentRouterConfig{
		PromptTemplate: defaultIntentPrompt,
		DefaultIntent:  retrieval.IntentDomainSpecific,
		MinConfidence:  0.7,
	}
}

const defaultIntentPrompt = `You are an expert intent classifier for a RAG system.
Your task is to classify the user's query into one of the following intents:

1. **chat**: Casual conversation, greetings, simple factual questions that LLM can answer directly
   - Examples: "Hello", "How are you?", "What is 2+2?", "Tell me a joke"

2. **domain_specific**: Complex questions requiring domain-specific knowledge retrieval
   - Examples: "What is our company's refund policy?", "Explain the architecture of goRAG", "How does the semantic cache work?"

3. **fact_check**: Questions about recent events, current facts, or external verification
   - Examples: "What's the weather today?", "Who won the game last night?", "What's the latest news about AI?"

Analyze the query carefully and consider:
- Does it require accessing our knowledge base? → domain_specific
- Can it be answered with general knowledge? → chat
- Does it need real-time or external information? → fact_check

[Query]
%s

Output your response as a valid JSON object with this exact structure:
{
  "intent": "chat|domain_specific|fact_check",
  "confidence": 0.0-1.0,
  "reason": "brief explanation of your reasoning"
}`

// intentRouter is the infrastructure implementation of retrieval.IntentClassifier.
// This contains the thick business logic for intent classification.
type intentRouter struct {
	llm       core.Client
	config    intentRouterConfig
	logger    logging.Logger
	collector observability.Collector
}

// NewIntentRouter creates a new intent router with logger and metrics.
func NewIntentRouter(llm core.Client, config intentRouterConfig, logger logging.Logger, collector observability.Collector) *intentRouter {
	if config.PromptTemplate == "" {
		config = DefaultIntentRouterConfig()
	}
	if logger == nil {
		logger = logging.NewNoopLogger()
	}
	if collector == nil {
		collector = observability.NewNoopCollector()
	}
	return &intentRouter{
		llm:       llm,
		config:    config,
		logger:    logger,
		collector: collector,
	}
}

// Classify performs intent classification on the query.
// This is the thick business logic method that was previously in the Step.
func (i *intentRouter) Classify(ctx context.Context, query *entity.Query) (*retrieval.IntentResult, error) {
	start := time.Now()
	defer func() {
		i.collector.RecordDuration("intent_classification", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		i.logger.Error("classify failed", fmt.Errorf("query is nil or empty"), map[string]interface{}{
			"operation": "intent_classification",
		})
		i.collector.RecordCount("intent_classification", "error", nil)
		return nil, fmt.Errorf("query is nil or empty")
	}

	i.logger.Debug("classifying intent", map[string]interface{}{
		"query": query.Text,
	})

	// Build prompt
	prompt := fmt.Sprintf(i.config.PromptTemplate, query.Text)

	// Call LLM
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}

	response, err := i.llm.Chat(ctx, messages)
	if err != nil {
		i.logger.Error("LLM chat failed", err, map[string]interface{}{
			"operation": "intent_classification",
			"query":     query.Text,
		})
		i.collector.RecordCount("intent_classification", "error", nil)
		return nil, fmt.Errorf("IntentClassifierImpl.Classify failed to call LLM: %w", err)
	}

	// Parse JSON response
	var result retrieval.IntentResult
	content := strings.TrimSpace(response.Content)

	// Extract JSON
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		// Fallback to default intent
		i.logger.Warn("failed to parse intent, using default", map[string]interface{}{
			"error":          err,
			"default_intent": i.config.DefaultIntent,
		})
		result = retrieval.IntentResult{
			Intent:     i.config.DefaultIntent,
			Confidence: 0.5,
			Reason:     "Failed to parse LLM response, using default intent",
		}
	}

	// Validate confidence threshold
	if result.Confidence < i.config.MinConfidence {
		i.logger.Debug("low confidence, using default intent", map[string]interface{}{
			"confidence":     result.Confidence,
			"threshold":      i.config.MinConfidence,
			"default_intent": i.config.DefaultIntent,
		})
		result.Intent = i.config.DefaultIntent
		result.Reason += fmt.Sprintf(" [Low confidence %.2f < %.2f, using default]",
			result.Confidence, i.config.MinConfidence)
	}

	i.logger.Info("intent classified successfully", map[string]interface{}{
		"intent":     result.Intent,
		"confidence": result.Confidence,
		"query":      query.Text,
	})
	i.collector.RecordCount("intent_classification", "success", nil)

	return &result, nil
}
