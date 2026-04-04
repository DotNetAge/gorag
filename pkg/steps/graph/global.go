package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/DotNetAge/gochat/pkg/embedding"
	"github.com/DotNetAge/gochat/pkg/pipeline"
	"github.com/DotNetAge/gorag/pkg/core"
	"github.com/DotNetAge/gorag/pkg/logging"
)

// GlobalSearch performs community-based search for macro-level queries.
// Best for: "What are the main themes?" type questions.
// Following Microsoft GraphRAG: searches community summaries.
type GlobalSearch struct {
	graphStore core.GraphStore
	embedder   embedding.Provider
	topK       int
	logger     logging.Logger
}

type GlobalSearchOption func(*GlobalSearch)

func WithGlobalTopK(topK int) GlobalSearchOption {
	return func(s *GlobalSearch) {
		s.topK = topK
	}
}

// NewGlobalSearch creates a global search step for community-centric retrieval.
func NewGlobalSearch(graphStore core.GraphStore, embedder embedding.Provider, opts ...GlobalSearchOption) pipeline.Step[*core.RetrievalContext] {
	s := &GlobalSearch{
		graphStore: graphStore,
		embedder:   embedder,
		topK:       5,
		logger:     logging.DefaultNoopLogger(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *GlobalSearch) Name() string {
	return "GlobalSearch"
}

func (s *GlobalSearch) Execute(ctx context.Context, rctx *core.RetrievalContext) error {
	if s.graphStore == nil {
		s.logger.Warn("GraphStore is nil, skipping global search", nil)
		return nil
	}

	// Get communities from graph store
	// Communities are stored during indexing phase
	communities, err := s.getCommunities(ctx)
	if err != nil {
		s.logger.Warn("Failed to get communities", map[string]any{"error": err.Error()})
		return nil
	}

	if len(communities) == 0 {
		s.logger.Info("No communities found for global search", nil)
		return nil
	}

	s.logger.Info("Starting global search", map[string]any{
		"communities": len(communities),
		"topK":        s.topK,
	})

	// Match communities to query
	var matches []*core.CommunityMatch
	if s.embedder != nil {
		// Semantic matching using embeddings
		matches, err = s.semanticMatch(ctx, rctx.Query.Text, communities)
		if err != nil {
			s.logger.Warn("Semantic matching failed, falling back to keyword", map[string]any{"error": err.Error()})
			matches = s.keywordMatch(rctx.Query.Text, communities)
		}
	} else {
		// Keyword-based matching
		matches = s.keywordMatch(rctx.Query.Text, communities)
	}

	// Store matches in context
	rctx.CommunityMatches = matches

	// Build global context from matched communities
	rctx.GraphContext = s.buildGlobalContext(matches)

	// Collect source chunks from matched communities
	chunkSet := make(map[string]bool)
	for _, match := range matches {
		community := s.findCommunityByID(communities, match.CommunityID)
		if community != nil {
			for _, chunkID := range community.SourceChunkIDs {
				chunkSet[chunkID] = true
			}
		}
	}

	chunkIDs := make([]string, 0, len(chunkSet))
	for id := range chunkSet {
		chunkIDs = append(chunkIDs, id)
	}
	rctx.Custom["graph_chunk_ids"] = chunkIDs

	s.logger.Info("Global search completed", map[string]any{
		"matches":    len(matches),
		"chunks":     len(chunkIDs),
	})

	return nil
}

func (s *GlobalSearch) getCommunities(ctx context.Context) ([]*core.Community, error) {
	// Query communities from graph store
	// This uses the GetCommunitySummaries method from GraphStore interface
	results, err := s.graphStore.GetCommunitySummaries(ctx, 0) // Level 0 = finest granularity
	if err != nil {
		return nil, err
	}

	var communities []*core.Community
	for _, row := range results {
		community := &core.Community{
			ID:       row["id"].(string),
			Summary:  row["summary"].(string),
		}
		if nodes, ok := row["nodes"].([]string); ok {
			community.NodeIDs = nodes
		}
		communities = append(communities, community)
	}

	return communities, nil
}

func (s *GlobalSearch) semanticMatch(ctx context.Context, query string, communities []*core.Community) ([]*core.CommunityMatch, error) {
	// Embed query
	queryVecs, err := s.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	queryVec := queryVecs[0]

	var matches []*core.CommunityMatch
	for _, community := range communities {
		if community.Summary == "" {
			continue
		}

		// Embed community summary
		summaryVecs, err := s.embedder.Embed(ctx, []string{community.Summary})
		if err != nil {
			continue
		}
		summaryVec := summaryVecs[0]

		// Calculate cosine similarity
		score := cosineSimilarity(queryVec, summaryVec)
		if score > 0.5 { // Threshold
			matches = append(matches, &core.CommunityMatch{
				CommunityID: community.ID,
				Score:       score,
				Summary:     community.Summary,
				Keywords:    community.Keywords,
			})
		}
	}

	// Sort by score and limit to topK
	if len(matches) > s.topK {
		matches = matches[:s.topK]
	}

	return matches, nil
}

func (s *GlobalSearch) keywordMatch(query string, communities []*core.Community) []*core.CommunityMatch {
	queryLower := strings.ToLower(query)
	var matches []*core.CommunityMatch

	for _, community := range communities {
		if community.Summary == "" {
			continue
		}

		// Simple keyword matching
		score := float32(0)
		for _, keyword := range community.Keywords {
			if strings.Contains(queryLower, strings.ToLower(keyword)) {
				score += 0.2
			}
		}

		// Check if summary contains query words
		summaryLower := strings.ToLower(community.Summary)
		queryWords := strings.Fields(queryLower)
		for _, word := range queryWords {
			if len(word) > 3 && strings.Contains(summaryLower, word) {
				score += 0.1
			}
		}

		if score > 0.3 {
			matches = append(matches, &core.CommunityMatch{
				CommunityID: community.ID,
				Score:       score,
				Summary:     community.Summary,
				Keywords:    community.Keywords,
			})
		}
	}

	// Limit to topK
	if len(matches) > s.topK {
		matches = matches[:s.topK]
	}

	return matches
}

func (s *GlobalSearch) buildGlobalContext(matches []*core.CommunityMatch) string {
	var sb strings.Builder

	sb.WriteString("[Global Context from Community Summaries]\n\n")
	for i, match := range matches {
		sb.WriteString(fmt.Sprintf("Topic %d (Score: %.2f):\n%s\n", i+1, match.Score, match.Summary))
		if len(match.Keywords) > 0 {
			sb.WriteString(fmt.Sprintf("Keywords: %s\n", strings.Join(match.Keywords, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (s *GlobalSearch) findCommunityByID(communities []*core.Community, id string) *core.Community {
	for _, c := range communities {
		if c.ID == id {
			return c
		}
	}
	return nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (sqrt32(normA) * sqrt32(normB))
}

func sqrt32(x float32) float32 {
	// Simple Newton's method for square root
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}
