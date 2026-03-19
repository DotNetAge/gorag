package enhancement

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/core"
)

var _ core.FilterExtractor = (*FilterExtractor)(nil)

// FilterExtractor uses an LLM to parse natural language constraints into filters.
type FilterExtractor struct {
	llm chat.Client
}

// NewFilterExtractor creates a new filter extractor.
func NewFilterExtractor(llm chat.Client) *FilterExtractor {
	return &FilterExtractor{llm: llm}
}

// Extract extracts key-value filters from the user's query.
func (f *FilterExtractor) Extract(ctx context.Context, query *core.Query) (map[string]any, error) {
	prompt := fmt.Sprintf(`You are a metadata extraction tool.
Extract explicit filtering conditions from the user's query (e.g., year, author, document type, company name).
Return ONLY a valid JSON object containing the key-value pairs. 
If no explicit filters are mentioned, return an empty JSON object {}.

Query: "%s"`, query.Text)

	messages := []chat.Message{chat.NewUserMessage(prompt)}
	response, err := f.llm.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	cleanJSON := strings.TrimPrefix(strings.TrimSpace(response.Content), "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	var filters map[string]any
	if err := json.Unmarshal([]byte(cleanJSON), &filters); err != nil {
		return make(map[string]any), nil
	}

	return filters, nil
}
