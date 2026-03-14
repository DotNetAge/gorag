// Package graph provides graph-related utilities for RAG systems.
// It includes components for extracting entities and relationships from text,
// as well as searching knowledge graphs for relevant information.
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

// SimpleLLMClient represents the required LLM functions for graph extraction.
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

var _ dataprep.GraphExtractor = (*GraphExtractor)(nil)

// GraphExtractor uses an LLM to extract Entities (Nodes) and Relationships (Edges) from text chunks.
// It helps build knowledge graphs from unstructured text for better retrieval and reasoning.
type GraphExtractor struct {
	// llm is the LLM client used for extracting graph elements
	llm SimpleLLMClient
}

// NewGraphExtractor creates a new graph extractor.
//
// Parameters:
// - llm: The LLM client to use for extraction
//
// Returns:
// - A new GraphExtractor instance
func NewGraphExtractor(llm SimpleLLMClient) *GraphExtractor {
	return &GraphExtractor{llm: llm}
}

// extractResult represents the result of graph extraction.
type extractResult struct {
	// Nodes are the entities extracted from the text
	Nodes []abstraction.Node `json:"nodes"`
	// Edges are the relationships between entities
	Edges []abstraction.Edge `json:"edges"`
}

// Extract extracts entities and relationships from a text chunk.
//
// Parameters:
// - ctx: The context for the operation
// - chunk: The text chunk to extract from
//
// Returns:
// - A slice of nodes (entities)
// - A slice of edges (relationships)
// - An error if extraction fails
func (e *GraphExtractor) Extract(ctx context.Context, chunk *entity.Chunk) ([]abstraction.Node, []abstraction.Edge, error) {
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
