package enhancer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

var _ retrieval.FilterExtractor = (*FilterExtractorImpl)(nil)

// FilterExtractorImpl uses an LLM to parse natural language constraints into key-value filters
// for precise Vector Database pre-filtering.
type FilterExtractorImpl struct {
	llm SimpleLLMClient
}

func NewFilterExtractor(llm SimpleLLMClient) *FilterExtractorImpl {
	return &FilterExtractorImpl{llm: llm}
}

func (f *FilterExtractorImpl) ExtractFilters(ctx context.Context, query *entity.Query) (map[string]any, error) {
	prompt := fmt.Sprintf(`You are a metadata extraction tool.
Extract explicit filtering conditions from the user's query (e.g., year, author, document type, company name).
Return ONLY a valid JSON object containing the key-value pairs. 
If no explicit filters are mentioned, return an empty JSON object {}.

Query: "%s"`, query.Text)

	response, err := f.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	cleanJSON := strings.TrimPrefix(strings.TrimSpace(response), "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	var filters map[string]any
	if err := json.Unmarshal([]byte(cleanJSON), &filters); err != nil {
		// Fallback to empty filter instead of breaking the pipeline
		return make(map[string]any), nil 
	}

	return filters, nil
}
