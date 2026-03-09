package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/DotNetAge/gorag/core"
	"github.com/DotNetAge/gorag/embedding"
	"github.com/DotNetAge/gorag/llm"
	"github.com/DotNetAge/gorag/vectorstore"
)

// MultiHopRAG implements multi-hop retrieval for complex questions
//
// Multi-hop RAG is designed to handle complex questions that require
// information from multiple sources. It performs an initial retrieval,
// analyzes the results, and generates follow-up queries to gather
// all necessary information.
//
// Example use case: "Compare Apple and Microsoft's latest AI investments"
// The system would:
// 1. Initial retrieval about AI investments
// 2. Analyze results and identify that we need specific information about Apple
// 3. Generate a follow-up query for Apple's AI investments
// 4. Repeat for Microsoft
// 5. Combine all information to answer the original question
type MultiHopRAG struct {
	llm            llm.Client
	embedder       embedding.Provider
	vectorStore    vectorstore.Store
	maxHops        int
	analysisPrompt string
	queryPrompt    string
}

// NewMultiHopRAG creates a new multi-hop RAG instance
func NewMultiHopRAG(
	llm llm.Client,
	embedder embedding.Provider,
	vectorStore vectorstore.Store,
) *MultiHopRAG {
	return &MultiHopRAG{
		llm:            llm,
		embedder:       embedder,
		vectorStore:    vectorStore,
		maxHops:        3, // Default: maximum 3 hops
		analysisPrompt: defaultAnalysisPrompt,
		queryPrompt:    defaultFollowUpQueryPrompt,
	}
}

// WithMaxHops sets the maximum number of hops
func (m *MultiHopRAG) WithMaxHops(hops int) *MultiHopRAG {
	if hops > 0 {
		m.maxHops = hops
	}
	return m
}

// WithAnalysisPrompt sets a custom analysis prompt
func (m *MultiHopRAG) WithAnalysisPrompt(prompt string) *MultiHopRAG {
	m.analysisPrompt = prompt
	return m
}

// WithQueryPrompt sets a custom follow-up query prompt
func (m *MultiHopRAG) WithQueryPrompt(prompt string) *MultiHopRAG {
	m.queryPrompt = prompt
	return m
}

// Search performs multi-hop retrieval
func (m *MultiHopRAG) Search(ctx context.Context, query string, topK int) ([]core.Result, error) {
	return m.SearchWithHops(ctx, query, topK, m.maxHops)
}

// SearchWithHops performs multi-hop retrieval with specified max hops
func (m *MultiHopRAG) SearchWithHops(ctx context.Context, query string, topK int, maxHops int) ([]core.Result, error) {
	var allResults []core.Result
	currentQuery := query
	hopCount := 0

	for hopCount < maxHops {
		// Get embedding for current query
		embeddings, err := m.embedder.Embed(ctx, []string{currentQuery})
		if err != nil {
			// Return collected results with error for debugging
			return m.deduplicateAndSort(allResults, topK), fmt.Errorf("embedding failed at hop %d: %w", hopCount, err)
		}

		// Search vector store
		searchOpts := vectorstore.SearchOptions{
			TopK: topK * 2, // Get more results for analysis
		}
		results, err := m.vectorStore.Search(ctx, embeddings[0], searchOpts)
		if err != nil {
			return m.deduplicateAndSort(allResults, topK), fmt.Errorf("search failed at hop %d: %w", hopCount, err)
		}

		// Add results to the collection
		allResults = append(allResults, results...)

		// Analyze results to determine if we need more information
		analysis, err := m.analyzeResults(ctx, currentQuery, results)
		if err != nil {
			return m.deduplicateAndSort(allResults, topK), fmt.Errorf("analysis failed at hop %d: %w", hopCount, err)
		}

		// Check if we need more hops
		if !analysis.NeedsMoreInformation {
			break
		}

		// Generate follow-up query
		followUpQuery, err := m.generateFollowUpQuery(ctx, currentQuery, analysis.MissingInformation)
		if err != nil {
			return m.deduplicateAndSort(allResults, topK), fmt.Errorf("follow-up query generation failed at hop %d: %w", hopCount, err)
		}
		if followUpQuery == "" {
			break
		}

		// Update current query and hop count
		currentQuery = followUpQuery
		hopCount++
	}

	// Deduplicate and sort results
	return m.deduplicateAndSort(allResults, topK), nil
}

// Query performs a multi-hop RAG query
func (m *MultiHopRAG) Query(ctx context.Context, question string, maxHops int, promptTemplate string) (*Response, error) {
	// Use local variable for hops
	hops := m.maxHops
	if maxHops > 0 {
		hops = maxHops
	}

	// Perform multi-hop search with specified hops
	results, err := m.SearchWithHops(ctx, question, 5, hops)
	if err != nil {
		return nil, err
	}

	// Build context from results
	contexts := make([]string, len(results))
	for i, result := range results {
		contexts[i] = result.Content
	}

	// Build prompt
	prompt := buildMultiHopPrompt(question, contexts, promptTemplate)

	// Generate answer
	answer, err := m.llm.Complete(ctx, prompt)
	if err != nil {
		return nil, err
	}

	return &Response{
		Answer:  answer,
		Sources: results,
	}, nil
}

