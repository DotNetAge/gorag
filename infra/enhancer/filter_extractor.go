// Package enhancer provides query and document enhancement utilities for RAG systems.
// It includes components for query rewriting, hypothetical document generation,
// and step-back prompting to improve retrieval and generation quality.
package enhancer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

// SimpleLLMClient defines a simple LLM client interface for text generation
type SimpleLLMClient interface {
	// Generate generates text based on the given prompt
	//
	// Parameters:
	// - ctx: The context for the operation
	// - prompt: The prompt to generate text from
	//
	// Returns:
	// - The generated text
	// - An error if generation fails
	Generate(ctx context.Context, prompt string) (string, error)
}

var _ retrieval.FilterExtractor = (*FilterExtractor)(nil)

// FilterExtractor uses an LLM to parse natural language constraints into key-value filters
// for precise Vector Database pre-filtering. It helps improve retrieval precision by
// extracting explicit filtering conditions from user queries.
type FilterExtractor struct {
	// llm is the LLM client used for extracting filters
	llm SimpleLLMClient
}

// NewFilterExtractor creates a new filter extractor.
//
// Parameters:
// - llm: The LLM client to use for extraction
//
// Returns:
// - A new FilterExtractor instance
func NewFilterExtractor(llm SimpleLLMClient) *FilterExtractor {
	return &FilterExtractor{llm: llm}
}

// ExtractFilters extracts key-value filters from the user's query.
//
// Parameters:
// - ctx: The context for the operation
// - query: The query to extract filters from
//
// Returns:
// - A map of key-value filters
// - An error if extraction fails
func (f *FilterExtractor) ExtractFilters(ctx context.Context, query *entity.Query) (map[string]any, error) {
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
