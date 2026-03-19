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
var _ core.IntentClassifier = (*intentRouter)(nil)

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

// intentRouter is the implementation of core.IntentClassifier.
type intentRouter struct {
	llm            chat.Client
	promptTemplate string
	defaultIntent  core.IntentType
	minConfidence  float32
	logger         logging.Logger
	collector      observability.Collector
}

// IntentRouterOption configures an intentRouter instance.
type IntentRouterOption func(*intentRouter)

// WithIntentPromptTemplate overrides the default intent classification prompt.
func WithIntentPromptTemplate(tmpl string) IntentRouterOption {
	return func(r *intentRouter) {
		if tmpl != "" {
			r.promptTemplate = tmpl
		}
	}
}

// WithDefaultIntent sets the fallback intent when LLM confidence is low.
func WithDefaultIntent(intent core.IntentType) IntentRouterOption {
	return func(r *intentRouter) {
		r.defaultIntent = intent
	}
}

// WithMinConfidence sets the minimum confidence threshold.
func WithMinConfidence(v float32) IntentRouterOption {
	return func(r *intentRouter) {
		if v > 0 {
			r.minConfidence = v
		}
	}
}

// WithIntentRouterLogger sets a structured logger.
func WithIntentRouterLogger(logger logging.Logger) IntentRouterOption {
	return func(r *intentRouter) {
		if logger != nil {
			r.logger = logger
		}
	}
}

// WithIntentRouterCollector sets an observability collector.
func WithIntentRouterCollector(collector observability.Collector) IntentRouterOption {
	return func(r *intentRouter) {
		if collector != nil {
			r.collector = collector
		}
	}
}

// NewIntentRouter creates a new intent router.
func NewIntentRouter(llm chat.Client, opts ...IntentRouterOption) *intentRouter {
	r := &intentRouter{
		llm:            llm,
		promptTemplate: defaultIntentPrompt,
		defaultIntent:  core.IntentDomainSpecific,
		minConfidence:  0.7,
		logger:         logging.NewNoopLogger(),
		collector:      observability.NewNoopCollector(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Classify performs intent classification on the query.
func (i *intentRouter) Classify(ctx context.Context, query *core.Query) (*core.IntentResult, error) {
	start := time.Now()
	defer func() {
		i.collector.RecordDuration("intent_classification", time.Since(start), nil)
	}()

	if query == nil || query.Text == "" {
		return nil, fmt.Errorf("query is nil or empty")
	}

	prompt := fmt.Sprintf(i.promptTemplate, query.Text)
	messages := []chat.Message{chat.NewUserMessage(prompt)}

	response, err := i.llm.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("intent classification failed: %w", err)
	}

	var result core.IntentResult
	content := strings.TrimSpace(response.Content)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		content = content[jsonStart : jsonEnd+1]
	}

	err = json.Unmarshal([]byte(content), &result)
	if err != nil {
		result = core.IntentResult{
			Intent:     i.defaultIntent,
			Confidence: 0.5,
			Reason:     "Failed to parse LLM response, using default",
		}
	}

	if result.Confidence < i.minConfidence {
		result.Intent = i.defaultIntent
	}

	return &result, nil
}
