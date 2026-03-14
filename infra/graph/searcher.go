package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gorag/pkg/domain/abstraction"
	"github.com/DotNetAge/gorag/pkg/usecase/retrieval"
)

var _ retrieval.GraphLocalSearcher = (*LocalSearcherImpl)(nil)
var _ retrieval.GraphGlobalSearcher = (*GlobalSearcherImpl)(nil)

// LocalSearcherImpl performs N-Hop traversal from specific entities to gather relational context.
type LocalSearcherImpl struct {
	store abstraction.GraphStore
}

func NewLocalSearcher(store abstraction.GraphStore) *LocalSearcherImpl {
	return &LocalSearcherImpl{store: store}
}

func (s *LocalSearcherImpl) Search(ctx context.Context, entities []string, maxHops int, topK int) (string, error) {
	var sb strings.Builder
	sb.WriteString("Graph Relationships Found:\n")

	visitedEdges := make(map[string]bool)

	for _, entityID := range entities {
		// Traverse N-Hops from the starting entity
		edges, err := s.store.GetNeighbors(ctx, entityID, maxHops)
		if err != nil {
			continue // Skip failing entities to keep robustness
		}

		count := 0
		for _, edge := range edges {
			if count >= topK {
				break
			}
			if !visitedEdges[edge.ID] {
				visitedEdges[edge.ID] = true
				sb.WriteString(fmt.Sprintf("- [%s] --(%s)--> [%s]\n", edge.Source, edge.Type, edge.Target))
				count++
			}
		}
	}

	return sb.String(), nil
}

// GlobalSearcherImpl performs Map-Reduce over Community Summaries for macro-level questions.
type GlobalSearcherImpl struct {
	store abstraction.GraphStore
	llm   SimpleLLMClient
}

func NewGlobalSearcher(store abstraction.GraphStore, llm SimpleLLMClient) *GlobalSearcherImpl {
	return &GlobalSearcherImpl{store: store, llm: llm}
}

func (s *GlobalSearcherImpl) Search(ctx context.Context, query string, communityLevel int) (string, error) {
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
		contextBuilder.WriteString(fmt.Sprintf("Community %d: %s\n", i+1, summary))
	}

	prompt := fmt.Sprintf(`You are a global data synthesizer.
Answer the user's macro-level question by synthesizing the following community summaries from our knowledge graph.
Ignore summaries that are irrelevant to the question.

[Question]
%s

[Community Summaries]
%s

[Global Answer]`, query, contextBuilder.String())

	return s.llm.Generate(ctx, prompt)
}