// buildMultiHopPrompt builds the prompt for multi-hop RAG
func buildMultiHopPrompt(question string, contexts []string, template string) string {
	if template != "" {
		// Build context string
		var contextStr strings.Builder
		for i, ctx := range contexts {
			contextStr.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
		}

		// Replace placeholders
		result := strings.ReplaceAll(template, "{question}", question)
		result = strings.ReplaceAll(result, "{context}", contextStr.String())
		return result
	}

	var buf strings.Builder
	buf.WriteString("基于以下上下文信息回答问题。如果上下文中没有相关信息，请说明无法回答。\n\n")
	buf.WriteString("上下文：\n")

	for i, ctx := range contexts {
		buf.WriteString(fmt.Sprintf("%d. %s\n", i+1, ctx))
	}

	buf.WriteString("\n问题：\n")
	buf.WriteString(question)
	buf.WriteString("\n\n答案：\n")

	return buf.String()
}

// AnalysisResult represents the result of analyzing search results
type AnalysisResult struct {
	NeedsMoreInformation bool
	MissingInformation   string
	Confidence           float32
}

// analyzeResults analyzes search results to determine if more information is needed
func (m *MultiHopRAG) analyzeResults(ctx context.Context, query string, results []core.Result) (*AnalysisResult, error) {
	// Build analysis prompt
	prompt := m.analysisPrompt
	prompt = strings.Replace(prompt, "{query}", query, 1)

	// Add results
	documents := ""
	for i, result := range results {
		documents += fmt.Sprintf("%d. %s\n", i+1, result.Content)
	}
	prompt = strings.Replace(prompt, "{documents}", documents, 1)

	// Get analysis from LLM
	response, err := m.llm.Complete(ctx, prompt)
	if err != nil {
		return &AnalysisResult{
			NeedsMoreInformation: false,
			Confidence:           0.0,
		}, nil
	}

	// Parse analysis result
	return m.parseAnalysis(response), nil
}

// generateFollowUpQuery generates a follow-up query based on missing information
func (m *MultiHopRAG) generateFollowUpQuery(ctx context.Context, originalQuery, missingInfo string) (string, error) {
	// Build query prompt
	prompt := m.queryPrompt
	prompt = strings.Replace(prompt, "{original_query}", originalQuery, 1)
	prompt = strings.Replace(prompt, "{missing_information}", missingInfo, 1)

	// Get follow-up query from LLM
	response, err := m.llm.Complete(ctx, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// deduplicateAndSort deduplicates and sorts results by score
func (m *MultiHopRAG) deduplicateAndSort(results []core.Result, topK int) []core.Result {
	// Deduplicate results
	seen := make(map[string]bool)
	var uniqueResults []core.Result

	for _, result := range results {
		if !seen[result.ID] {
			seen[result.ID] = true
			uniqueResults = append(uniqueResults, result)
		}
	}

	// Sort by score
	sort.Slice(uniqueResults, func(i, j int) bool {
		return uniqueResults[i].Score > uniqueResults[j].Score
	})

	// Return top K results
	if len(uniqueResults) > topK {
		uniqueResults = uniqueResults[:topK]
	}

	return uniqueResults
}

// parseAnalysis parses the analysis response from LLM
func (m *MultiHopRAG) parseAnalysis(response string) *AnalysisResult {
	response = strings.ToLower(response)

	needsMoreInfo := strings.Contains(response, "yes") || strings.Contains(response, "needs more")
	var missingInfo string

	// Extract missing information
	if idx := strings.Index(response, "missing:"); idx != -1 {
		missingInfo = strings.TrimSpace(response[idx+len("missing:"):])
	} else if idx := strings.Index(response, "need:"); idx != -1 {
		missingInfo = strings.TrimSpace(response[idx+len("need:"):])
	}

	return &AnalysisResult{
		NeedsMoreInformation: needsMoreInfo,
		MissingInformation:   missingInfo,
		Confidence:           0.8, // Default confidence
	}
}

// Default prompts
const (
	defaultAnalysisPrompt = `You are an AI assistant analyzing search results for a complex question.

Original Question: {query}

Search Results:
{documents}

Please analyze these results and determine if they contain enough information to answer the original question completely. Consider if there are any gaps or missing information that would be needed for a comprehensive answer.

Respond with:
1. YES or NO indicating whether more information is needed
2. If YES, describe what specific information is missing

Example response:
YES
Missing information about Apple's 2024 AI investment figures`

	defaultFollowUpQueryPrompt = `You are an AI assistant generating a follow-up query for multi-hop retrieval.

Original Question: {original_query}

Missing Information: {missing_information}

Generate a specific, focused follow-up query that will help retrieve the missing information. The query should be designed to get exactly the information needed to fill the gap.

Example follow-up query:
What was Apple's AI investment budget for 2024?`
)
