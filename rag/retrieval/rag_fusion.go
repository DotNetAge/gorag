package retrieval

import (
	"context"
	"github.com/DotNetAge/gorag/utils/llmutil"
	"fmt"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedding"
	gochatcore "github.com/DotNetAge/gochat/pkg/core"
	"github.com/DotNetAge/gorag/vectorstore"
)

// RAGFusion implements the RAG-Fusion retrieval strategy
// https://arxiv.org/abs/2309.00812
type RAGFusion struct {
	llm          gochatcore.Client
	embedder     embedding.Provider
	vectorStore  vectorstore.Store
	keywordStore KeywordStore
	numQueries   int
	fusionAlpha  float32
	queryPrompt  string
}

// NewRAGFusion creates a new RAG-Fusion instance
func NewRAGFusion(
	llm gochatcore.Client,
	embedder embedding.Provider,
	vectorStore vectorstore.Store,
	keywordStore KeywordStore,
) *RAGFusion {
	return &RAGFusion{
		llm:          llm,
		embedder:     embedder,
		vectorStore:  vectorStore,
		keywordStore: keywordStore,
		numQueries:   4,   // Default: generate 4 query variations
		fusionAlpha:  0.1, // Default: 0.1 for reciprocal rank fusion
		queryPrompt:  defaultQueryPrompt,
	}
}

// WithNumQueries sets the number of query variations to generate
func (rf *RAGFusion) WithNumQueries(num int) *RAGFusion {
	if num > 0 {
		rf.numQueries = num
	}
	return rf
}

// WithFusionAlpha sets the alpha parameter for reciprocal rank fusion
func (rf *RAGFusion) WithFusionAlpha(alpha float32) *RAGFusion {
	if alpha > 0 {
		rf.fusionAlpha = alpha
	}
	return rf
}

// WithQueryPrompt sets a custom query generation prompt
func (rf *RAGFusion) WithQueryPrompt(prompt string) *RAGFusion {
	rf.queryPrompt = prompt
	return rf
}

// Search performs RAG-Fusion search
func (rf *RAGFusion) Search(ctx context.Context, originalQuery string, topK int) ([]core.Result, error) {
	// Generate query variations
	queries, err := rf.generateQueryVariations(ctx, originalQuery)
	if err != nil {
		// Fallback to original query if query generation fails
		queries = []string{originalQuery}
	}

	// Add original query to the list
	queries = append(queries, originalQuery)

	// Deduplicate queries
	queries = rf.deduplicateQueries(queries)

	// Perform search for each query
	allResults := make(map[string]core.Result)
	ranks := make(map[string][]int)
	var searchErrors []error

	for _, query := range queries {
		// Get embedding for query
		embeddings, err := rf.embedder.Embed(ctx, []string{query})
		if err != nil {
			searchErrors = append(searchErrors, fmt.Errorf("embedding failed for query %q: %w", query, err))
			continue
		}

		// Search vector store
		searchOpts := vectorstore.SearchOptions{
			TopK: topK * 2, // Get more results for fusion
		}
		results, err := rf.vectorStore.Search(ctx, embeddings[0], searchOpts)
		if err != nil {
			searchErrors = append(searchErrors, fmt.Errorf("search failed for query %q: %w", query, err))
			continue
		}

		// Also perform keyword search if available
		if rf.keywordStore != nil {
			keywordResults, err := rf.keywordStore.Search(ctx, query, topK*2)
			if err == nil {
				results = append(results, keywordResults...)
			}
		}

		// Store results and their ranks
		for rank, result := range results {
			if _, exists := allResults[result.ID]; !exists {
				allResults[result.ID] = result
				ranks[result.ID] = []int{}
			}
			ranks[result.ID] = append(ranks[result.ID], rank+1) // Rank starts at 1
		}
	}

	// If all queries failed, return error
	if len(allResults) == 0 && len(searchErrors) > 0 {
		return nil, fmt.Errorf("all RAG fusion queries failed: %v", searchErrors)
	}

	// Fuse results using reciprocal rank fusion
	fusedResults := rf.fuseResults(allResults, ranks)

	// Return top K results
	if len(fusedResults) > topK {
		fusedResults = fusedResults[:topK]
	}

	return fusedResults, nil
}

// generateQueryVariations generates multiple query variations
func (rf *RAGFusion) generateQueryVariations(ctx context.Context, originalQuery string) ([]string, error) {
	prompt := strings.Replace(rf.queryPrompt, "{query}", originalQuery, 1)
	prompt = strings.Replace(prompt, "{num_queries}", fmt.Sprintf("%d", rf.numQueries), 1)

	// Get query variations from LLM
	response, err := llmutil.Complete(ctx, rf.llm, prompt)
	if err != nil {
		return nil, err
	}

	// Parse query variations
	queries := rf.parseQueryVariations(response)

	return queries, nil
}

// parseQueryVariations parses query variations from LLM response
func (rf *RAGFusion) parseQueryVariations(response string) []string {
	var queries []string
	lines := strings.Split(response, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove numbering if present (e.g., "1. " or "- ")
		line = strings.TrimPrefix(line, "1. ")
		line = strings.TrimPrefix(line, "2. ")
		line = strings.TrimPrefix(line, "3. ")
		line = strings.TrimPrefix(line, "4. ")
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "*")

		if line != "" {
			queries = append(queries, line)
		}
	}

	return queries
}

// deduplicateQueries removes duplicate queries
func (rf *RAGFusion) deduplicateQueries(queries []string) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, query := range queries {
		query = strings.TrimSpace(query)
		if !seen[query] && query != "" {
			seen[query] = true
			unique = append(unique, query)
		}
	}

	return unique
}

// fuseResults fuses results using reciprocal rank fusion
func (rf *RAGFusion) fuseResults(results map[string]core.Result, ranks map[string][]int) []core.Result {
	type scoredResult struct {
		core.Result
		score float32
	}

	var scoredResults []scoredResult

	for id, result := range results {
		// Calculate reciprocal rank score
		score := float32(0)
		for _, rank := range ranks[id] {
			score += 1.0 / (float32(rank) + rf.fusionAlpha)
		}

		scoredResults = append(scoredResults, scoredResult{
			Result: result,
			score:  score,
		})
	}

	// Sort by score
	sort.Slice(scoredResults, func(i, j int) bool {
		return scoredResults[i].score > scoredResults[j].score
	})

	// Convert back to core.Result
	fused := make([]core.Result, len(scoredResults))
	for i, sr := range scoredResults {
		fused[i] = sr.Result
		fused[i].Score = sr.score
	}

	return fused
}

const defaultQueryPrompt = `
You are given a user query. Please generate {num_queries} different variations of this query that capture different aspects or perspectives of the original question.

Original query: {query}

Query variations:
1. 
2. 
3. 
4. 
`
