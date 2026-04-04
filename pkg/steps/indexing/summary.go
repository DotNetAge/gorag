package stepinx

import (
	"context"
	"fmt"
	"strings"

	chat "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

const communitySummaryPrompt = `You are an expert in knowledge graph analysis and summarization.
Your task is to generate a comprehensive summary for a community of related entities.

[Community Information]
Level: %d (0 = finest granularity)
Node Count: %d
Nodes: %s

[Task]
Generate a concise summary (2-4 sentences) that captures:
1. The main topic or theme of this community
2. Key entities and their relationships
3. Important concepts or events

Summary:`

type generateSummaries struct {
	llm        chat.Client
	graphStore core.GraphStore
	logger     logging.Logger
}

// GenerateSummaries creates a step that generates summaries for detected communities.
// Following Microsoft GraphRAG design, this enables global search by providing
// high-level descriptions of community content.
func GenerateSummaries(llm chat.Client, graphStore core.GraphStore, logger logging.Logger) pipeline.Step[*core.IndexingContext] {
	if logger == nil {
		logger = logging.DefaultNoopLogger()
	}
	return &generateSummaries{
		llm:        llm,
		graphStore: graphStore,
		logger:     logger,
	}
}

func (s *generateSummaries) Name() string {
	return "GenerateSummaries"
}

func (s *generateSummaries) Execute(ctx context.Context, state *core.IndexingContext) error {
	if s.llm == nil {
		s.logger.Warn("LLM is nil, skipping summary generation", nil)
		return nil
	}

	if len(state.Communities) == 0 {
		s.logger.Info("No communities to summarize", nil)
		return nil
	}

	s.logger.Info("Starting community summary generation", map[string]any{
		"communities": len(state.Communities),
	})

	for i, community := range state.Communities {
		// Skip if already has summary
		if community.Summary != "" {
			continue
		}

		// Get node details for context
		nodeDescriptions := s.getNodeDescriptions(ctx, community.NodeIDs)

		// Generate summary via LLM
		summary, err := s.generateSummary(ctx, community, nodeDescriptions)
		if err != nil {
			s.logger.Warn("Failed to generate summary for community", map[string]any{
				"community_id": community.ID,
				"error":        err.Error(),
			})
			continue
		}

		state.Communities[i].Summary = summary

		// Extract keywords from summary
		keywords := s.extractKeywords(summary)
		state.Communities[i].Keywords = keywords

		// Collect source chunk IDs from community nodes
		state.Communities[i].SourceChunkIDs = s.collectSourceChunks(ctx, community.NodeIDs)

		s.logger.Debug("Generated community summary", map[string]any{
			"community_id": community.ID,
			"level":        community.Level,
			"node_count":   len(community.NodeIDs),
		})
	}

	s.logger.Info("Community summary generation completed", map[string]any{
		"total": len(state.Communities),
	})

	return nil
}

func (s *generateSummaries) getNodeDescriptions(ctx context.Context, nodeIDs []string) string {
	if s.graphStore == nil {
		return strings.Join(nodeIDs, ", ")
	}

	var descriptions []string
	for _, id := range nodeIDs {
		node, err := s.graphStore.GetNode(ctx, id)
		if err != nil || node == nil {
			descriptions = append(descriptions, id)
			continue
		}

		desc := fmt.Sprintf("%s (%s)", id, node.Type)
		descriptions = append(descriptions, desc)
	}

	return strings.Join(descriptions, ", ")
}

func (s *generateSummaries) generateSummary(ctx context.Context, community *core.Community, nodeDescriptions string) (string, error) {
	prompt := fmt.Sprintf(communitySummaryPrompt,
		community.Level,
		len(community.NodeIDs),
		nodeDescriptions,
	)

	messages := []chat.Message{
		chat.NewUserMessage(prompt),
	}

	resp, err := s.llm.Chat(ctx, messages)
	if err != nil {
		return "", fmt.Errorf("LLM chat failed: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

func (s *generateSummaries) extractKeywords(summary string) []string {
	// Simple keyword extraction based on common patterns
	// In production, could use NLP libraries or LLM
	words := strings.Fields(strings.ToLower(summary))
	var keywords []string

	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "being": true,
		"have": true, "has": true, "had": true, "do": true, "does": true,
		"did": true, "will": true, "would": true, "could": true, "should": true,
		"may": true, "might": true, "must": true, "shall": true, "can": true,
		"this": true, "that": true, "these": true, "those": true,
		"and": true, "or": true, "but": true, "in": true, "on": true,
		"at": true, "to": true, "for": true, "of": true, "with": true,
	}

	seen := make(map[string]bool)
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) > 3 && !stopWords[word] && !seen[word] {
			keywords = append(keywords, word)
			seen[word] = true
		}
	}

	// Limit to top 10 keywords
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}

	return keywords
}

func (s *generateSummaries) collectSourceChunks(ctx context.Context, nodeIDs []string) []string {
	if s.graphStore == nil {
		return nil
	}

	chunkSet := make(map[string]bool)
	for _, nodeID := range nodeIDs {
		node, err := s.graphStore.GetNode(ctx, nodeID)
		if err != nil || node == nil {
			continue
		}
		for _, chunkID := range node.SourceChunkIDs {
			chunkSet[chunkID] = true
		}
	}

	var chunks []string
	for chunkID := range chunkSet {
		chunks = append(chunks, chunkID)
	}

	return chunks
}
