package rag

import (
	"context"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/vectorstore"
)

// ContextCompressor compresses and optimizes context for LLM input
type ContextCompressor struct {
	llm                llm.Client
	similarityThreshold float32
	maxContextSize     int
	summaryPrompt      string
}

// NewContextCompressor creates a new context compressor
func NewContextCompressor(llm llm.Client) *ContextCompressor {
	return &ContextCompressor{
		llm:                llm,
		similarityThreshold: 0.8,
		maxContextSize:     4000,
		summaryPrompt:      defaultSummaryPrompt,
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

// Compress compresses and optimizes context
func (c *ContextCompressor) Compress(ctx context.Context, query string, results []vectorstore.Result) ([]vectorstore.Result, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Step 1: Filter by relevance score
	filtered := c.filterByRelevance(results)

	// Step 2: Remove redundancy
	deDuplicated := c.removeRedundancy(filtered)

	// Step 3: Reorder by relevance
	reordered := c.reorderByRelevance(deDuplicated)

	// Step 4: Truncate to fit context window
	truncated := c.truncateToContextSize(reordered, query)

	return truncated, nil
}

// filterByRelevance filters results by relevance score
func (c *ContextCompressor) filterByRelevance(results []vectorstore.Result) []vectorstore.Result {
	var filtered []vectorstore.Result
	for _, result := range results {
		// Filter out results with very low relevance
		if result.Score > 0.3 {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

// removeRedundancy removes redundant content
func (c *ContextCompressor) removeRedundancy(results []vectorstore.Result) []vectorstore.Result {
	if len(results) <= 1 {
		return results
	}

	var unique []vectorstore.Result
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
func (c *ContextCompressor) reorderByRelevance(results []vectorstore.Result) []vectorstore.Result {
	sorted := make([]vectorstore.Result, len(results))
	copy(sorted, results)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	return sorted
}

// truncateToContextSize truncates results to fit the context window
func (c *ContextCompressor) truncateToContextSize(results []vectorstore.Result, query string) []vectorstore.Result {
	var selected []vectorstore.Result
	totalSize := len(query)

	for _, result := range results {
		contentSize := len(result.Content)
		if totalSize+contentSize+100 <= c.maxContextSize {
			selected = append(selected, result)
			totalSize += contentSize
		} else {
			// Try to summarize the remaining content
			summarized := c.summarizeContent(result.Content, query)
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
func (c *ContextCompressor) summarizeContent(content, query string) string {
	prompt := strings.Replace(c.summaryPrompt, "{content}", content, 1)
	prompt = strings.Replace(prompt, "{query}", query, 1)

	// Get summary from LLM
	summary, err := c.llm.Complete(context.Background(), prompt)
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
