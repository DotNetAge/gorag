package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/vectorstore"
)

// Reranker implements result reranking using LLM
type Reranker struct {
	llm     llm.Client
	topK    int
	prompt  string
}

// NewReranker creates a new reranker
func NewReranker(llm llm.Client, topK int) *Reranker {
	if topK <= 0 {
		topK = 3
	}

	return &Reranker{
		llm:    llm,
		topK:   topK,
		prompt: defaultRerankPrompt,
	}
}

// WithPrompt sets the rerank prompt
func (r *Reranker) WithPrompt(prompt string) *Reranker {
	r.prompt = prompt
	return r
}

// Rerank reranks search results based on relevance to the query
func (r *Reranker) Rerank(ctx context.Context, query string, results []vectorstore.Result) ([]vectorstore.Result, error) {
	if len(results) <= r.topK {
		return results, nil
	}

	// Build rerank prompt
	prompt := r.buildRerankPrompt(query, results)

	// Get rerank scores from LLM
	scores, err := r.getRelevanceScores(ctx, prompt, len(results))
	if err != nil {
		return results, err
	}

	// Assign scores to results
	for i, score := range scores {
		if i < len(results) {
			results[i].Score = score
		}
	}

	// Sort by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if len(results) > r.topK {
		results = results[:r.topK]
	}

	return results, nil
}

// buildRerankPrompt builds the prompt for reranking
func (r *Reranker) buildRerankPrompt(query string, results []vectorstore.Result) string {
	prompt := r.prompt
	prompt = strings.Replace(prompt, "{query}", query, 1)

	// Add documents
	documents := ""
	for i, result := range results {
		documents += fmt.Sprintf("%d. %s\n", i+1, result.Content)
	}
	prompt = strings.Replace(prompt, "{documents}", documents, 1)

	return prompt
}

// getRelevanceScores gets relevance scores from LLM
func (r *Reranker) getRelevanceScores(ctx context.Context, prompt string, count int) ([]float32, error) {
	// Get response from LLM
	response, err := r.llm.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Parse scores from response
	scores := r.parseScores(response, count)

	return scores, nil
}

// parseScores parses relevance scores from LLM response
func (r *Reranker) parseScores(response string, expectedCount int) []float32 {
	scores := make([]float32, expectedCount)
	
	// Default scores if parsing fails
	for i := range scores {
		scores[i] = 0.5
	}
	
	// Try to extract comma-separated scores
	// Example: "0.9, 0.7, 0.3, 0.1"
	
	// Clean response
	response = strings.TrimSpace(response)
	response = strings.Trim(response, "\n")
	
	// Split by comma
	scoreStrs := strings.Split(response, ",")
	
	// Parse each score
	for i, scoreStr := range scoreStrs {
		if i >= expectedCount {
			break
		}
		
		scoreStr = strings.TrimSpace(scoreStr)
		score, err := strconv.ParseFloat(scoreStr, 32)
		if err == nil {
			// Clamp score between 0 and 1
			if score < 0 {
				score = 0
			} else if score > 1 {
				score = 1
			}
			scores[i] = float32(score)
		}
	}
	
	return scores
}

const defaultRerankPrompt = `
You are a relevance ranker. For each document below, assign a relevance score from 0 to 1 based on how well it answers the query.

Query: {query}

Documents:
{documents}

Return only a comma-separated list of scores in the same order as the documents. For example:
0.9, 0.7, 0.3, 0.1
`
