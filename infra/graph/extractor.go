package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/domain/entity"
	"github.com/DotNetAge/gorag/pkg/usecase/dataprep"
)

// SimpleLLMClient represents the required LLM functions.
type SimpleLLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

var _ dataprep.GraphExtractor = (*GraphExtractorImpl)(nil)

// GraphExtractorImpl uses an LLM to extract Entities (Nodes) and Relationships (Edges) from text chunks.
type GraphExtractorImpl struct {
	llm SimpleLLMClient
}

func NewGraphExtractor(llm SimpleLLMClient) *GraphExtractorImpl {
	return &GraphExtractorImpl{llm: llm}
}

type extractResult struct {
	Nodes []abstraction.Node `json:"nodes"`
	Edges []abstraction.Edge `json:"edges"`
}

func (e *GraphExtractorImpl) Extract(ctx context.Context, chunk *entity.Chunk) ([]abstraction.Node, []abstraction.Edge, error) {
	prompt := fmt.Sprintf(`You are a top-tier knowledge graph extraction algorithm.
Your task is to extract entities and their relationships from the given text.
Return ONLY a valid JSON object with "nodes" and "edges" arrays.
Nodes must have "id" (entity name) and "type" (e.g., PERSON, ORGANIZATION, CONCEPT).
Edges must have "source" (node id), "target" (node id), and "type" (relationship description).

[Text]
%s`, chunk.Content)

	response, err := e.llm.Generate(ctx, prompt)
	if err != nil {
		return nil, nil, err
	}

	// Clean up potential markdown formatting from LLM output
	cleanJSON := strings.TrimPrefix(strings.TrimSpace(response), "```json")
	cleanJSON = strings.TrimPrefix(cleanJSON, "```")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")

	var result extractResult
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse extracted graph JSON: %w\nRaw Output: %s", err, response)
	}

	// Attach chunk tracking metadata to the nodes/edges for traceability
	for i := range result.Nodes {
		if result.Nodes[i].Properties == nil {
			result.Nodes[i].Properties = make(map[string]any)
		}
		result.Nodes[i].Properties["source_chunk_id"] = chunk.ID
	}
	for i := range result.Edges {
		if result.Edges[i].Properties == nil {
			result.Edges[i].Properties = make(map[string]any)
		}
		result.Edges[i].Properties["source_chunk_id"] = chunk.ID
	}

	return result.Nodes, result.Edges, nil
}
