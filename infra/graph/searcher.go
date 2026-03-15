// Package graph provides graph-related utilities for RAG systems.
// It includes components for extracting entities and relationships from text,
// as well as searching knowledge graphs for relevant information.
package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

var _ retrieval.GraphLocalSearcher = (*LocalSearcher)(nil)
var _ retrieval.GraphGlobalSearcher = (*GlobalSearcher)(nil)

// LocalSearcher performs N-Hop traversal from specific entities to gather relational context.
// It helps retrieve relevant information by traversing the knowledge graph from given entities.
type LocalSearcher struct {
	// store is the graph store used for searching
	store abstraction.GraphStore
}

// NewLocalSearcher creates a new local searcher.
//
// Parameters:
// - store: The graph store to use for searching
//
// Returns:
// - A new LocalSearcher instance
func NewLocalSearcher(store abstraction.GraphStore) *LocalSearcher {
	return &LocalSearcher{store: store}
}

// Search performs N-Hop traversal from specific entities to gather relational context.
//
// Parameters:
// - ctx: The context for the operation
// - entities: The entities to start the search from
// - maxHops: The maximum number of hops to traverse
// - topK: The maximum number of results to return
//
// Returns:
// - A string representation of the search results
// - An error if searching fails
func (s *LocalSearcher) Search(ctx context.Context, entities []string, maxHops int, topK int) (string, error) {
	var sb strings.Builder
	sb.WriteString("Graph Relationships Found:\n")

	visitedNodes := make(map[string]bool)

	for _, entityID := range entities {
		// Traverse N-Hops from the starting entity
		nodes, err := s.store.GetNeighbors(ctx, entityID, maxHops)
		if err != nil {
			continue // Skip failing entities to keep robustness
		}

		count := 0
		for _, node := range nodes {
			if count >= topK {
				break
			}
			if !visitedNodes[node.ID] {
				visitedNodes[node.ID] = true
				sb.WriteString(fmt.Sprintf("- Node: %s (Type: %s)\n", node.ID, node.Type))
				count++
			}
		}
	}

	return sb.String(), nil
}

// GlobalSearcher performs Map-Reduce over Community Summaries for macro-level questions.
// It helps answer broad questions by synthesizing information from multiple communities
// in the knowledge graph.
type GlobalSearcher struct {
	// store is the graph store used for searching
	store abstraction.GraphStore
	// llm is the LLM client used for synthesizing results
	llm core.Client
}

// NewGlobalSearcher creates a new global searcher.
//
// Parameters:
// - store: The graph store to use for searching
// - llm: The LLM client to use for synthesizing results
//
// Returns:
// - A new GlobalSearcher instance
func NewGlobalSearcher(store abstraction.GraphStore, llm core.Client) *GlobalSearcher {
	return &GlobalSearcher{store: store, llm: llm}
}

// Search performs Map-Reduce over Community Summaries for macro-level questions.
//
// Parameters:
// - ctx: The context for the operation
// - query: The query to answer
// - communityLevel: The level of community summaries to use
//
// Returns:
// - A synthesized answer based on community summaries
// - An error if searching fails
func (s *GlobalSearcher) Search(ctx context.Context, query string, communityLevel int) (string, error) {
	summaries, err := s.store.GetCommunitySummaries(ctx, communityLevel)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve community summaries: %w", err)
	}

	if len(summaries) == 0 {
		return "No global community data available.", nil
	}

	// MAP Phase: Filter/Score each summary (simplified here by joining,
	// but a true Map-Reduce would ask LLM to score relevance first if there are too many).

	// REDUCE Phase: Ask LLM to synthesize a global answer from the summaries
	var contextBuilder strings.Builder
	for i, summary := range summaries {
		contextBuilder.WriteString(fmt.Sprintf("Community %d: %v\n", i+1, summary))
	}

	prompt := fmt.Sprintf(`You are a global data synthesizer.
Answer the user's macro-level question by synthesizing the following community summaries from our knowledge graph.
Ignore summaries that are irrelevant to the question.

[Question]
%s

[Community Summaries]
%s

[Global Answer]`, query, contextBuilder.String())

	// Use gochat's standard Chat interface
	messages := []core.Message{
		core.NewUserMessage(prompt),
	}

	response, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return "", err
	}

	return response.Content, nil
}
