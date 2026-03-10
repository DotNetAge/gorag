package rag

import (
	"context"
	"github.com/DotNetAge/gorag/utils/llmutil"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/core"
	gochatcore "github.com/DotNetAge/gochat/pkg/core"
)

// ContextCompressor compresses and optimizes context for LLM input
type ContextCompressor struct {
	llm                gochatcore.Client
	similarityThreshold float32
	maxContextSize     int
	summaryPrompt      string
	minRelevanceScore  float32
}

// NewContextCompressor creates a new context compressor
func NewContextCompressor(llm gochatcore.Client) *ContextCompressor {
	return &ContextCompressor{
		llm:                llm,
		similarityThreshold: 0.8,
		maxContextSize:     4000,
		summaryPrompt:      defaultSummaryPrompt,
		minRelevanceScore:  0.3,
	}
}

// WithSimilarityThreshold sets the similarity threshold for redundancy removal
func (c *ContextCompressor) WithSimilarityThreshold(threshold float32) *ContextCompressor {
	c.similarityThreshold = threshold
	return c
}

// WithMaxContextSize sets the maximum context size
func (c *ContextCompressor) WithMaxContextSize(size int) *ContextCompressor {
	c.maxContextSize = size
	return c
}

// WithSummaryPrompt sets a custom summary prompt
func (c *ContextCompressor) WithSummaryPrompt(prompt string) *ContextCompressor {
	c.summaryPrompt = prompt
	return c
}

// WithMinRelevanceScore sets the minimum relevance score for filtering
func (c *ContextCompressor) WithMinRelevanceScore(score float32) *ContextCompressor {
	c.minRelevanceScore = score
	return c
}

// Compress compresses and optimizes context
func (c *ContextCompressor) Compress(ctx context.Context, query string, results []core.Result) ([]core.Result, error) {
	if c == nil || len(results) == 0 {
		return results, nil
	}

	// Step 1: Filter by relevance score
	filtered := c.filterByRelevance(results)

	// Step 2: Remove redundancy
	deDuplicated := c.removeRedundancy(filtered)

	// Step 3: Reorder by relevance
	reordered := c.reorderByRelevance(deDuplicated)

	// Step 4: Truncate to fit context window
	truncated := c.truncateToContextSize(ctx, reordered, query)

	return truncated, nil
}

// filterByRelevance filters results by relevance score
func (c *ContextCompressor) filterByRelevance(results []core.Result) []core.Result {
	var filtered []core.Result
	for _, result := range results {
		if result.Score > c.minRelevanceScore {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// removeRedundancy removes redundant content
func (c *ContextCompressor) removeRedundancy(results []core.Result) []core.Result {
	if len(results) <= 1 {
		return results
	}

	var unique []core.Result
	seen := make(map[string]bool)

	for _, result := range results {
		// Check for exact duplicates
		content := strings.TrimSpace(result.Content)
		if seen[content] {
			continue
		}

		// Check for near duplicates
		isDuplicate := false
		for _, existing := range unique {
			similarity := c.calculateSimilarity(content, strings.TrimSpace(existing.Content))
			if similarity > c.similarityThreshold {
				isDuplicate = true
				break
			}
		}

		if !isDuplicate {
			seen[content] = true
			unique = append(unique, result)
		}
	}

	return unique
}

// reorderByRelevance reorders results by relevance score
func (c *ContextCompressor) reorderByRelevance(results []core.Result) []core.Result {
	sorted := make([]core.Result, len(results))
	copy(sorted, results)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	return sorted
}

// truncateToContextSize truncates results to fit the context window
func (c *ContextCompressor) truncateToContextSize(ctx context.Context, results []core.Result, query string) []core.Result {
	var selected []core.Result
	totalSize := len(query)

	for _, result := range results {
		contentSize := len(result.Content)
		if totalSize+contentSize+100 <= c.maxContextSize {
			selected = append(selected, result)
			totalSize += contentSize
		} else {
			// Try to summarize the remaining content
			summarized := c.summarizeContent(ctx, result.Content, query)
			if totalSize+len(summarized)+100 <= c.maxContextSize {
				// Create a new result with the summary
				summarizedResult := result
				summarizedResult.Content = summarized
				selected = append(selected, summarizedResult)
				totalSize += len(summarized)
			}
			break
		}
	}

	return selected
}

// summarizeContent summarizes content to fit in context
func (c *ContextCompressor) summarizeContent(ctx context.Context, content, query string) string {
	prompt := strings.Replace(c.summaryPrompt, "{content}", content, 1)
	prompt = strings.Replace(prompt, "{query}", query, 1)

	// Get summary from LLM using the caller's context
	summary, err := llmutil.Complete(ctx, c.llm, prompt)
	if err != nil {
		// Fallback to truncation if summarization fails
		maxLength := 300
		if len(content) <= maxLength {
			return content
		}
		return content[:maxLength] + "..."
	}

	return strings.TrimSpace(summary)
}

// calculateSimilarity calculates similarity between two texts
func (c *ContextCompressor) calculateSimilarity(text1, text2 string) float32 {
	// Simple similarity calculation based on common words
	words1 := c.extractWords(text1)
	words2 := c.extractWords(text2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0
	}

	common := 0
	for word := range words1 {
		if words2[word] {
			common++
		}
	}

	return float32(common) / float32(len(words1)+len(words2)-common)
}

// extractWords extracts unique words from text
func (c *ContextCompressor) extractWords(text string) map[string]bool {
	words := make(map[string]bool)
	text = strings.ToLower(text)
	
	// Split by whitespace and punctuation
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})

	for _, token := range tokens {
		if len(token) > 2 { // Ignore short words
			words[token] = true
		}
	}

	return words
}

const defaultSummaryPrompt = `
Please summarize the following content to make it concise while preserving the most important information relevant to the query. The summary should be no longer than 3-4 sentences.

Query: {query}

Content:
{content}

Summary:
`
