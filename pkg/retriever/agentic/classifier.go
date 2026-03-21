package agentic

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/core"
	ragcore "github.com/DotNetAge/gorag/pkg/core"
)

type llmClassifier struct {
	client core.Client
}

func NewLLMClassifier(client core.Client) ragcore.IntentClassifier {
	return &llmClassifier{client: client}
}

func (c *llmClassifier) Classify(ctx context.Context, query *ragcore.Query) (*ragcore.IntentResult, error) {
	if c.client == nil {
		return &ragcore.IntentResult{Intent: ragcore.IntentChat, Confidence: 1.0}, nil
	}

	prompt := fmt.Sprintf(`Classify the following user query into one of these RAG intents:
- Chat: General conversation or greetings.
- FactCheck: Specific questions requiring factual accuracy and multi-step reasoning.
- Relational: Questions about relationships, hierarchies, or connections between entities.
- DomainSpecific: Highly technical or specialized domain questions.

Query: "%s"

Output ONLY the intent name.`, query.Text)

	resp, err := c.client.Chat(ctx, []core.Message{
		core.NewUserMessage(prompt),
	})
	if err != nil {
		return nil, err
	}

	intent := ragcore.IntentChat
	// In gochat v0.1.9, Chat returns *Response which has a Message field.
	raw := strings.ToLower(resp.Message.TextContent())

	if strings.Contains(raw, "factcheck") {
		intent = ragcore.IntentFactCheck
	} else if strings.Contains(raw, "relational") {
		intent = ragcore.IntentRelational
	} else if strings.Contains(raw, "domainspecific") {
		intent = ragcore.IntentDomainSpecific
	}

	return &ragcore.IntentResult{
		Intent:     intent,
		Confidence: 0.9,
		Reason:     "LLM classification",
	}, nil
}
