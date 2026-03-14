package retrieval

import "context"

// GraphLocalSearcher performs Entity-Centric N-Hop search in the GraphStore.
type GraphLocalSearcher interface {
	// Search returns a textual description of the N-Hop relationship sub-graph.
	Search(ctx context.Context, entities []string, maxHops int, topK int) (string, error)
}

// GraphGlobalSearcher utilizes Community Detection summaries to answer macro-level questions.
type GraphGlobalSearcher interface {
	// Search performs a Map-Reduce over community summaries at a specific hierarchical level.
	Search(ctx context.Context, query string, communityLevel int) (string, error)
}
