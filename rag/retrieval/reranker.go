package retrieval

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/llm"
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
func (r *Reranker) Rerank(ctx context.Context, query string, results []core.Result) ([]core.Result, error) {
	if len(results) <= r.topK {
		return results, nil
	}

	// Build rerank prompt
	prompt := r.buildRerankPrompt(query, results)

	// Get rerank scores from LLM
	scores, err := r.getRelevanceScores(ctx, prompt, len(results))
	if err != nil {
		// If reranking fails, return original results sorted by original scores
		// This provides graceful degradation
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
		if len(results) > r.topK {
			results = results[:r.topK]
		}
		// Return without error for backward compatibility
		return results, nil
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
func (r *Reranker) buildRerankPrompt(query string, results []core.Result) string {
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
	scores, err := r.parseScores(response, count)
	if err != nil {
		return nil, fmt.Errorf("failed to parse rerank scores: %w", err)
	}

	return scores, nil
}

// parseScores parses relevance scores from LLM response
func (r *Reranker) parseScores(response string, expectedCount int) ([]float32, error) {
	// Clean response
	response = strings.TrimSpace(response)
	response = strings.Trim(response, "\n")

	// Try multiple parsing strategies

	// Strategy 1: Comma-separated scores (e.g., "0.9, 0.7, 0.3, 0.1")
	if parsed, ok := r.parseCommaSeparated(response, expectedCount); ok {
		return parsed, nil
	}

	// Strategy 2: Numbered list (e.g., "1. 0.9\n2. 0.7\n3. 0.3\n4. 0.1")
	if parsed, ok := r.parseNumberedList(response, expectedCount); ok {
		return parsed, nil
	}

	// Strategy 3: Extract all numbers from response
	if parsed, ok := r.parseAllNumbers(response, expectedCount); ok {
		return parsed, nil
	}

	return nil, fmt.Errorf("failed to parse scores from LLM response: %s", response)
}

// parseCommaSeparated parses comma-separated scores
func (r *Reranker) parseCommaSeparated(response string, expectedCount int) ([]float32, bool) {
	scoreStrs := strings.Split(response, ",")
	if len(scoreStrs) < expectedCount {
		return nil, false
	}

	scores := make([]float32, expectedCount)
	successCount := 0

	for i := 0; i < expectedCount; i++ {
		scoreStr := strings.TrimSpace(scoreStrs[i])
		score, err := strconv.ParseFloat(scoreStr, 32)
		if err == nil {
			// Clamp score between 0 and 1
			if score < 0 {
				score = 0
			} else if score > 1 {
				score = 1
			}
			scores[i] = float32(score)
			successCount++
		}
	}

	// Require at least 50% successful parses
	if successCount < expectedCount/2 {
		return nil, false
	}

	return scores, true
}

// parseNumberedList parses scores from a numbered list
func (r *Reranker) parseNumberedList(response string, expectedCount int) ([]float32, bool) {
	lines := strings.Split(response, "\n")
	scores := make([]float32, expectedCount)
	successCount := 0
	scoreIndex := 0

	for _, line := range lines {
		if scoreIndex >= expectedCount {
			break
		}

		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		// Remove number prefix (e.g., "1. 0.9" -> "0.9")
		parts := strings.SplitN(line, ".", 2)
		if len(parts) != 2 {
			continue
		}

		scoreStr := strings.TrimSpace(parts[1])
		score, err := strconv.ParseFloat(scoreStr, 32)
		if err == nil {
			// Clamp score between 0 and 1
			if score < 0 {
				score = 0
			} else if score > 1 {
				score = 1
			}
			scores[scoreIndex] = float32(score)
			successCount++
		}
		scoreIndex++
	}

	// Require at least 50% successful parses
	if successCount < expectedCount/2 {
		return nil, false
	}

	return scores, true
}

// parseAllNumbers extracts all numbers from response
func (r *Reranker) parseAllNumbers(response string, expectedCount int) ([]float32, bool) {
	// Use regex to find all floating point numbers
	re := regexp.MustCompile(`\b\d+\.\d+\b`)
	matches := re.FindAllString(response, -1)

	if len(matches) < expectedCount {
		return nil, false
	}

	scores := make([]float32, expectedCount)
	successCount := 0

	for i := 0; i < expectedCount; i++ {
		score, err := strconv.ParseFloat(matches[i], 32)
		if err == nil {
			// Clamp score between 0 and 1
			if score < 0 {
				score = 0
			} else if score > 1 {
				score = 1
			}
			scores[i] = float32(score)
			successCount++
		}
	}

	// Require at least 50% successful parses
	if successCount < expectedCount/2 {
		return nil, false
	}

	return scores, true
}

const defaultRerankPrompt = `
You are a relevance ranker. For each document below, assign a relevance score from 0 to 1 based on how well it answers the query.

Query: {query}

Documents:
{documents}

Return only a comma-separated list of scores in the same order as the documents. For example:
0.9, 0.7, 0.3, 0.1
`
